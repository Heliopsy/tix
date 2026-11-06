// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/quickadd"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/taskio"
)

// auditTaskImport names the audit action one task import records for the run.
const auditTaskImport = "task.import"

// defaultImportPreview is how many rows a dry run shows when none is asked.
const defaultImportPreview = 20

// maxImportSample bounds the sample values one column reports.
const maxImportSample = 3

// errImportRolledBack ends the transaction of a dry run or of an import with a
// refused row, so the rows it wrote are rolled back while its report stands.
var errImportRolledBack = errors.New("task import rolled back")

// ImportTasks reads a file of tasks into one project under a column and value
// mapping, all or nothing.
func (l *Local) ImportTasks(ctx context.Context, r io.Reader, in core.TaskImportInput) (*core.TaskImportResult, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskImport, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	mapping, err := importMapping(in)
	if err != nil {
		return nil, err
	}
	if in.Format == core.TaskFormatLines {
		mapping.Unknown = map[taskio.Target]taskio.UnknownPolicy{taskio.TargetAssignee: taskio.PolicyError}
	}
	if r == nil {
		r = strings.NewReader(in.Content)
	}
	buffered, err := bufferUpload(r, taskio.DefaultLimits.MaxBytes, "import file")
	if err != nil {
		return nil, err
	}
	// The buffer is read-only and its Close only removes a spill file.
	defer func() { _ = buffered.Close() }()

	file, err := readImportFile(buffered, in, mapping, l.clock.Now())
	if err != nil {
		return nil, err
	}
	imp := &taskImport{l: l, actor: actor, in: in, mapping: mapping, file: file}
	err = l.write(ctx, actor, func(m *mutation) error {
		if err := imp.run(ctx, m); err != nil {
			return err
		}
		if in.DryRun || imp.result.Failed > 0 {
			return errImportRolledBack
		}
		return nil
	})
	if err != nil && !errors.Is(err, errImportRolledBack) {
		return nil, err
	}
	imp.result.Committed = err == nil
	return imp.result, nil
}

// importMapping is the mapping the input carries, or the empty one that
// leaves every column to the suggestion.
func importMapping(in core.TaskImportInput) (*taskio.Mapping, error) {
	m := &taskio.Mapping{Version: taskio.MappingVersion}
	if strings.TrimSpace(in.Mapping) != "" {
		parsed, err := taskio.ParseMapping([]byte(in.Mapping))
		if err != nil {
			return nil, core.OnField(err, "mapping")
		}
		m = parsed
	}
	if in.Mode != "" {
		m.Mode = taskio.Mode(in.Mode)
	}
	if in.Format != "" && in.Format != core.TaskFormatLines {
		m.Format = string(in.Format)
	}
	return m, nil
}

// importFile is a whole file read into rows, before any database work.
type importFile struct {
	format  core.TaskFormat
	source  string
	headers []string
	rows    []taskio.Row
	drafts  []taskio.Draft
	lines   bool
}

// readImportFile reads every row. It runs before the transaction opens, so a
// slow upload or a malformed file never holds the write lock.
func readImportFile(r io.Reader, in core.TaskImportInput, m *taskio.Mapping, now time.Time) (*importFile, error) {
	if in.Format == core.TaskFormatLines {
		return readImportLines(r, now)
	}
	opts, err := m.ReadOptions(taskio.DefaultLimits)
	if err != nil {
		return nil, core.OnField(err, "mapping")
	}
	if opts.Format == "" && in.Filename != "" {
		if f := taskio.DetectFormat(in.Filename, nil); f != taskio.FormatCSV || strings.HasSuffix(strings.ToLower(in.Filename), ".csv") {
			opts.Format = f
		}
	}
	reader, err := taskio.Open(r, opts)
	if err != nil {
		return nil, err
	}
	out := &importFile{source: reader.Source(), headers: reader.Headers()}
	for {
		row, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		out.rows = append(out.rows, row)
	}
	out.format = core.TaskFormat(opts.Format)
	if out.format == "" {
		out.format = detectedFormat(out)
	}
	return out, nil
}

// detectedFormat names the format a reader settled on when none was asked:
// rows of a JSON file carry dotted paths or came from an array, which the
// reader does not report, so the headers are the evidence left.
func detectedFormat(f *importFile) core.TaskFormat {
	if f.source == taskio.SourceTrello {
		return core.TaskFormatJSON
	}
	return core.TaskFormatCSV
}

