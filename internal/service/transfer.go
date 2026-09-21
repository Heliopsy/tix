package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/transfer"
)

// auditImport names the audit action an import records for the whole run.
const auditImport = "import"

// transferPage bounds how many rows a snapshot walk holds at once, so export
// stays bounded by this constant rather than by the size of the tenant.
const transferPage = 200

// ExportTo streams a snapshot of the caller's tenant to w as it walks the data.
func (l *Local) ExportTo(ctx context.Context, in core.ExportInput, w io.Writer) error {
	actor, err := l.authorize(ctx, authz.ActionExport, authz.Resource{})
	if err != nil {
		return err
	}
	enc := transfer.NewEncoder(w)
	return l.read(ctx, actor, func(tx store.Tx) error {
		tenant, err := tx.GetTenant(ctx)
		if err != nil {
			return err
		}
		if err := enc.Header(tenant.Key, l.clock.Now()); err != nil {
			return err
		}
		projects, err := exportProjects(ctx, tx, in.ProjectRefs)
		if err != nil {
			return err
		}
		if err := writeWorkflows(ctx, tx, enc, projects, len(in.ProjectRefs) > 0); err != nil {
			return err
		}
		if err := writeProjects(ctx, tx, enc, projects); err != nil {
			return err
		}
		if err := writeTags(ctx, tx, enc); err != nil {
			return err
		}
		return writeTasks(ctx, tx, enc, projects, in)
	})
}

// exportProjects resolves the requested projects, or every project, in key order.
func exportProjects(ctx context.Context, tx store.Tx, refs []string) ([]core.Project, error) {
	var out []core.Project
	if len(refs) == 0 {
		all, err := listAllProjects(ctx, tx)
		if err != nil {
			return nil, err
		}
		out = all
	} else {
		seen := map[string]bool{}
		for _, ref := range refs {
			p, err := lookupProject(ctx, tx, ref)
			if err != nil {
				return nil, err
			}
			if seen[p.ID] {
				continue
			}
			seen[p.ID] = true
			out = append(out, *p)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Key < out[b].Key })
	return out, nil
}

// listAllProjects pages through every project of the tenant, archived included.
func listAllProjects(ctx context.Context, tx store.Tx) ([]core.Project, error) {
	f := core.ProjectFilter{
		IncludeArchived: true,
		Page:            core.Page{Limit: transferPage, Sort: core.SortCreatedAt, Direction: core.Ascending},
	}
	var out []core.Project
	for {
		page, err := tx.ListProjects(ctx, f)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		cursor := nextProjectCursor(f.Page, page)
		if cursor == "" {
			return out, nil
		}
		f.Page.Cursor = cursor
	}
}

// projectIDSet indexes the exported projects by identifier.
func projectIDSet(projects []core.Project) map[string]bool {
	out := make(map[string]bool, len(projects))
	for _, p := range projects {
		out[p.ID] = true
	}
	return out
}

// writeWorkflows emits the workflows a snapshot needs, in key order.
func writeWorkflows(ctx context.Context, tx store.Tx, enc *transfer.Encoder, projects []core.Project, filtered bool) error {
	flows, err := tx.ListWorkflows(ctx)
	if err != nil {
		return err
	}
	used := make(map[string]bool, len(projects))
	for _, p := range projects {
		used[p.WorkflowID] = true
	}
	for _, wf := range flows {
		if filtered && !used[wf.ID] {
			continue
		}
		wf.TenantID = ""
		if err := enc.Encode(core.SnapshotRecord{Kind: core.RecordWorkflow, Workflow: &wf}); err != nil {
			return err
		}
	}
	return nil
}

// writeProjects emits the projects and then their field definitions.
func writeProjects(ctx context.Context, tx store.Tx, enc *transfer.Encoder, projects []core.Project) error {
	for _, p := range projects {
		p.TenantID = ""
		if err := enc.Encode(core.SnapshotRecord{Kind: core.RecordProject, Project: &p}); err != nil {
			return err
		}
	}
	for _, p := range projects {
		defs, err := tx.ListFieldDefs(ctx, p.ID)
		if err != nil {
			return err
		}
		for _, d := range defs {
			d.TenantID = ""
			if err := enc.Encode(core.SnapshotRecord{Kind: core.RecordFieldDef, FieldDef: &d}); err != nil {
				return err
			}
		}
	}
	return nil
}

// writeTags emits the tenant's tags in name order.
func writeTags(ctx context.Context, tx store.Tx, enc *transfer.Encoder) error {
	tags, err := tx.ListTags(ctx)
	if err != nil {
		return err
	}
	for _, tag := range tags {
		tag.TenantID = ""
		if err := enc.Encode(core.SnapshotRecord{Kind: core.RecordLabel, Tag: &tag}); err != nil {
			return err
		}
	}
	return nil
}

// writeTasks emits tasks, then the relationships and attachments that reference
// them, each in its own pass so nothing is buffered between them. Tasks are
// walked with tombstones included: a soft-deleted task still travels, as a
// record carrying its deleted_at, so an import elsewhere can tell a deletion
// happened rather than silently resurrecting the row. Its dependencies,
// comments and artifacts do not travel; they are of no use once the task is
// dead, and the deleted task itself is out of scope for a live task's own
// references (see taskInScope).
func writeTasks(ctx context.Context, tx store.Tx, enc *transfer.Encoder, projects []core.Project, in core.ExportInput) error {
	scope := projectIDSet(projects)
	for _, p := range projects {
		if err := eachTask(ctx, tx, p.ID, true, func(t core.Task) error {
			out, err := exportTask(ctx, tx, t, scope)
			if err != nil {
				return err
			}
			return enc.Encode(core.SnapshotRecord{Kind: core.RecordTask, Task: out})
		}); err != nil {
			return err
		}
	}
	for _, p := range projects {
		if err := eachTask(ctx, tx, p.ID, false, func(t core.Task) error {
			return writeDependencies(ctx, tx, enc, t, scope)
		}); err != nil {
			return err
		}
	}
	if in.IncludeComments {
		for _, p := range projects {
			if err := eachTask(ctx, tx, p.ID, false, func(t core.Task) error {
				return writeComments(ctx, tx, enc, t)
			}); err != nil {
				return err
			}
		}
	}
	if !in.IncludeArtifacts {
		return nil
	}
	for _, p := range projects {
		if err := eachTask(ctx, tx, p.ID, false, func(t core.Task) error {
			return writeArtifacts(ctx, tx, enc, t)
		}); err != nil {
			return err
		}
	}
	return nil
}

