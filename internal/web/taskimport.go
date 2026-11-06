// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"bytes"
	"encoding/csv"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/quickadd"
	"github.com/heliopsy/tix/internal/taskio"
)

// The steps of an import, in order. Every step has its own address, so the
// browser's Back walks them, and every step has a Back that keeps what was
// chosen and a Cancel that leaves.
const (
	importStepUpload  = "upload"
	importStepColumns = "columns"
	importStepValues  = "values"
	importStepPreview = "preview"
	importStepResult  = "result"
)

// importSteps lists the steps with the words the stepper names them by.
var importSteps = []struct{ Key, Label string }{
	{importStepUpload, "Upload"},
	{importStepColumns, "Map columns"},
	{importStepValues, "Map values"},
	{importStepPreview, "Preview"},
	{importStepResult, "Result"},
}

// importTargetLabels name the fields a column can fill.
var importTargetLabels = map[taskio.Target]string{
	taskio.TargetIgnore: "Ignore", taskio.TargetAppendBody: "Append to description",
	taskio.TargetTitle: "Title", taskio.TargetBody: "Description", taskio.TargetStatus: "Status",
	taskio.TargetPriority: "Priority", taskio.TargetAssignee: "Assignee", taskio.TargetTags: "Tags",
	taskio.TargetDue: "Due", taskio.TargetParent: "Parent", taskio.TargetDependsOn: "Depends on",
	taskio.TargetRecurrence: "Repeat", taskio.TargetExternalID: "External ID",
	taskio.TargetAlias: "Other ID", taskio.TargetRef: "tix reference",
}

// importModeLabels name the modes.
var importModeLabels = map[core.TaskImportMode]string{
	core.TaskImportUpsert: "Update tasks already imported, create the rest",
	core.TaskImportCreate: "Create every row as a new task",
	core.TaskImportSkip:   "Leave tasks already imported alone, create the rest",
}

// importPolicyLabels name what happens to a value with no mapping.
var importPolicyLabels = map[taskio.UnknownPolicy]string{
	taskio.PolicyDefault: "Use the default", taskio.PolicyIgnore: "Leave it empty",
	taskio.PolicySkipRow: "Skip the row", taskio.PolicyError: "Stop with an error",
}

// importRoutes are the steps of a project's import.
func (h *handler) importRoutes() []route {
	return []route{
		get(RouteProjectImport, "import.html", h.showImportUpload, "GetProject"),
		post(RouteProjectImport, h.stageImport, "ImportTasks"),
		get(RouteProjectImportStep, "import.html", h.showImportStep, "ImportTasks"),
		post(RouteProjectImportStep, h.saveImportStep, "ImportTasks"),
	}
}

// importChoice is one option of a select.
type importChoice struct {
	Value    string
	Label    string
	Selected bool
}

// importColumnRow is one column of the file on the columns step.
type importColumnRow struct {
	Index   int
	Header  string
	Sample  []string
	Guess   bool
	Choices []importChoice
}

// importValueRow is one distinct value on the values step.
type importValueRow struct {
	Index   int
	Value   string
	Count   int
	Guess   bool
	Mapped  string
	Choices []importChoice
}

// importValueGroup is one field's values on the values step.
type importValueGroup struct {
	Target   string
	Label    string
	Rows     []importValueRow
	Policies []importChoice
	Free     bool
}

// importStepLink is one step of the stepper.
type importStepLink struct {
	Key     string
	Label   string
	Number  int
	Current bool
	Done    bool
}

// importView is what every step of an import draws.
type importView struct {
	Project core.Project
	Step    string
	Steps   []importStepLink
	Stage   *importStage
	Base    string
	Back    string
	Cancel  string
	Error   string

	Result     *core.TaskImportResult
	Columns    []importColumnRow
	Groups     []importValueGroup
	Modes      []importChoice
	Statuses   []importChoice
	Prios      []importChoice
	Source     string
	DateFmt    string
	Delimiter  string
	Delimiters []string
	CanImport  bool
}

// stepHref is the address of one step of a staged import.
func stepHref(key, id, step string) string {
	if step == importStepUpload {
		return RouteProjects + "/" + key + "/import?id=" + id
	}
	return RouteProjects + "/" + key + "/import/" + id + "/" + step
}