// readImportLines reads one task per line in quick-add syntax. Blank lines and
// lines starting with # followed by a space are skipped, and a leading list
// marker such as "- " or "* " or "- [ ] " is dropped, so a pasted Markdown
// list reads as it looks.
func readImportLines(r io.Reader, now time.Time) (*importFile, error) {
	out := &importFile{format: core.TaskFormatLines, lines: true}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64<<10), taskio.DefaultLimits.MaxCell)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "# ") {
			continue
		}
		for _, marker := range []string{"- [ ] ", "- [x] ", "* [ ] ", "- ", "* ", "+ "} {
			text = strings.TrimSpace(strings.TrimPrefix(text, marker))
		}
		if len(out.drafts) >= taskio.DefaultLimits.MaxRows {
			return nil, core.Invalid("the list has more than %d lines; split it and import the parts",
				taskio.DefaultLimits.MaxRows)
		}
		out.drafts = append(out.drafts, lineDraft(line, text, now))
	}
	if err := scanner.Err(); err != nil {
		return nil, core.Invalid("reading the list: %v", err)
	}
	if len(out.drafts) == 0 {
		return nil, core.Invalid("the list holds no tasks; write one task per line")
	}
	return out, nil
}

// lineDraft reads one quick-add line as a draft.
func lineDraft(line int, text string, now time.Time) taskio.Draft {
	d := taskio.Draft{Line: line}
	parsed, err := quickadd.Parse(text, now)
	if err != nil {
		d.Title = text
		d.Errors = append(d.Errors, err.Error())
		return d
	}
	d.Title, d.Priority, d.Tags, d.Due = parsed.Title, parsed.Priority, parsed.Tags, parsed.Due
	d.Assignee, d.AssigneeRaw = parsed.Assignee, parsed.Assignee
	d.Status, d.StatusRaw = parsed.Status, parsed.Status
	d.Parent = parsed.Parent
	if parsed.Project != "" {
		d.Errors = append(d.Errors, "a line cannot name a project; it is imported into the one chosen")
	}
	if d.Title == "" {
		d.Errors = append(d.Errors, "title is empty")
	}
	return d
}

// taskImport is one import run.
type taskImport struct {
	l       *Local
	actor   *core.Actor
	in      core.TaskImportInput
	mapping *taskio.Mapping
	file    *importFile
	result  *core.TaskImportResult

	project  *core.Project
	workflow core.WorkflowDefinition
	defs     map[string]core.FieldDef
	columns  map[string]taskio.Target
	system   string
	values   valueBook

	actors map[string]string
	rows   []importRow
	keys   map[string]string
}

// importRow is a draft and what became of it.
type importRow struct {
	draft  taskio.Draft
	out    core.TaskImportRow
	taskID string
	task   *core.Task
}

// run plans and applies the whole file inside m's transaction.
func (i *taskImport) run(ctx context.Context, m *mutation) error {
	if err := i.prepare(ctx, m.tx); err != nil {
		return err
	}
	for k := range i.rows {
		if err := i.applyRow(ctx, m, &i.rows[k]); err != nil {
			return err
		}
	}
	for k := range i.rows {
		if err := i.linkRow(ctx, m, &i.rows[k]); err != nil {
			return err
		}
	}
	i.report()
	if i.in.DryRun || i.result.Failed > 0 {
		return nil
	}
	summary := map[string]any{
		"created": i.result.Created, "updated": i.result.Updated, "skipped": i.result.Skipped,
		"rows": i.result.Rows, "format": string(i.result.Format), "source": i.result.Source,
		"mode": string(i.result.Mode), "mapping": i.result.Mapping,
	}
	return m.Record(auditTaskImport, core.EventImportCompleted, "project", i.project.ID, i.project.ID,
		nil, summary, map[string]any{"project": i.project.Key, "created": i.result.Created,
			"updated": i.result.Updated, "skipped": i.result.Skipped})
}