// eachTask streams one project's tasks in sequence order, including
// soft-deleted tasks when includeDeleted is set.
func eachTask(ctx context.Context, tx store.Tx, projectID string, includeDeleted bool, fn func(core.Task) error) error {
	f := core.TaskFilter{
		ProjectIDs:     []string{projectID},
		IncludeDeleted: includeDeleted,
		Page:           core.Page{Limit: transferPage, Sort: core.SortSeq, Direction: core.Ascending},
	}
	for {
		tasks, err := tx.ListTasks(ctx, f)
		if err != nil {
			return err
		}
		for _, t := range tasks {
			if err := fn(t); err != nil {
				return err
			}
		}
		if len(tasks) < transferPage {
			return nil
		}
		last := tasks[len(tasks)-1]
		f.Page.Cursor = core.Cursor{
			SortValue: taskSortValue(core.SortSeq, last),
			ID:        last.ID,
			Sort:      core.SortSeq,
			Direction: core.Ascending,
		}.Encode()
	}
}

// exportTask drops the transient claim state and any reference that leaves the
// exported scope, so a filtered snapshot never names a row it does not carry.
func exportTask(ctx context.Context, tx store.Tx, t core.Task, scope map[string]bool) (*core.Task, error) {
	t.TenantID = ""
	t.DependsOn = nil
	t.Blocked = false
	t.ClaimedByActorID = ""
	t.ClaimedAt = nil
	t.LeaseExpiresAt = nil
	t.ClaimCount = 0
	if t.ParentID != "" {
		in, err := taskInScope(ctx, tx, t.ParentID, scope)
		if err != nil {
			return nil, err
		}
		if !in {
			t.ParentID = ""
		}
	}
	return &t, nil
}