// previousStep is the step Back leads to.
func previousStep(step string, lines bool) string {
	switch step {
	case importStepColumns:
		return importStepUpload
	case importStepValues:
		return importStepColumns
	case importStepPreview:
		if lines {
			return importStepUpload
		}
		return importStepValues
	case importStepResult:
		return importStepPreview
	}
	return importStepUpload
}

// newImportView builds the frame every step shares.
func newImportView(project core.Project, step string, st *importStage) importView {
	v := importView{Project: project, Step: step, Stage: st, Cancel: RouteProjects + "/" + project.Key}
	current := 0
	for k, s := range importSteps {
		if s.Key == step {
			current = k
		}
	}
	for k, s := range importSteps {
		v.Steps = append(v.Steps, importStepLink{Key: s.Key, Label: s.Label, Number: k + 1,
			Current: k == current, Done: k < current})
	}
	if st == nil {
		v.Back = v.Cancel
		return v
	}
	v.Base = RouteProjects + "/" + project.Key + "/import/" + st.ID
	v.Back = stepHref(project.Key, st.ID, previousStep(step, st.Format == core.TaskFormatLines))
	return v
}

// showImportUpload draws the first step: a file, or a pasted list. Coming back
// to it with an upload already staged keeps that upload and the list.
func (h *handler) showImportUpload(w http.ResponseWriter, r *http.Request) error {
	project, err := h.svc.GetProject(r.Context(), r.PathValue("key"))
	if err != nil {
		return err
	}
	if redirectOldKey(w, r, r.PathValue("key"), project) {
		return nil
	}
	var st *importStage
	if id := r.URL.Query().Get("id"); id != "" {
		if actor, ok := core.ActorFrom(r.Context()); ok {
			st, _ = h.imports.get(actor, project.Key, id)
		}
	}
	v := newImportView(*project, importStepUpload, st)
	v.Back = v.Cancel
	return h.render(w, r, "import.html", project.Name, v)
}

// stageImport takes the upload, or the pasted list, and moves to the next
// step: the columns, or straight to the preview when asked or when there is
// nothing to map.
func (h *handler) stageImport(w http.ResponseWriter, r *http.Request) error {
	project, err := h.svc.GetProject(r.Context(), r.PathValue("key"))
	if err != nil {
		return err
	}
	actor, err := core.RequireActor(r.Context())
	if err != nil {
		return err
	}
	if field(r, "go") == "cancel" {
		h.imports.drop(field(r, "id"))
		redirect(w, r, RouteProjects+"/"+project.Key, "")
		return nil
	}
	lines := strings.TrimSpace(r.PostFormValue("lines"))
	filename, body, err := uploadedImport(r)
	if err != nil {
		return h.refuseImportUpload(w, r, project, err)
	}
	var st *importStage
	switch {
	case body != nil:
		st, err = h.imports.put(actor, project.Key, filename, bytes.NewReader(body))
	case lines != "":
		st, err = h.imports.put(actor, project.Key, "", strings.NewReader(lines))
		if st != nil {
			st.Format, st.Lines = core.TaskFormatLines, lines
		}
	case field(r, "id") != "":
		st, err = h.imports.get(actor, project.Key, field(r, "id"))
	default:
		err = core.Invalid("choose a file, or paste a list of tasks").WithDetail(core.DetailField, "file")
	}
	if err != nil {
		return h.refuseImportUpload(w, r, project, err)
	}
	if mapping := uploadedMapping(r); mapping != "" {
		if _, err := taskio.ParseMapping([]byte(mapping)); err != nil {
			return h.refuseImportUpload(w, r, project, core.OnField(err, "mapping_file"))
		}
		st.Mapping = mapping
	}
	next := importStepColumns
	if st.Format == core.TaskFormatLines || field(r, "go") == "preview" {
		next = importStepPreview
	}
	redirect(w, r, stepHref(project.Key, st.ID, next), "")
	return nil
}

// refuseImportUpload draws the upload step again with the reason.
func (h *handler) refuseImportUpload(w http.ResponseWriter, r *http.Request, project *core.Project, err error) error {
	if !refusable(err) {
		return err
	}
	v := newImportView(*project, importStepUpload, nil)
	v.Error = errText(err)
	w.WriteHeader(core.KindOf(err).HTTPStatus())
	return h.render(w, r, "import.html", project.Name, v)
}