// prepare resolves the project, its workflow and fields, the column mapping
// and every row's draft.
func (i *taskImport) prepare(ctx context.Context, tx store.Tx) error {
	project, err := lookupProject(ctx, tx, i.in.ProjectRef)
	if err != nil {
		return core.OnField(err, "project_ref")
	}
	if _, err := i.l.authorize(ctx, authz.ActionTaskImport, authz.Resource{ProjectID: project.ID}); err != nil {
		return err
	}
	i.project = project
	wf, err := tx.GetWorkflowByID(ctx, project.WorkflowID)
	if err != nil {
		return err
	}
	i.workflow = wf.Definition
	defs, err := tx.ListFieldDefs(ctx, project.ID)
	if err != nil {
		return err
	}
	i.defs = map[string]core.FieldDef{}
	infos := make([]taskio.FieldInfo, 0, len(defs))
	for _, d := range defs {
		i.defs[d.Key] = d
		infos = append(infos, taskio.FieldInfo{Key: d.Key, Label: d.Label, Type: d.Type, Options: d.EnumOptions})
	}
	i.system = i.mapping.SourceName(i.file.source)
	i.actors = map[string]string{}
	i.keys = map[string]string{}
	i.result = &core.TaskImportResult{
		DryRun: i.in.DryRun, Format: i.file.format, Source: i.file.source,
		Mode: core.TaskImportMode(i.mapping.EffectiveMode()), Values: map[string][]core.TaskImportValue{},
	}
	drafts := i.file.drafts
	if !i.file.lines {
		i.columns = i.mapping.Resolve(i.file.headers, i.file.source, infos)
		i.describeColumns(infos)
		for _, row := range i.file.rows {
			drafts = append(drafts, i.mapping.Draft(row, i.columns))
		}
	} else {
		i.result.Confident = true
	}
	i.values = newValueBook(i.mapping, i.workflow)
	i.values.actors = func(name string) bool { return i.actors[strings.ToLower(name)] != "" }
	i.rows = make([]importRow, len(drafts))
	for k, d := range drafts {
		i.rows[k] = importRow{draft: d, out: core.TaskImportRow{Line: d.Line, ExternalID: d.ExternalID, Title: d.Title}}
		i.values.observe(d)
	}
	i.result.Rows = len(drafts)
	return nil
}

// describeColumns reports every column, whether the mapping named it, and a
// few of its values; the suggestion is confident when every column it filled
// was recognised by name rather than guessed into the description.
func (i *taskImport) describeColumns(infos []taskio.FieldInfo) {
	suggested := taskio.Suggest(i.file.headers, i.file.source, infos)
	confident := true
	for _, h := range i.file.headers {
		_, named := i.mapping.Columns[h]
		if !named {
			_, named = i.mapping.Columns[taskio.BaseHeader(h)]
		}
		col := core.TaskImportColumn{Header: h, Target: string(i.columns[h]), Suggested: !named}
		for _, row := range i.file.rows {
			if v := strings.TrimSpace(row.Values[h]); v != "" {
				col.Sample = append(col.Sample, truncate(v, 80))
				if len(col.Sample) == maxImportSample {
					break
				}
			}
		}
		if !named && suggested[h] == taskio.TargetAppendBody && len(col.Sample) > 0 {
			confident = false
		}
		i.result.Columns = append(i.result.Columns, col)
	}
	if !slices.Contains(slices.Collect(mapValues(i.columns)), taskio.TargetTitle) {
		confident = false
	}
	i.result.Confident = confident
}