// taskInScope reports whether a referenced task is part of this export.
func taskInScope(ctx context.Context, tx store.Tx, taskID string, scope map[string]bool) (bool, error) {
	other, err := tx.GetTask(ctx, core.TaskRef{ID: taskID})
	if core.IsKind(err, core.KindNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !other.Deleted() && scope[other.ProjectID], nil
}

// writeDependencies emits a task's edges whose other end is also exported.
func writeDependencies(ctx context.Context, tx store.Tx, enc *transfer.Encoder, t core.Task, scope map[string]bool) error {
	deps, err := tx.ListDependencies(ctx, t.ID)
	if err != nil {
		return err
	}
	for _, d := range deps {
		in, err := taskInScope(ctx, tx, d.DependsOn, scope)
		if err != nil {
			return err
		}
		if !in {
			continue
		}
		d.TenantID = ""
		if err := enc.Encode(core.SnapshotRecord{Kind: core.RecordDependency, Dependency: &d}); err != nil {
			return err
		}
	}
	return nil
}

// writeComments emits a task's live comments oldest first.
func writeComments(ctx context.Context, tx store.Tx, enc *transfer.Encoder, t core.Task) error {
	comments, err := tx.ListComments(ctx, t.ID)
	if err != nil {
		return err
	}
	for _, c := range comments {
		c.TenantID = ""
		if err := enc.Encode(core.SnapshotRecord{Kind: core.RecordComment, Comment: &c}); err != nil {
			return err
		}
	}
	return nil
}

// writeArtifacts emits a task's artifacts oldest first.
func writeArtifacts(ctx context.Context, tx store.Tx, enc *transfer.Encoder, t core.Task) error {
	artifacts, err := tx.ListArtifacts(ctx, t.ID)
	if err != nil {
		return err
	}
	for _, a := range artifacts {
		a.TenantID = ""
		if err := enc.Encode(core.SnapshotRecord{Kind: core.RecordArtifact, Artifact: &a}); err != nil {
			return err
		}
	}
	return nil
}

// ImportFrom reads a snapshot record by record and applies it atomically.
func (l *Local) ImportFrom(ctx context.Context, r io.Reader, in core.ImportInput) (*core.ImportResult, error) {
	actor, err := l.authorize(ctx, authz.ActionImport, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	dec := transfer.NewDecoder(r)
	if _, err := dec.Header(); err != nil {
		return nil, err
	}

	imp := newImporter(l, actor, in, dec)
	if in.DryRun {
		if err := l.read(ctx, actor, func(tx store.Tx) error { return imp.run(ctx, tx, nil) }); err != nil {
			return nil, err
		}
		return imp.result, nil
	}
	if err := l.write(ctx, core.SystemActor(actor.TenantID), func(m *mutation) error {
		return imp.run(ctx, m.tx, m)
	}); err != nil {
		return nil, err
	}
	return imp.result, nil
}

// parentLink defers a subtask's parent until the whole task stream is read,
// because a parent may carry a higher sequence number than its child.
type parentLink struct {
	taskID   string
	parentID string
	line     int
}

// importer applies one streamed snapshot.
type importer struct {
	local  *Local
	caller *core.Actor
	mode   core.ImportMode
	dryRun bool
	dec    *transfer.Decoder
	result *core.ImportResult

	ids          map[string]string
	projects     map[string]*core.Project
	flows        map[string]*core.Workflow
	defs         map[string][]core.FieldDef
	importedDefs map[string]map[string]bool
	taskOwner    map[string]string
	lastFlow     string
	parents      []parentLink
	problems     []string
}

// newImporter builds an importer for one run.
func newImporter(l *Local, actor *core.Actor, in core.ImportInput, dec *transfer.Decoder) *importer {
	return &importer{
		local: l, caller: actor, mode: in.Mode, dryRun: in.DryRun, dec: dec,
		result: &core.ImportResult{
			Created: map[string]int{}, Updated: map[string]int{},
			Skipped: map[string]int{}, Deleted: map[string]int{}, DryRun: in.DryRun,
		},
		ids:          map[string]string{},
		projects:     map[string]*core.Project{},
		flows:        map[string]*core.Workflow{},
		defs:         map[string][]core.FieldDef{},
		importedDefs: map[string]map[string]bool{},
		taskOwner:    map[string]string{},
	}
}

// run consumes the stream and applies it. A nil mutation means a dry run, which
// decides everything a real run decides and writes nothing.
func (i *importer) run(ctx context.Context, tx store.Tx, m *mutation) error {
	for {
		rec, err := i.dec.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if err := i.apply(ctx, tx, m, rec); err != nil {
			return err
		}
	}
	if err := i.linkParents(ctx, tx); err != nil {
		return err
	}
	if len(i.problems) > 0 {
		return core.Invalid("snapshot failed validation: %s", strings.Join(i.problems, "; "))
	}
	if err := i.reconcile(ctx, tx, m); err != nil {
		return err
	}
	if m == nil {
		return nil
	}
	return m.Record(auditImport, core.EventImportCompleted, "tenant", i.caller.TenantID, "",
		nil, i.result, map[string]any{"mode": string(i.mode)})
}

// apply dispatches one record.
func (i *importer) apply(ctx context.Context, tx store.Tx, m *mutation, rec *core.SnapshotRecord) error {
	switch rec.Kind {
	case core.RecordHeader:
		return nil
	case core.RecordWorkflow:
		return i.applyWorkflow(ctx, tx, m, *rec.Workflow)
	case core.RecordProject:
		return i.applyProject(ctx, tx, m, *rec.Project)
	case core.RecordFieldDef:
		return i.applyFieldDef(ctx, tx, m, *rec.FieldDef)
	case core.RecordLabel:
		return i.applyTag(ctx, tx, *rec.Tag)
	case core.RecordTask:
		return i.applyTask(ctx, tx, m, *rec.Task)
	case core.RecordDependency:
		return i.applyDependency(ctx, tx, m, *rec.Dependency)
	case core.RecordComment:
		return i.applyComment(ctx, tx, m, *rec.Comment)
	case core.RecordArtifact:
		return i.applyArtifact(ctx, tx, m, *rec.Artifact)
	default:
		return core.Invalid("snapshot line %d has unsupported record kind %q", i.dec.Line(), rec.Kind)
	}
}

// reject records a validation failure and carries on, so one import reports
// every problem rather than stopping at the first.
func (i *importer) reject(format string, args ...any) error {
	i.problems = append(i.problems, fmt.Sprintf(format, args...))
	return nil
}

// count records one record's outcome.
func (i *importer) count(kind core.RecordKind, update bool) {
	if update {
		i.result.Updated[string(kind)]++
		return
	}
	i.result.Created[string(kind)]++
}

// mapID remembers which target row a snapshot identifier became.
func (i *importer) mapID(snapshotID, targetID string) {
	if snapshotID != "" {
		i.ids[snapshotID] = targetID
	}
}

// plannedID is the identifier a dry run assumes a new row would be given.
func (i *importer) plannedID(snapshotID string) string {
	if snapshotID == "" {
		return i.local.ids.New()
	}
	i.mapID(snapshotID, snapshotID)
	return snapshotID
}

// createWithID writes a new row under the snapshot's identifier, and retries
// under a fresh one when that identifier is already used by a row this tenant
// cannot see. Every reference to it is rewritten through the identifier map.
func (i *importer) createWithID(kind core.RecordKind, snapshotID string, write func(string) error) (string, error) {
	target := snapshotID
	if target == "" {
		target = i.local.ids.New()
	}
	err := write(target)
	if err == nil {
		i.mapID(snapshotID, target)
		return target, nil
	}
	if !core.IsKind(err, core.KindConflict) {
		return "", err
	}
	fresh := i.local.ids.New()
	if err := write(fresh); err != nil {
		return "", err
	}
	i.mapID(snapshotID, fresh)
	i.result.Warnings = append(i.result.Warnings,
		fmt.Sprintf("remapped %s %s to %s", kind, snapshotID, fresh))
	return fresh, nil
}

// applyWorkflow creates or updates one workflow, matched by key.
func (i *importer) applyWorkflow(ctx context.Context, tx store.Tx, m *mutation, w core.Workflow) error {
	w.Key = strings.ToLower(strings.TrimSpace(w.Key))
	name := w.Name
	if name == "" {
		name = w.Key
	}
	in := core.WorkflowInput{Key: w.Key, Name: name, Definition: w.Definition}
	if err := in.Validate(); err != nil {
		return i.reject("workflow at line %d is not valid: %v", i.dec.Line(), err)
	}
	existing, err := findWorkflow(ctx, tx, w.Key)
	if err != nil {
		return err
	}

	target := &core.Workflow{Key: w.Key, Name: name, Definition: w.Definition}
	i.count(core.RecordWorkflow, existing != nil)
	switch {
	case existing != nil:
		target.ID = existing.ID
		target.Builtin = existing.Builtin
		i.mapID(w.ID, existing.ID)
	case i.dryRun:
		target.ID = i.plannedID(w.ID)
	}
	i.flows[target.ID] = target
	i.lastFlow = target.ID
	if i.dryRun {
		return nil
	}
	if existing == nil {
		id, err := i.createWithID(core.RecordWorkflow, w.ID, func(id string) error {
			target.ID = id
			return tx.PutWorkflow(ctx, target)
		})
		if err != nil {
			return err
		}
		i.flows[id] = target
		i.lastFlow = id
	} else if err := tx.PutWorkflow(ctx, target); err != nil {
		return err
	}
	return m.Record(auditWorkflowPut, core.EventWorkflowUpdated, "workflow", target.ID, "",
		existing, target, map[string]any{"key": target.Key})
}

// applyProject creates or updates one project, matched by key.
func (i *importer) applyProject(ctx context.Context, tx store.Tx, m *mutation, p core.Project) error {
	p.Key = strings.ToLower(strings.TrimSpace(p.Key))
	if err := core.ValidateProjectKey(p.Key); err != nil {
		return i.reject("project at line %d is not valid: %v", i.dec.Line(), err)
	}
	if !p.Color.Valid() {
		return i.reject("project %q at line %d has colour %q, which is not in the palette", p.Key, i.dec.Line(), p.Color)
	}
	if _, err := core.NormalizeProjectIcon(p.Icon); err != nil {
		return i.reject("project %q at line %d has an invalid icon: %v", p.Key, i.dec.Line(), err)
	}
	existing, err := findProject(ctx, tx, p.Key)
	if err != nil {
		return err
	}
	workflowID, ok := i.workflowRef(p.WorkflowID)
	if !ok {
		if existing == nil {
			return i.reject("project %q at line %d references workflow %q, which the snapshot does not contain",
				p.Key, i.dec.Line(), p.WorkflowID)
		}
		workflowID = existing.WorkflowID
	}
	name := p.Name
	if name == "" {
		name = p.Key
	}

	target := &core.Project{
		Key: p.Key, Name: name, Description: p.Description,
		WorkflowID: workflowID, Color: p.Color, Icon: p.Icon,
		ArchivedAt: p.ArchivedAt, CreatedAt: p.CreatedAt,
	}
	i.count(core.RecordProject, existing != nil)
	switch {
	case existing != nil:
		target.ID = existing.ID
		i.mapID(p.ID, existing.ID)
		if !i.dryRun {
			if err := tx.UpdateProject(ctx, target); err != nil {
				return err
			}
		}
	case i.dryRun:
		target.ID = i.plannedID(p.ID)
	default:
		if _, err := i.createWithID(core.RecordProject, p.ID, func(id string) error {
			target.ID = id
			return tx.CreateProject(ctx, target)
		}); err != nil {
			return err
		}
	}
	i.projects[target.ID] = target
	if err := i.loadDefs(ctx, tx, target.ID, existing != nil); err != nil {
		return err
	}
	if i.dryRun {
		return nil
	}
	return m.Record(auditProjectCreate, core.EventProjectCreated, "project", target.ID, target.ID,
		existing, target, map[string]any{"key": target.Key})
}

// workflowRef resolves a project's workflow reference, treating an empty one as
// the workflow the snapshot declared last so a hand-written snapshot works.
func (i *importer) workflowRef(ref string) (string, bool) {
	if ref == "" {
		return i.lastFlow, i.lastFlow != ""
	}
	id, ok := i.ids[ref]
	return id, ok
}

// findProject returns a project by key, or nil when the tenant has none.
func findProject(ctx context.Context, tx store.Tx, key string) (*core.Project, error) {
	p, err := tx.GetProject(ctx, key)
	if core.IsKind(err, core.KindNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// loadDefs seeds the field definition cache a project's tasks validate against.
func (i *importer) loadDefs(ctx context.Context, tx store.Tx, projectID string, existing bool) error {
	if _, ok := i.defs[projectID]; ok {
		return nil
	}
	i.defs[projectID] = []core.FieldDef{}
	i.importedDefs[projectID] = map[string]bool{}
	if !existing {
		return nil
	}
	defs, err := tx.ListFieldDefs(ctx, projectID)
	if err != nil {
		return err
	}
	i.defs[projectID] = defs
	return nil
}

// applyFieldDef creates or updates one custom field definition.
func (i *importer) applyFieldDef(ctx context.Context, tx store.Tx, m *mutation, d core.FieldDef) error {
	projectID, ok := i.ids[d.ProjectID]
	if !ok {
		return i.reject("field definition at line %d belongs to project %q, which the snapshot does not contain",
			i.dec.Line(), d.ProjectID)
	}
	d.Key = strings.TrimSpace(d.Key)
	if d.Key == "" {
		return i.reject("field definition at line %d has no key", i.dec.Line())
	}
	if !d.Type.Valid() {
		return i.reject("field definition %q at line %d has unknown type %q", d.Key, i.dec.Line(), d.Type)
	}

	snapshotID := d.ID
	target := d
	target.TenantID = ""
	target.ProjectID = projectID
	target.ID = ""
	update := false
	for _, existing := range i.defs[projectID] {
		if existing.Key == d.Key {
			update = true
			target.ID = existing.ID
			i.mapID(snapshotID, existing.ID)
			break
		}
	}
	i.count(core.RecordFieldDef, update)
	if i.dryRun {
		if !update {
			target.ID = i.plannedID(snapshotID)
		}
		i.putDef(projectID, target)
		return nil
	}
	if update {
		if err := tx.PutFieldDef(ctx, &target); err != nil {
			return err
		}
	} else if _, err := i.createWithID(core.RecordFieldDef, snapshotID, func(id string) error {
		target.ID = id
		return tx.PutFieldDef(ctx, &target)
	}); err != nil {
		return err
	}
	i.putDef(projectID, target)
	return m.Record(auditFieldPut, core.EventFieldUpdated, "field", target.ID, projectID,
		nil, target, map[string]any{"key": target.Key})
}

// putDef records a definition in the cache the importer validates against.
func (i *importer) putDef(projectID string, d core.FieldDef) {
	if i.importedDefs[projectID] == nil {
		i.importedDefs[projectID] = map[string]bool{}
	}
	i.importedDefs[projectID][d.Key] = true
	defs := i.defs[projectID]
	for idx := range defs {
		if defs[idx].Key == d.Key {
			defs[idx] = d
			i.defs[projectID] = defs
			return
		}
	}
	i.defs[projectID] = append(defs, d)
}

// applyTag creates or updates one tag, matched by name within its project.
func (i *importer) applyTag(ctx context.Context, tx store.Tx, tag core.Tag) error {
	tag.Name = strings.TrimSpace(tag.Name)
	if tag.Name == "" {
		return i.reject("tag at line %d has no name", i.dec.Line())
	}
	projectID := ""
	if tag.ProjectID != "" {
		mapped, ok := i.ids[tag.ProjectID]
		if !ok {
			return i.reject("tag %q at line %d belongs to project %q, which the snapshot does not contain",
				tag.Name, i.dec.Line(), tag.ProjectID)
		}
		projectID = mapped
	}

	existing, err := findTag(ctx, tx, projectID, tag.Name)
	if err != nil {
		return err
	}
	target := &core.Tag{ProjectID: projectID, Name: tag.Name, Color: tag.Color}
	i.count(core.RecordLabel, existing != nil)
	if existing != nil {
		target.ID = existing.ID
		i.mapID(tag.ID, existing.ID)
		if i.dryRun {
			return nil
		}
		return tx.PutTag(ctx, target)
	}
	if i.dryRun {
		i.plannedID(tag.ID)
		return nil
	}
	_, err = i.createWithID(core.RecordLabel, tag.ID, func(id string) error {
		target.ID = id
		return tx.PutTag(ctx, target)
	})
	return err
}

// findTag returns a tag by project and name, or nil when it does not exist.
func findTag(ctx context.Context, tx store.Tx, projectID, name string) (*core.Tag, error) {
	tags, err := tx.ListTags(ctx)
	if err != nil {
		return nil, err
	}
	for idx := range tags {
		if tags[idx].Name == name && tags[idx].ProjectID == projectID {
			return &tags[idx], nil
		}
	}
	return nil, nil
}

// applyTask dispatches one task record once its identity has been resolved.
// Identity is the task's ULID, never the human-facing project-and-sequence
// reference: a sequence number is a per-project counter, so two databases can
// independently mint the same "infra-42" for unrelated tasks, and matching on
// it would silently merge them. A snapshot task record with no ULID (an old,
// hand-written, or otherwise stripped snapshot) cannot be matched to anything
// and is always created fresh; see matchTask.
func (i *importer) applyTask(ctx context.Context, tx store.Tx, m *mutation, t core.Task) error {
	line := i.dec.Line()
	projectID, ok := i.ids[t.ProjectID]
	if !ok {
		return i.reject("task %q at line %d belongs to project %q, which the snapshot does not contain",
			t.Title, line, t.ProjectID)
	}
	project := i.projects[projectID]
	existing, err := i.matchTask(ctx, tx, t.ID, projectID)
	if err != nil {
		return err
	}
	if t.Deleted() {
		return i.applyTaskDeletion(ctx, tx, m, t, project, projectID, existing, line)
	}
	return i.applyTaskLive(ctx, tx, m, t, project, projectID, existing, line)
}

// matchTask resolves the row a snapshot task's ULID names, scoped to the
// project the record is being imported into. An identifier that is blank,
// unused, or that belongs to a task in a different project is reported as no
// match: the caller then creates a new row, and if the identifier is already
// taken it is remapped the same way any other identifier collision is (see
// createWithID). Matching a same-ID row in a different project directly would
// silently move an unrelated task, so that case is deliberately treated as no
// match rather than as an update.
func (i *importer) matchTask(ctx context.Context, tx store.Tx, snapshotID, projectID string) (*core.Task, error) {
	if snapshotID == "" {
		return nil, nil
	}
	other, err := tx.GetTask(ctx, core.TaskRef{ID: snapshotID})
	if core.IsKind(err, core.KindNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if other.ProjectID != projectID {
		return nil, nil
	}
	return other, nil
}

// taskStale reports whether a snapshot record is older than the row it would
// replace, so an import never lets an older snapshot revert newer data or
// resurrect a task that was deleted more recently than the snapshot was taken.
// A record with no timestamp carries no evidence either way and is never
// treated as stale.
func taskStale(incoming, existing core.Task) bool {
	return !incoming.UpdatedAt.IsZero() && incoming.UpdatedAt.Before(existing.UpdatedAt)
}

// taskContentEqual reports whether target already matches what is stored, so
// an import that would change nothing writes nothing and audits nothing.
// Deletion state is compared by the caller, not here.
func taskContentEqual(existing, target *core.Task) bool {
	return existing.Title == target.Title &&
		existing.Body == target.Body &&
		existing.Status == target.Status &&
		existing.Priority == target.Priority &&
		existing.AssigneeActorID == target.AssigneeActorID &&
		timePtrEqual(existing.DueAt, target.DueAt) &&
		timePtrEqual(existing.StartedAt, target.StartedAt) &&
		timePtrEqual(existing.CompletedAt, target.CompletedAt) &&
		customFieldsEqual(existing.CustomFields, target.CustomFields) &&
		tagsEqual(existing.Tags, target.Tags)
}

// customFieldsEqual compares custom field values by their JSON representation
// rather than by Go type, because a value read back from storage and a value
// freshly decoded from a snapshot can differ only in numeric type (int64
// versus float64) while meaning the same thing.
func customFieldsEqual(a, b map[string]any) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	ab, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bb, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(ab) == string(bb)
}

// timePtrEqual compares two optional timestamps.
func timePtrEqual(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

// tagsEqual compares two tag sets without regard to order.
func tagsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sa, sb := append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(sa)
	sort.Strings(sb)
	for i := range sa {
		if sa[i] != sb[i] {
			return false
		}
	}
	return true
}

// buildLiveTaskTarget validates a snapshot task's content against the target
// project and returns the row it would become. ok is false when the record
// was rejected, in which case the rejection is already recorded and the
// caller simply stops processing this record.
func (i *importer) buildLiveTaskTarget(ctx context.Context, tx store.Tx, project *core.Project, projectID string, t core.Task, line int) (target *core.Task, ok bool, err error) {
	flow, err := i.workflowFor(ctx, tx, project)
	if err != nil {
		return nil, false, err
	}
	title := strings.TrimSpace(t.Title)
	if title == "" {
		return nil, false, i.reject("task at line %d has no title", line)
	}
	status := t.Status
	if status == "" {
		status = flow.Definition.Initial
	}
	if !flow.Definition.HasState(status) {
		return nil, false, i.reject("task %q at line %d is in state %q, which workflow %q does not define",
			t.Title, line, status, flow.Key)
	}
	fields, err := validateTaskFields(i.defs[projectID], t.CustomFields, false)
	if err != nil {
		return nil, false, i.reject("task %q at line %d: %v", t.Title, line, err)
	}
	creator, err := i.actorRef(ctx, tx, t.CreatorActorID, true)
	if err != nil {
		return nil, false, err
	}
	assignee, err := i.actorRef(ctx, tx, t.AssigneeActorID, false)
	if err != nil {
		return nil, false, err
	}

	target = &core.Task{
		ProjectID: projectID, Seq: t.Seq, Title: title, Body: t.Body,
		Status: status, Priority: t.Priority, AssigneeActorID: assignee, CreatorActorID: creator,
		DueAt: t.DueAt, StartedAt: t.StartedAt, CompletedAt: t.CompletedAt,
		CustomFields: fields, CreatedAt: t.CreatedAt, Tags: t.Tags,
	}
	if target.Priority == 0 {
		target.Priority = core.PriorityNormal
	}
	return target, true, nil
}

// avoidSeqCollision reassigns a genuinely new task's sequence number when the
// snapshot's number is already held by an unrelated task in the target
// project. This is only reachable once identity matching (see matchTask) has
// already decided this is not the same task: two independently seeded
// projects sharing a key can each mint a task numbered 1, and importing one
// into the other must not collide with, or silently reuse, a number some
// other task already holds.
func (i *importer) avoidSeqCollision(ctx context.Context, tx store.Tx, project *core.Project, target *core.Task, line int) error {
	if target.Seq <= 0 {
		return nil
	}
	_, err := tx.GetTask(ctx, core.TaskRef{ProjectKey: project.Key, Seq: target.Seq})
	if core.IsKind(err, core.KindNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	// The project's own counter cannot be trusted to mint the replacement: it
	// only advances when a task is created without an explicit number, and
	// every task an import creates carries one, so the counter can still read
	// zero in a project that already holds numbered tasks.
	fresh, err := nextAvailableSeq(ctx, tx, target.ProjectID)
	if err != nil {
		return err
	}
	i.result.Warnings = append(i.result.Warnings, fmt.Sprintf(
		"line %d: renumbered task %q from %s-%d to %s-%d, whose original number in project %q was already held by an unrelated task",
		line, target.Title, project.Key, target.Seq, project.Key, fresh, project.Key))
	target.Seq = fresh
	return nil
}

// nextAvailableSeq returns a sequence number no task, live or deleted,
// currently holds in the project.
func nextAvailableSeq(ctx context.Context, tx store.Tx, projectID string) (int64, error) {
	tasks, err := tx.ListTasks(ctx, core.TaskFilter{
		ProjectIDs:     []string{projectID},
		IncludeDeleted: true,
		Page:           core.Page{Limit: 1, Sort: core.SortSeq, Direction: core.Descending},
	})
	if err != nil {
		return 0, err
	}
	if len(tasks) == 0 {
		return 1, nil
	}
	return tasks[0].Seq + 1, nil
}

// applyTaskLive creates or updates a task the snapshot carries as live. A
// match against a newer target row is skipped rather than overwritten; a
// match against a target row that is currently deleted is resurrected only
// when the snapshot's record is newer than the deletion, since that is the
// only case in which the snapshot is known to postdate it.
func (i *importer) applyTaskLive(ctx context.Context, tx store.Tx, m *mutation, t core.Task, project *core.Project, projectID string, existing *core.Task, line int) error {
	target, ok, err := i.buildLiveTaskTarget(ctx, tx, project, projectID, t, line)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	if existing == nil {
		if err := i.avoidSeqCollision(ctx, tx, project, target, line); err != nil {
			return err
		}
		if i.dryRun {
			target.ID = i.plannedID(t.ID)
		} else if _, err := i.createWithID(core.RecordTask, t.ID, func(id string) error {
			target.ID = id
			return tx.CreateTask(ctx, target)
		}); err != nil {
			return err
		}
		i.count(core.RecordTask, false)
		i.taskOwner[target.ID] = projectID
		if t.ParentID != "" {
			i.parents = append(i.parents, parentLink{taskID: target.ID, parentID: t.ParentID, line: line})
		}
		if i.dryRun {
			return nil
		}
		if err := replaceTaskLabels(ctx, tx, target, t.Tags); err != nil {
			return err
		}
		return m.Record("task.create", core.EventTaskCreated, "task", target.ID, projectID,
			nil, target, map[string]any{"title": target.Title, "status": target.Status})
	}

	i.mapID(t.ID, existing.ID)
	i.taskOwner[existing.ID] = projectID
	if t.ParentID != "" {
		i.parents = append(i.parents, parentLink{taskID: existing.ID, parentID: t.ParentID, line: line})
	}

	if taskStale(t, *existing) {
		i.result.Skipped[string(core.RecordTask)]++
		i.result.Warnings = append(i.result.Warnings, fmt.Sprintf(
			"line %d: kept task %q, which was updated more recently than the snapshot's version", line, existing.Ref))
		return nil
	}

	target.ID = existing.ID
	target.Seq = existing.Seq
	resurrecting := existing.Deleted()
	if !resurrecting && taskContentEqual(existing, target) {
		i.result.Skipped[string(core.RecordTask)]++
		return nil
	}

	i.count(core.RecordTask, true)
	if i.dryRun {
		return nil
	}
	if resurrecting {
		if err := tx.RestoreTask(ctx, existing.ID); err != nil {
			return err
		}
	}
	if err := tx.UpdateTask(ctx, target); err != nil {
		return err
	}
	if err := replaceTaskLabels(ctx, tx, target, t.Tags); err != nil {
		return err
	}
	return m.Record("task.update", core.EventTaskUpdated, "task", target.ID, projectID,
		existing, target, map[string]any{"title": target.Title, "status": target.Status})
}

// applyTaskDeletion applies a tombstone. Against a row the target has never
// seen, the tombstone is still materialised so the sequence number and
// identity it names survive a round trip into an empty database. Against an
// existing row, the deletion is applied only when it is not older than the
// row's own last update, so a stale tombstone can never remove work a target
// has since built on; an already-deleted row is left alone rather than
// deleted again.
func (i *importer) applyTaskDeletion(ctx context.Context, tx store.Tx, m *mutation, t core.Task, project *core.Project, projectID string, existing *core.Task, line int) error {
	if existing == nil {
		target, ok, err := i.buildLiveTaskTarget(ctx, tx, project, projectID, t, line)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		target.DeletedAt = t.DeletedAt
		if err := i.avoidSeqCollision(ctx, tx, project, target, line); err != nil {
			return err
		}
		if i.dryRun {
			target.ID = i.plannedID(t.ID)
		} else if _, err := i.createWithID(core.RecordTask, t.ID, func(id string) error {
			target.ID = id
			return tx.CreateTask(ctx, target)
		}); err != nil {
			return err
		}
		i.result.Deleted[string(core.RecordTask)]++
		i.taskOwner[target.ID] = projectID
		if t.ParentID != "" {
			i.parents = append(i.parents, parentLink{taskID: target.ID, parentID: t.ParentID, line: line})
		}
		if i.dryRun {
			return nil
		}
		return m.Record("task.delete", core.EventTaskDeleted, "task", target.ID, projectID,
			nil, target, map[string]any{"ref": target.Ref})
	}

	i.mapID(t.ID, existing.ID)
	i.taskOwner[existing.ID] = projectID

	if taskStale(t, *existing) {
		i.result.Skipped[string(core.RecordTask)]++
		i.result.Warnings = append(i.result.Warnings, fmt.Sprintf(
			"line %d: kept task %q live, which was updated more recently than the deletion the snapshot carries", line, existing.Ref))
		return nil
	}
	if existing.Deleted() {
		i.result.Skipped[string(core.RecordTask)]++
		return nil
	}

	i.result.Deleted[string(core.RecordTask)]++
	if i.dryRun {
		return nil
	}
	if err := tx.DeleteTask(ctx, existing.ID, false); err != nil {
		return err
	}
	after := *existing
	after.DeletedAt = t.DeletedAt
	return m.Record("task.delete", core.EventTaskDeleted, "task", existing.ID, projectID,
		existing, after, map[string]any{"ref": existing.Ref})
}

// workflowFor returns the state machine a project's tasks are checked against.
func (i *importer) workflowFor(ctx context.Context, tx store.Tx, project *core.Project) (*core.Workflow, error) {
	if flow, ok := i.flows[project.WorkflowID]; ok {
		return flow, nil
	}
	flow, err := tx.GetWorkflowByID(ctx, project.WorkflowID)
	if err != nil {
		return nil, err
	}
	i.flows[flow.ID] = flow
	return flow, nil
}

// actorRef keeps an actor the target knows and otherwise falls back to the
// operator running the import, because a snapshot carries no actors.
func (i *importer) actorRef(ctx context.Context, tx store.Tx, actorID string, required bool) (string, error) {
	if actorID != "" {
		switch _, err := tx.GetActor(ctx, actorID); {
		case err == nil:
			return actorID, nil
		case !core.IsKind(err, core.KindNotFound):
			return "", err
		}
	}
	if required {
		return i.caller.ID, nil
	}
	return "", nil
}

// applyDependency links two imported tasks.
func (i *importer) applyDependency(ctx context.Context, tx store.Tx, m *mutation, d core.Dependency) error {
	taskID, ok := i.ids[d.TaskID]
	if !ok {
		return i.reject("dependency at line %d names task %q, which the snapshot does not contain",
			i.dec.Line(), d.TaskID)
	}
	dependsOn, ok := i.ids[d.DependsOn]
	if !ok {
		return i.reject("dependency at line %d depends on task %q, which the snapshot does not contain",
			i.dec.Line(), d.DependsOn)
	}
	if taskID == dependsOn {
		return i.reject("dependency at line %d makes a task depend on itself", i.dec.Line())
	}

	existing, err := tx.ListDependencies(ctx, taskID)
	if err != nil {
		return err
	}
	for _, edge := range existing {
		if edge.DependsOn == dependsOn {
			i.result.Skipped[string(core.RecordDependency)]++
			return nil
		}
	}
	i.count(core.RecordDependency, false)
	if i.dryRun {
		return nil
	}
	cycles, err := tx.DependencyPathExists(ctx, dependsOn, taskID)
	if err != nil {
		return err
	}
	if cycles {
		return i.reject("dependency at line %d would close a cycle", i.dec.Line())
	}
	if err := tx.AddDependency(ctx, &core.Dependency{TaskID: taskID, DependsOn: dependsOn}); err != nil {
		return err
	}
	return m.Record("dependency.add", core.EventDependencyAdded, "task", taskID, i.taskOwner[taskID],
		nil, map[string]any{"task_id": taskID, "depends_on": dependsOn}, nil)
}

// applyComment creates or updates one comment on an imported task.
func (i *importer) applyComment(ctx context.Context, tx store.Tx, m *mutation, c core.Comment) error {
	taskID, ok := i.ids[c.TaskID]
	if !ok {
		return i.reject("comment at line %d belongs to task %q, which the snapshot does not contain",
			i.dec.Line(), c.TaskID)
	}
	body, err := commentBody(c.Body)
	if err != nil {
		return i.reject("comment at line %d: %v", i.dec.Line(), err)
	}
	author, err := i.actorRef(ctx, tx, c.AuthorActorID, true)
	if err != nil {
		return err
	}

	existing, err := findComment(ctx, tx, c.ID, taskID)
	if err != nil {
		return err
	}
	target := &core.Comment{TaskID: taskID, AuthorActorID: author, Body: body, CreatedAt: c.CreatedAt}
	i.count(core.RecordComment, existing != nil)
	switch {
	case existing != nil:
		target.ID = existing.ID
		i.mapID(c.ID, existing.ID)
		if !i.dryRun {
			if err := tx.UpdateComment(ctx, target); err != nil {
				return err
			}
		}
	case i.dryRun:
		target.ID = i.plannedID(c.ID)
	default:
		if _, err := i.createWithID(core.RecordComment, c.ID, func(id string) error {
			target.ID = id
			return tx.CreateComment(ctx, target)
		}); err != nil {
			return err
		}
	}
	if i.dryRun {
		return nil
	}
	return m.Record("comment.add", core.EventCommentAdded, "comment", target.ID, i.taskOwner[taskID],
		existing, target, map[string]any{"task_id": taskID})
}

// findComment returns an existing comment of a task, or nil.
func findComment(ctx context.Context, tx store.Tx, id, taskID string) (*core.Comment, error) {
	if id == "" {
		return nil, nil
	}
	c, err := tx.GetComment(ctx, id)
	if core.IsKind(err, core.KindNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if c.TaskID != taskID {
		return nil, nil
	}
	return c, nil
}

// applyArtifact creates or updates one artifact on an imported task.
func (i *importer) applyArtifact(ctx context.Context, tx store.Tx, m *mutation, a core.Artifact) error {
	taskID, ok := i.ids[a.TaskID]
	if !ok {
		return i.reject("artifact at line %d belongs to task %q, which the snapshot does not contain",
			i.dec.Line(), a.TaskID)
	}
	if !artifactKindValid(a.Kind) {
		return i.reject("artifact at line %d has unknown kind %q", i.dec.Line(), a.Kind)
	}
	actor, err := i.actorRef(ctx, tx, a.ActorID, true)
	if err != nil {
		return err
	}

	update, err := artifactExists(ctx, tx, taskID, a.ID)
	if err != nil {
		return err
	}
	target := &core.Artifact{
		TaskID: taskID, ActorID: actor, Kind: a.Kind, Name: a.Name,
		Payload: a.Payload, ContentType: a.ContentType, Blob: a.Blob, CreatedAt: a.CreatedAt,
	}
	i.count(core.RecordArtifact, update)
	switch {
	case update:
		target.ID = a.ID
		i.mapID(a.ID, a.ID)
		if !i.dryRun {
			if err := tx.PutArtifact(ctx, target); err != nil {
				return err
			}
		}
	case i.dryRun:
		target.ID = i.plannedID(a.ID)
	default:
		if _, err := i.createWithID(core.RecordArtifact, a.ID, func(id string) error {
			target.ID = id
			return tx.PutArtifact(ctx, target)
		}); err != nil {
			return err
		}
	}
	if i.dryRun {
		return nil
	}
	return m.Record("artifact.put", core.EventArtifactAdded, "artifact", target.ID, i.taskOwner[taskID],
		nil, artifactSummary(target), map[string]any{"kind": string(target.Kind), "name": target.Name})
}

// artifactKindValid reports whether the kind is one tix stores.
func artifactKindValid(k core.ArtifactKind) bool {
	switch k {
	case core.ArtifactResult, core.ArtifactLog, core.ArtifactFile, core.ArtifactMetric:
		return true
	default:
		return false
	}
}

// artifactExists reports whether a task already carries this artifact.
func artifactExists(ctx context.Context, tx store.Tx, taskID, id string) (bool, error) {
	if id == "" {
		return false, nil
	}
	artifacts, err := tx.ListArtifacts(ctx, taskID)
	if err != nil {
		return false, err
	}
	for _, a := range artifacts {
		if a.ID == id {
			return true, nil
		}
	}
	return false, nil
}

// linkParents resolves the deferred subtask links once every task is mapped.
func (i *importer) linkParents(ctx context.Context, tx store.Tx) error {
	for _, link := range i.parents {
		parentID, ok := i.ids[link.parentID]
		if !ok {
			_ = i.reject("task at line %d has parent %q, which the snapshot does not contain",
				link.line, link.parentID)
			continue
		}
		if parentID == link.taskID {
			_ = i.reject("task at line %d is its own parent", link.line)
			continue
		}
		if i.dryRun {
			continue
		}
		task, err := tx.GetTask(ctx, core.TaskRef{ID: link.taskID})
		if err != nil {
			return err
		}
		if task.ParentID == parentID {
			continue
		}
		task.ParentID = parentID
		if err := tx.UpdateTask(ctx, task); err != nil {
			return err
		}
	}
	return nil
}

// reconcile removes, in replace mode, the rows of an imported project that the
// snapshot did not carry.
func (i *importer) reconcile(ctx context.Context, tx store.Tx, m *mutation) error {
	if i.mode != core.ImportReplace {
		return nil
	}
	for _, projectID := range i.sortedProjects() {
		if err := i.removeAbsentTasks(ctx, tx, m, projectID); err != nil {
			return err
		}
		if err := i.removeAbsentDefs(ctx, tx, m, projectID); err != nil {
			return err
		}
	}
	return nil
}

// sortedProjects returns the imported project identifiers in key order.
func (i *importer) sortedProjects() []string {
	out := make([]string, 0, len(i.projects))
	for id := range i.projects {
		out = append(out, id)
	}
	sort.Slice(out, func(a, b int) bool { return i.projects[out[a]].Key < i.projects[out[b]].Key })
	return out
}

// removeAbsentTasks deletes the project's tasks the snapshot did not carry.
func (i *importer) removeAbsentTasks(ctx context.Context, tx store.Tx, m *mutation, projectID string) error {
	var doomed []core.Task
	if err := eachTask(ctx, tx, projectID, false, func(t core.Task) error {
		if _, kept := i.taskOwner[t.ID]; !kept {
			doomed = append(doomed, t)
		}
		return nil
	}); err != nil {
		return err
	}
	for _, t := range doomed {
		i.result.Deleted[string(core.RecordTask)]++
		if i.dryRun {
			continue
		}
		if err := tx.DeleteTask(ctx, t.ID, true); err != nil {
			return err
		}
		if err := m.Record("task.delete", core.EventTaskDeleted, "task", t.ID, projectID,
			t, nil, map[string]any{"ref": t.Ref, "replaced": true}); err != nil {
			return err
		}
	}
	return nil
}

// removeAbsentDefs deletes the project's field definitions the snapshot did not carry.
func (i *importer) removeAbsentDefs(ctx context.Context, tx store.Tx, m *mutation, projectID string) error {
	kept := i.importedDefs[projectID]
	for _, d := range i.defs[projectID] {
		if kept[d.Key] {
			continue
		}
		i.result.Deleted[string(core.RecordFieldDef)]++
		if i.dryRun {
			continue
		}
		if err := tx.DeleteFieldDef(ctx, projectID, d.Key); err != nil {
			return err
		}
		if err := m.Record(auditFieldDelete, core.EventFieldUpdated, "field", d.ID, projectID,
			d, nil, map[string]any{"key": d.Key, "replaced": true}); err != nil {
			return err
		}
	}
	return nil
}