// errText is a refusal's message without its kind.
func errText(err error) string {
	var ce *core.Error
	if errors.As(err, &ce) {
		return ce.Message
	}
	return err.Error()
}

// uploadedImport is the file part of the upload step, if one was chosen.
func uploadedImport(r *http.Request) (string, []byte, error) {
	if r.MultipartForm == nil {
		return "", nil, nil
	}
	files := r.MultipartForm.File["file"]
	if len(files) == 0 || files[0].Size == 0 {
		return "", nil, nil
	}
	f, err := files[0].Open()
	if err != nil {
		return "", nil, core.Invalid("the uploaded file could not be read").WithDetail(core.DetailField, "file")
	}
	defer func() { _ = f.Close() }()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(f); err != nil {
		return "", nil, core.Invalid("the uploaded file could not be read").WithDetail(core.DetailField, "file")
	}
	return files[0].Filename, buf.Bytes(), nil
}

// uploadedMapping is a mapping file sent with the upload, if any.
func uploadedMapping(r *http.Request) string {
	if r.MultipartForm == nil {
		return ""
	}
	files := r.MultipartForm.File["mapping_file"]
	if len(files) == 0 || files[0].Size == 0 {
		return ""
	}
	f, err := files[0].Open()
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(f)
	return buf.String()
}

// stagedImport resolves the project and the reader's own stage a step names.
func (h *handler) stagedImport(r *http.Request) (*core.Project, *importStage, error) {
	project, err := h.svc.GetProject(r.Context(), r.PathValue("key"))
	if err != nil {
		return nil, nil, err
	}
	actor, err := core.RequireActor(r.Context())
	if err != nil {
		return nil, nil, err
	}
	st, err := h.imports.get(actor, project.Key, r.PathValue("id"))
	if err != nil {
		return nil, nil, err
	}
	return project, st, nil
}

// dryRun runs the stage's file through the import without writing, which is
// how every step learns the columns, the values and the preview.
func (h *handler) dryRun(r *http.Request, project *core.Project, st *importStage, commit bool) (*core.TaskImportResult, error) {
	f, err := h.imports.open(st)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return h.svc.ImportTasks(r.Context(), f, core.TaskImportInput{
		ProjectRef: project.Key, Format: st.Format, Filename: st.Filename,
		Mapping: st.Mapping, Mode: st.Mode, DryRun: !commit, Preview: core.MaxImportPreview,
	})
}

// showImportStep draws one step after the upload, or a file a step offers.
func (h *handler) showImportStep(w http.ResponseWriter, r *http.Request) error {
	project, st, err := h.stagedImport(r)
	if err != nil {
		return err
	}
	step := r.PathValue("step")
	switch step {
	case "errors.csv":
		return writeImportErrors(w, st)
	case "mapping.yaml":
		return h.writeImportMapping(w, r, project, st)
	case importStepColumns, importStepValues:
		if st.Format == core.TaskFormatLines {
			redirect(w, r, stepHref(project.Key, st.ID, importStepPreview), "")
			return nil
		}
	case importStepPreview:
	case importStepResult:
		if st.Result == nil {
			redirect(w, r, stepHref(project.Key, st.ID, importStepPreview), "")
			return nil
		}
		v := newImportView(*project, step, st)
		v.Result = st.Result
		return h.render(w, r, "import.html", project.Name, v)
	default:
		return core.NotFound("no import step %q", step)
	}
	result, err := h.dryRun(r, project, st, false)
	if err != nil {
		v := newImportView(*project, step, st)
		v.Error = errText(err)
		if !refusable(err) {
			return err
		}
		w.WriteHeader(core.KindOf(err).HTTPStatus())
		return h.render(w, r, "import.html", project.Name, v)
	}
	if st.Result == nil || !st.Result.Committed {
		st.Result = result
	}
	v, err := h.importStepView(r, project, st, step, result)
	if err != nil {
		return err
	}
	return h.render(w, r, "import.html", project.Name, v)
}