func mapValues(m map[string]taskio.Target) func(func(taskio.Target) bool) {
	return func(yield func(taskio.Target) bool) {
		for _, v := range m {
			if !yield(v) {
				return
			}
		}
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// fail records one refusal of a row.
func (row *importRow) fail(format string, args ...any) {
	row.out.Errors = append(row.out.Errors, fmt.Sprintf(format, args...))
}

// applyRow decides a row's action and writes it, unlinked.
func (i *taskImport) applyRow(ctx context.Context, m *mutation, row *importRow) error {
	d := row.draft
	for _, e := range d.Errors {
		row.fail("%s", e)
	}
	if d.Skip != "" {
		row.out.Action, row.out.Reason = core.ImportRowSkip, d.Skip
		return nil
	}
	existing, err := i.match(ctx, m.tx, d)
	if err != nil {
		return err
	}
	if existing != nil && i.result.Mode == core.TaskImportSkip {
		row.out.Action, row.out.Ref, row.out.Reason = core.ImportRowSkip, existing.Ref, "already imported as "+existing.Ref
		row.taskID = existing.ID
		i.remember(d, existing.ID)
		return nil
	}
	if existing != nil && i.result.Mode == core.TaskImportCreate {
		existing = nil
	}
	fields, skip, err := i.resolve(ctx, m.tx, row, existing)
	if err != nil {
		return err
	}
	if skip != "" {
		row.out.Action, row.out.Reason = core.ImportRowSkip, skip
		return nil
	}
	if len(row.out.Errors) > 0 {
		row.out.Action = core.ImportRowError
		return nil
	}
	if existing == nil {
		return i.create(ctx, m, row, fields)
	}
	return i.update(ctx, m, row, existing, fields)
}

// remember lets later rows of the file name this row by any key it carries.
func (i *taskImport) remember(d taskio.Draft, taskID string) {
	for _, k := range append([]string{d.ExternalID, d.Ref}, d.Aliases...) {
		if k = strings.ToLower(strings.TrimSpace(k)); k != "" {
			i.keys[k] = taskID
		}
	}
}

// match finds the task a row already became: by its external id under this
// import's source, then by its reference within the project.
func (i *taskImport) match(ctx context.Context, tx store.Tx, d taskio.Draft) (*core.Task, error) {
	if ext := externalKey(d); ext != "" {
		ref, err := tx.GetExternalRef(ctx, i.system, ext, syncEntityTask)
		switch {
		case err == nil:
			t, err := tx.GetTask(ctx, core.TaskRef{ID: ref.EntityID})
			if err == nil && !t.Deleted() && t.ProjectID == i.project.ID {
				return t, nil
			}
			if err != nil && !core.IsKind(err, core.KindNotFound) {
				return nil, err
			}
		case !core.IsKind(err, core.KindNotFound):
			return nil, err
		}
	}
	if d.Ref == "" {
		return nil, nil
	}
	t, err := taskByRef(ctx, tx, d.Ref)
	if core.IsKind(err, core.KindNotFound) || core.IsKind(err, core.KindInvalid) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if t.Deleted() || t.ProjectID != i.project.ID {
		return nil, nil
	}
	return t, nil
}

// rowFields are a row's values in stored form.
type rowFields struct {
	status     string
	assignee   string
	custom     map[string]any
	recurrence string
}

// resolve turns a row's status, assignee, custom fields and repeat rule into
// stored values, applying the unknown-value policies. A non-empty skip says
// the row is left out on purpose.
func (i *taskImport) resolve(ctx context.Context, tx store.Tx, row *importRow, existing *core.Task) (rowFields, string, error) {
	d := row.draft
	out := rowFields{custom: map[string]any{}}
	if d.Title == "" && existing == nil && !slices.Contains(row.out.Errors, "title is empty") {
		row.fail("title is empty")
	}
	if len(d.Title) > core.MaxTitleLength {
		row.fail("title is longer than %d characters", core.MaxTitleLength)
	}
	if len(d.Body) > core.MaxBodyLength {
		row.fail("description is longer than %d characters", core.MaxBodyLength)
	}
	status, skip := i.values.status(d.Status)
	if skip != "" {
		return out, skip, nil
	}
	switch {
	case status == "" && d.Status != "":
		row.fail("status %q is not a state of this project's workflow; map it to one", d.Status)
	case status == "" && existing == nil:
		out.status = i.values.defaultStatus()
	default:
		out.status = status
	}
	assignee, skip, err := i.assignee(ctx, tx, row)
	if err != nil || skip != "" {
		return out, skip, err
	}
	out.assignee = assignee
	for key, raw := range d.Custom {
		def, ok := i.defs[key]
		if !ok {
			row.fail("custom field %q is not defined for this project", key)
			continue
		}
		v, err := importFieldValue(def, raw, i.mapping.DateFormat)
		if err != nil {
			row.fail("%s", errMessage(err))
			continue
		}
		out.custom[key] = v
	}
	if d.Recurrence != "" {
		rule, err := core.NormalizeRecurrence(d.Recurrence)
		if err != nil {
			row.fail("repeat rule %q: %s", d.Recurrence, errMessage(err))
		}
		out.recurrence = rule
	}
	return out, "", nil
}

// assignee resolves a row's assignee, after its value mapping, to an actor.
func (i *taskImport) assignee(ctx context.Context, tx store.Tx, row *importRow) (string, string, error) {
	name := row.draft.Assignee
	if name == "" {
		return "", "", nil
	}
	id, known := i.actors[strings.ToLower(name)]
	if !known {
		resolved, err := resolveAssignee(ctx, tx, name)
		switch {
		case err == nil:
			id = resolved
		case !core.IsKind(err, core.KindNotFound) && !core.IsKind(err, core.KindInvalid):
			return "", "", err
		}
		i.actors[strings.ToLower(name)] = id
	}
	if id != "" {
		return id, "", nil
	}
	switch i.mapping.Policy(taskio.TargetAssignee) {
	case taskio.PolicySkipRow:
		return "", "assignee " + name + " is not a tix actor", nil
	case taskio.PolicyError, taskio.PolicyKeep:
		row.fail("assignee %q is not a tix actor; map it to a handle", name)
	}
	return "", "", nil
}

// create writes a new task from a row, with its tags and external reference.
func (i *taskImport) create(ctx context.Context, m *mutation, row *importRow, f rowFields) error {
	d := row.draft
	custom, err := validateTaskFields(slices.Collect(defValues(i.defs)), f.custom, true)
	if err != nil {
		row.fail("%s", errMessage(err))
		row.out.Action = core.ImportRowError
		return nil
	}
	seq, err := m.tx.NextSeq(ctx, i.project.ID)
	if err != nil {
		return err
	}
	priority := d.Priority
	if priority == 0 {
		priority = core.PriorityNormal
	}
	task := &core.Task{
		ProjectID: i.project.ID, Seq: seq, Title: d.Title, Body: d.Body, Status: f.status,
		Priority: priority, AssigneeActorID: f.assignee, CreatorActorID: i.actor.ID,
		DueAt: d.Due, CustomFields: custom, Recurrence: f.recurrence,
	}
	if i.workflow.IsTerminal(task.Status) {
		completed := m.now
		task.CompletedAt = &completed
	}
	if err := m.tx.CreateTask(ctx, task); err != nil {
		return err
	}
	for _, name := range d.Tags {
		if _, err := attachTaskLabel(ctx, m.tx, task.ID, name); err != nil {
			row.fail("tag %q: %s", name, errMessage(err))
		}
	}
	if err := i.externalRef(ctx, m, d, task.ID); err != nil {
		return err
	}
	stored, err := m.tx.GetTask(ctx, core.TaskRef{ID: task.ID})
	if err != nil {
		return err
	}
	if err := m.Record("task.create", core.EventTaskCreated, "task", task.ID, i.project.ID, nil, stored,
		map[string]any{"ref": stored.Ref, "title": stored.Title, "status": stored.Status, "import": i.system}); err != nil {
		return err
	}
	row.taskID, row.task = task.ID, stored
	row.out.Action = core.ImportRowCreate
	if len(row.out.Errors) > 0 {
		row.out.Action = core.ImportRowError
	}
	i.describe(row, stored)
	i.remember(d, task.ID)
	return nil
}

func defValues(m map[string]core.FieldDef) func(func(core.FieldDef) bool) {
	return func(yield func(core.FieldDef) bool) {
		for _, v := range m {
			if !yield(v) {
				return
			}
		}
	}
}

// update writes a row over the task it matched. Only what the row carries
// changes: an empty cell leaves the stored value alone.
func (i *taskImport) update(ctx context.Context, m *mutation, row *importRow, existing *core.Task, f rowFields) error {
	d := row.draft
	before := *existing
	after := *existing
	if d.Title != "" {
		after.Title = d.Title
	}
	if d.Body != "" {
		after.Body = d.Body
	}
	if f.status != "" && f.status != after.Status {
		after.Status = f.status
		switch {
		case i.workflow.IsTerminal(after.Status) && after.CompletedAt == nil:
			completed := m.now
			after.CompletedAt = &completed
		case !i.workflow.IsTerminal(after.Status):
			after.CompletedAt = nil
		}
	}
	if d.Priority != 0 {
		after.Priority = d.Priority
	}
	if f.assignee != "" {
		after.AssigneeActorID = f.assignee
	}
	if d.Due != nil {
		after.DueAt = d.Due
	}
	if f.recurrence != "" {
		after.Recurrence = f.recurrence
	}
	if len(f.custom) > 0 {
		merged, err := mergeTaskFields(slices.Collect(defValues(i.defs)), existing.CustomFields, f.custom)
		if err != nil {
			row.fail("%s", errMessage(err))
			row.out.Action = core.ImportRowError
			return nil
		}
		after.CustomFields = merged
	}
	if len(d.Tags) > 0 {
		after.Tags = d.Tags
	}
	if len(changedTaskFields(&before, &after)) == 0 {
		row.out.Action, row.out.Reason = core.ImportRowSkip, "unchanged"
		row.taskID, row.task = existing.ID, existing
		i.describe(row, existing)
		i.remember(d, existing.ID)
		return nil
	}
	after.Tags = before.Tags
	if err := m.tx.UpdateTask(ctx, &after); err != nil {
		return err
	}
	if len(d.Tags) > 0 {
		if err := replaceTaskLabels(ctx, m.tx, &after, d.Tags); err != nil {
			row.fail("tags: %s", errMessage(err))
		}
	}
	if err := i.externalRef(ctx, m, d, after.ID); err != nil {
		return err
	}
	stored, err := m.tx.GetTask(ctx, core.TaskRef{ID: after.ID})
	if err != nil {
		return err
	}
	if err := m.Record("task.update", core.EventTaskUpdated, "task", after.ID, i.project.ID, before, stored,
		map[string]any{"ref": stored.Ref, "fields": changedTaskFields(&before, stored), "import": i.system}); err != nil {
		return err
	}
	row.taskID, row.task = after.ID, stored
	row.out.Action = core.ImportRowUpdate
	if len(row.out.Errors) > 0 {
		row.out.Action = core.ImportRowError
	}
	i.describe(row, stored)
	i.remember(d, after.ID)
	return nil
}

// externalRef records the row's external id so the next import of the same
// file updates the task rather than creating it again.
func (i *taskImport) externalRef(ctx context.Context, m *mutation, d taskio.Draft, taskID string) error {
	ext := externalKey(d)
	if ext == "" {
		return nil
	}
	return m.tx.PutExternalRef(ctx, &core.ExternalRef{
		EntityType: syncEntityTask, EntityID: taskID, System: i.system,
		ExternalID: ext, LastSyncedAt: m.now,
	})
}

// externalKey is what a row is known by in its source: its external id, or,
// in a file of another project's tasks, the reference it had there.
func externalKey(d taskio.Draft) string {
	if d.ExternalID != "" {
		return d.ExternalID
	}
	return strings.ToLower(d.Ref)
}

// describe fills a row's report from the task it wrote.
func (i *taskImport) describe(row *importRow, t *core.Task) {
	row.out.Ref, row.out.Title, row.out.Status, row.out.Priority = t.Ref, t.Title, t.Status, t.Priority
	row.out.Tags, row.out.Due = t.Tags, t.DueAt
	if t.AssigneeActorID != "" {
		row.out.Assignee = row.draft.Assignee
	}
}

// linkRow sets a written row's parent and dependencies, each named by a key
// of another row of the file or by the reference of a task tix already has.
func (i *taskImport) linkRow(ctx context.Context, m *mutation, row *importRow) error {
	if row.task == nil {
		return nil
	}
	d := row.draft
	if d.Parent != "" {
		parent, err := i.linked(ctx, m.tx, d.Parent)
		if err != nil {
			return err
		}
		switch {
		case parent == "":
			row.fail("parent %q is neither a row of this file nor a task of the project", d.Parent)
		case parent != row.task.ParentID:
			cyclic, err := closesParentCycle(ctx, row.task.ID, parent, storeParentOf(m.tx))
			if err != nil {
				return err
			}
			if cyclic {
				row.fail("parent %q would make the task its own ancestor", d.Parent)
				break
			}
			task, err := m.tx.GetTask(ctx, core.TaskRef{ID: row.task.ID})
			if err != nil {
				return err
			}
			task.ParentID = parent
			if err := m.tx.UpdateTask(ctx, task); err != nil {
				return err
			}
			row.out.Parent = d.Parent
			if pt, err := m.tx.GetTask(ctx, core.TaskRef{ID: parent}); err == nil {
				row.out.Parent = pt.Ref
			}
			if row.out.Action == core.ImportRowSkip {
				row.out.Action, row.out.Reason = core.ImportRowUpdate, ""
			}
		}
	}
	for _, name := range d.DependsOn {
		depID, err := i.linked(ctx, m.tx, name)
		if err != nil {
			return err
		}
		if depID == "" {
			row.fail("dependency %q is neither a row of this file nor a task of the project", name)
			continue
		}
		dep, err := m.tx.GetTask(ctx, core.TaskRef{ID: depID})
		if err != nil {
			return err
		}
		exists := slices.Contains(row.task.DependsOn, depID)
		if exists {
			continue
		}
		if err := linkDependency(ctx, m.tx, row.task, dep); err != nil {
			if !core.IsKind(err, core.KindInvalid) {
				return err
			}
			row.fail("dependency %q: %s", name, errMessage(err))
		}
	}
	if len(row.out.Errors) > 0 {
		row.out.Action = core.ImportRowError
	}
	return nil
}

// linked resolves a parent or dependency name: a key of a row of this file
// first, then a task reference within the project.
func (i *taskImport) linked(ctx context.Context, tx store.Tx, name string) (string, error) {
	if id, ok := i.keys[strings.ToLower(strings.TrimSpace(name))]; ok {
		return id, nil
	}
	t, err := taskByRef(ctx, tx, name)
	switch {
	case core.IsKind(err, core.KindNotFound) || core.IsKind(err, core.KindInvalid):
		return "", nil
	case err != nil:
		return "", err
	case t.Deleted() || t.ProjectID != i.project.ID:
		return "", nil
	}
	return t.ID, nil
}

// report counts the rows, keeps the first ones as the preview and lists
// every refusal.
func (i *taskImport) report() {
	limit := i.in.Preview
	if limit == 0 {
		limit = defaultImportPreview
	}
	for _, row := range i.rows {
		switch row.out.Action {
		case core.ImportRowCreate:
			i.result.Created++
		case core.ImportRowUpdate:
			i.result.Updated++
		case core.ImportRowSkip:
			i.result.Skipped++
		default:
			row.out.Action = core.ImportRowError
			i.result.Failed++
			for _, e := range row.out.Errors {
				if len(i.result.Errors) < core.MaxImportErrors {
					i.result.Errors = append(i.result.Errors, core.TaskImportError{Line: row.out.Line, Message: e})
				}
			}
		}
		if len(i.result.Preview) < limit {
			i.result.Preview = append(i.result.Preview, row.out)
		}
	}
	i.result.Values = i.values.report()
	effective := *i.mapping
	if !i.file.lines {
		effective.Columns = make(map[string]taskio.Target, len(i.columns))
		for h, t := range i.columns {
			effective.Columns[h] = t
		}
		effective.Values = i.values.mapped()
	}
	if raw, err := effective.Marshal(); err == nil {
		i.result.Mapping = string(raw)
	}
}

// importFieldValue reads one cell as a custom field's value.
func importFieldValue(def core.FieldDef, raw, dateFormat string) (any, error) {
	raw = strings.TrimSpace(raw)
	switch def.Type {
	case core.FieldInt:
		n, err := strconv.ParseInt(strings.ReplaceAll(raw, " ", ""), 10, 64)
		if err != nil {
			f, ferr := strconv.ParseFloat(raw, 64)
			if ferr != nil || f != float64(int64(f)) {
				return nil, core.Invalid("custom field %q needs a whole number, not %q", def.Key, raw)
			}
			n = int64(f)
		}
		return n, nil
	case core.FieldFloat:
		f, err := strconv.ParseFloat(strings.Replace(raw, ",", ".", 1), 64)
		if err != nil {
			return nil, core.Invalid("custom field %q needs a number, not %q", def.Key, raw)
		}
		return f, nil
	case core.FieldBool:
		switch strings.ToLower(raw) {
		case "true", "yes", "y", "1", "on", "да":
			return true, nil
		case "false", "no", "n", "0", "off", "не":
			return false, nil
		}
		return nil, core.Invalid("custom field %q needs yes or no, not %q", def.Key, raw)
	case core.FieldDate, core.FieldDateTime:
		t, err := taskio.ParseDate(raw, dateFormat)
		if err != nil {
			return nil, core.Invalid("custom field %q: %s", def.Key, errMessage(err))
		}
		return coerceFieldValue(def, t)
	case core.FieldEnum:
		for _, opt := range def.EnumOptions {
			if strings.EqualFold(opt, raw) {
				return opt, nil
			}
		}
		return coerceFieldValue(def, raw)
	case core.FieldJSON:
		var v any
		if json.Valid([]byte(raw)) {
			_ = json.Unmarshal([]byte(raw), &v)
			return v, nil
		}
		return raw, nil
	default:
		return coerceFieldValue(def, raw)
	}
}

// errMessage is an error's message without its kind, for a row's report.
func errMessage(err error) string {
	var ce *core.Error
	if errors.As(err, &ce) {
		return ce.Message
	}
	return err.Error()
}