// importStepView fills a step's choices from a dry run.
func (h *handler) importStepView(r *http.Request, project *core.Project, st *importStage, step string,
	result *core.TaskImportResult) (importView, error) {
	v := newImportView(*project, step, st)
	v.Result = result
	mapping := stageMapping(st)
	v.Source, v.DateFmt, v.Delimiter = mapping.Source, mapping.DateFormat, mapping.Delimiter
	v.Delimiters = []string{",", ";", "tab", "|"}
	v.CanImport = result.Failed == 0 && result.Rows > result.Skipped
	defs, err := h.svc.ListFieldDefs(r.Context(), project.Key)
	if err != nil {
		return v, err
	}
	workflow, err := h.workflowOf(r, project)
	if err != nil {
		return v, err
	}
	targets := importTargets(defs)
	for k, c := range result.Columns {
		row := importColumnRow{Index: k, Header: c.Header, Sample: c.Sample, Guess: c.Suggested}
		for _, t := range targets {
			row.Choices = append(row.Choices, importChoice{Value: t.Value, Label: t.Label, Selected: t.Value == c.Target})
		}
		v.Columns = append(v.Columns, row)
	}
	for _, m := range core.TaskImportModes {
		v.Modes = append(v.Modes, importChoice{Value: string(m), Label: importModeLabels[m], Selected: m == result.Mode})
	}
	v.Statuses = append(v.Statuses, importChoice{Value: "", Label: "No mapping"})
	for _, s := range workflow.Definition.States {
		v.Statuses = append(v.Statuses, importChoice{Value: s.Key, Label: s.Label})
	}
	v.Prios = append(v.Prios, importChoice{Value: "", Label: "No mapping"})
	for p := core.PriorityHighest; p <= core.PriorityLowest; p++ {
		name := quickadd.PriorityName(p)
		v.Prios = append(v.Prios, importChoice{Value: name, Label: name})
	}
	v.Groups = []importValueGroup{
		valueGroup(taskio.TargetStatus, "Status", result.Values["status"], v.Statuses, mapping, false),
		valueGroup(taskio.TargetPriority, "Priority", result.Values["priority"], v.Prios, mapping, false),
		valueGroup(taskio.TargetAssignee, "Assignee", result.Values["assignee"], nil, mapping, true),
	}
	return v, nil
}

// valueGroup builds one field's rows of the values step.
func valueGroup(t taskio.Target, label string, values []core.TaskImportValue, choices []importChoice,
	mapping *taskio.Mapping, free bool) importValueGroup {
	g := importValueGroup{Target: string(t), Label: label, Free: free}
	for k, val := range values {
		row := importValueRow{Index: k, Value: val.Value, Count: val.Count, Guess: val.Suggested, Mapped: val.Mapped}
		for _, c := range choices {
			c.Selected = c.Value != "" && c.Value == val.Mapped
			row.Choices = append(row.Choices, c)
		}
		g.Rows = append(g.Rows, row)
	}
	current := mapping.Policy(t)
	for _, p := range []taskio.UnknownPolicy{taskio.PolicyDefault, taskio.PolicyIgnore, taskio.PolicySkipRow, taskio.PolicyError} {
		if (t == taskio.TargetAssignee) == (p == taskio.PolicyDefault) {
			continue
		}
		g.Policies = append(g.Policies, importChoice{Value: string(p), Label: importPolicyLabels[p], Selected: p == current})
	}
	return g
}

// importTargets is every field a column can fill, custom fields last.
func importTargets(defs []core.FieldDef) []importChoice {
	var out []importChoice
	for _, t := range taskio.Targets {
		out = append(out, importChoice{Value: string(t), Label: importTargetLabels[t]})
	}
	for _, d := range defs {
		out = append(out, importChoice{Value: string(taskio.CustomTarget(d.Key)), Label: d.Label})
	}
	return out
}

// stageMapping is the stage's mapping, or an empty one.
func stageMapping(st *importStage) *taskio.Mapping {
	if st.Mapping != "" {
		if m, err := taskio.ParseMapping([]byte(st.Mapping)); err == nil {
			return m
		}
	}
	return &taskio.Mapping{Version: taskio.MappingVersion}
}

// saveImportStep keeps what a step chose and moves where its button says:
// back, on, straight to the preview, or, from the preview, the import itself.
func (h *handler) saveImportStep(w http.ResponseWriter, r *http.Request) error {
	project, st, err := h.stagedImport(r)
	if err != nil {
		return err
	}
	step := r.PathValue("step")
	switch step {
	case importStepColumns:
		saveImportColumns(r, st)
	case importStepValues:
		saveImportValues(r, st)
	}
	lines := st.Format == core.TaskFormatLines
	switch field(r, "go") {
	case "back":
		redirect(w, r, stepHref(project.Key, st.ID, previousStep(step, lines)), "")
	case "preview":
		redirect(w, r, stepHref(project.Key, st.ID, importStepPreview), "")
	case "import":
		result, err := h.dryRun(r, project, st, true)
		if err != nil {
			return err
		}
		st.Result = result
		redirect(w, r, stepHref(project.Key, st.ID, importStepResult), "")
	default:
		next := importStepValues
		if step == importStepValues {
			next = importStepPreview
		}
		redirect(w, r, stepHref(project.Key, st.ID, next), "")
	}
	return nil
}

// saveImportColumns writes the columns step into the stage's mapping. Every
// column is written down, so what the reader saw chosen is what runs, even
// where it was tix's own suggestion.
func saveImportColumns(r *http.Request, st *importStage) {
	m := stageMapping(st)
	if st.Result != nil {
		m.Columns = map[string]taskio.Target{}
		for k, c := range st.Result.Columns {
			target := taskio.Target(r.PostFormValue("col_" + strconv.Itoa(k)))
			if target == "" || !target.Valid() {
				target = taskio.Target(c.Target)
			}
			m.Columns[c.Header] = target
		}
	}
	if mode := core.TaskImportMode(field(r, "mode")); mode.Valid() {
		st.Mode = mode
		m.Mode = taskio.Mode(mode)
	}
	m.Source = field(r, "source")
	m.DateFormat = field(r, "date_format")
	if d := field(r, "delimiter"); d == "" || slices.Contains([]string{",", ";", "tab", "|"}, d) {
		m.Delimiter = d
	}
	if raw, err := m.Marshal(); err == nil {
		if _, err := taskio.ParseMapping(raw); err == nil {
			st.Mapping = string(raw)
		}
	}
}

// saveImportValues writes the values step into the stage's mapping.
func saveImportValues(r *http.Request, st *importStage) {
	m := stageMapping(st)
	if m.Values == nil {
		m.Values = map[taskio.Target]map[string]string{}
	}
	if m.Unknown == nil {
		m.Unknown = map[taskio.Target]taskio.UnknownPolicy{}
	}
	for _, t := range []taskio.Target{taskio.TargetStatus, taskio.TargetPriority, taskio.TargetAssignee} {
		values := map[string]string{}
		for k := 0; ; k++ {
			key := string(t) + "_" + strconv.Itoa(k)
			raw, ok := r.PostForm["raw_"+key]
			if !ok || len(raw) == 0 {
				break
			}
			if v := strings.TrimSpace(r.PostFormValue("val_" + key)); v != "" {
				values[raw[0]] = v
			}
		}
		if len(values) > 0 {
			m.Values[t] = values
		} else {
			delete(m.Values, t)
		}
		if p := taskio.UnknownPolicy(field(r, "unknown_"+string(t))); p != "" {
			m.Unknown[t] = p
		}
	}
	if raw, err := m.Marshal(); err == nil {
		if _, err := taskio.ParseMapping(raw); err == nil {
			st.Mapping = string(raw)
		}
	}
}

// writeImportErrors answers the stage's refusals as a CSV error report.
func writeImportErrors(w http.ResponseWriter, st *importStage) error {
	var buf bytes.Buffer
	out := csv.NewWriter(&buf)
	_ = out.Write([]string{"line", "error"})
	if st.Result != nil {
		for _, e := range st.Result.Errors {
			_ = out.Write([]string{strconv.Itoa(e.Line), output.GuardCell(e.Message)})
		}
	}
	out.Flush()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="import-errors.csv"`)
	_, err := w.Write(buf.Bytes())
	return err
}

// writeImportMapping answers the mapping in force as a file to keep.
func (h *handler) writeImportMapping(w http.ResponseWriter, r *http.Request, project *core.Project, st *importStage) error {
	mapping := st.Mapping
	if st.Result != nil && st.Result.Mapping != "" {
		mapping = st.Result.Mapping
	} else if result, err := h.dryRun(r, project, st, false); err == nil {
		mapping = result.Mapping
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+strings.ToLower(project.Key)+`-mapping.yaml"`)
	_, err := w.Write([]byte(mapping))
	return err
}

// importCount is how many tasks an import writes.
func importCount(r *core.TaskImportResult) int { return r.Created + r.Updated }
