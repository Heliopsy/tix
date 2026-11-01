// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// templateScanPages bounds how many pages of templates a screen walks.
const templateScanPages = 20

// templateRoutes are the screens and forms of task templates.
func (h *handler) templateRoutes() []route {
	return []route{
		get(RouteTemplates, "templates.html", h.showTemplates, "ListTemplates"),
		get(RouteTemplateNew, "templatenew.html", h.showUseTemplate, "ListTemplates", "ListProjects"),
		post(RouteTemplateNew, h.useTemplate, "UseTemplate"),
		get(RouteTemplate, "template.html", h.showTemplate, "GetTemplate", "ListProjects"),
		post(RouteTemplateEdit, h.updateTemplate, "UpdateTemplate"),
		post(RouteTemplateDel, h.deleteTemplate, "DeleteTemplate"),
		get(RouteTaskTemplate, "tasktemplate.html", h.showSaveTemplate, "GetTask", "TaskTree"),
		post(RouteTaskTemplate, h.saveTemplate, "SaveTemplate"),
	}
}

// allTemplates walks every page of the templates the reader can read,
// narrowed to one project and the tenant-wide ones when project is set.
func (h *handler) allTemplates(r *http.Request, project string) ([]core.TaskTemplate, error) {
	f := core.TemplateFilter{ProjectRef: project, Page: core.Page{Limit: core.MaxPageLimit}}
	var out []core.TaskTemplate
	for range templateScanPages {
		found, next, err := h.svc.ListTemplates(r.Context(), f)
		if err != nil {
			return nil, err
		}
		out = append(out, found...)
		if next == "" {
			break
		}
		f.Page.Cursor = next
	}
	return out, nil
}

// templatesView is what the Templates screen draws.
type templatesView struct {
	Templates []core.TaskTemplate
}

// showTemplates lists the templates the reader can read.
func (h *handler) showTemplates(w http.ResponseWriter, r *http.Request) error {
	items, err := h.allTemplates(r, "")
	if err != nil {
		return err
	}
	return h.render(w, r, "templates.html", "Templates", templatesView{Templates: items})
}

// templateForm is what the template edit form was submitted with.
type templateForm struct {
	Name, Project, DueIn, Title, Body, Priority, Tags, Repeat, Fields string
}

// templateFormOf is the form a template is first drawn with.
func templateFormOf(t core.TaskTemplate) templateForm {
	f := templateForm{
		Name: t.Name, Project: t.ProjectKey, Title: t.Task.Title, Body: t.Task.Body,
		Tags: strings.Join(t.Task.Tags, ", "), Repeat: t.Task.Recurrence, Fields: fieldLines(t.Task.CustomFields),
	}
	if t.DueIn > 0 {
		f.DueIn = t.DueIn.Human()
	}
	if t.Task.Priority.Valid() {
		f.Priority = strconv.Itoa(int(t.Task.Priority))
	}
	return f
}

// fieldLines writes custom fields one key=value per line, sorted by key.
func fieldLines(values map[string]any) string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		v := values[k]
		if s, ok := v.(string); ok {
			lines = append(lines, k+"="+s)
			continue
		}
		raw, err := json.Marshal(v)
		if err != nil {
			continue
		}
		lines = append(lines, k+"="+string(raw))
	}
	return strings.Join(lines, "\n")
}

// parseFieldLines reads key=value lines, a value that parses as JSON taken as
// that value, and sets every key the template had and the lines dropped to
// nil so the update removes it.
func parseFieldLines(text string, had map[string]any) (map[string]any, error) {
	out := map[string]any{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, raw, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, core.Invalid("field line %q must be key=value", line).WithDetail(core.DetailField, "custom_fields")
		}
		raw = strings.TrimSpace(raw)
		var decoded any
		if err := json.Unmarshal([]byte(raw), &decoded); err == nil && decoded != nil {
			out[key] = decoded
			continue
		}
		out[key] = raw
	}
	for k := range had {
		if _, kept := out[k]; !kept {
			out[k] = nil
		}
	}
	return out, nil
}

// templateView is what the template screen draws: the template, the projects
// it may belong to and the edit form.
type templateView struct {
	Template   core.TaskTemplate
	Projects   []core.Project
	Priorities []priorityChoice
	Form       templateForm
	Errors     fieldErrors
}

// showTemplate draws one template with its edit form.
func (h *handler) showTemplate(w http.ResponseWriter, r *http.Request) error {
	data, err := h.buildTemplateView(r)
	if err != nil {
		return err
	}
	data.Form = templateFormOf(data.Template)
	return h.render(w, r, "template.html", data.Template.Name, data)
}

func (h *handler) buildTemplateView(r *http.Request) (templateView, error) {
	tpl, err := h.svc.GetTemplate(r.Context(), r.PathValue("id"))
	if err != nil {
		return templateView{}, err
	}
	projects, _, err := h.allProjects(r, false)
	if err != nil {
		return templateView{}, err
	}
	return templateView{Template: *tpl, Projects: copyTargets(r, projects), Priorities: priorityChoices}, nil
}

// readTemplateForm reads a submitted template edit form.
func readTemplateForm(r *http.Request) templateForm {
	return templateForm{
		Name: field(r, "name"), Project: field(r, "project_ref"), DueIn: field(r, "due_in"),
		Title: field(r, "title"), Body: prose(r, "body"), Priority: field(r, "priority"),
		Tags: field(r, "tags"), Repeat: field(r, "repeat"), Fields: prose(r, "custom_fields"),
	}
}

// input is the update the form asks for, every field sent as the form holds
// it, so the service writes nothing when nothing changed.
func (f templateForm) input(r *http.Request, had map[string]any) (core.UpdateTemplateInput, error) {
	due, err := dueInField(f.DueIn)
	if err != nil {
		return core.UpdateTemplateInput{}, err
	}
	priority, err := priorityField(r, "priority")
	if err != nil {
		return core.UpdateTemplateInput{}, core.OnField(err, "priority")
	}
	fields, err := parseFieldLines(f.Fields, had)
	if err != nil {
		return core.UpdateTemplateInput{}, err
	}
	tags := splitList(f.Tags)
	in := core.UpdateTemplateInput{
		Name: &f.Name, ProjectRef: &f.Project, DueIn: &due, Title: &f.Title, Body: &f.Body,
		Tags: &tags, Recurrence: &f.Repeat, CustomFields: fields,
	}
	if priority != 0 {
		in.Priority = &priority
	}
	return in, nil
}

// dueInField reads a due offset such as 3d; empty is none.
func dueInField(raw string) (core.Duration, error) {
	if raw == "" {
		return 0, nil
	}
	d, err := core.ParseDuration(raw)
	if err != nil {
		return 0, core.OnField(err, "due_in")
	}
	return d, nil
}

// updateTemplate saves the edit form, drawing it again on a refusal.
func (h *handler) updateTemplate(w http.ResponseWriter, r *http.Request) error {
	form := readTemplateForm(r)
	data, err := h.buildTemplateView(r)
	if err != nil {
		return err
	}
	in, err := form.input(r, data.Template.Task.CustomFields)
	if err == nil {
		_, err = h.svc.UpdateTemplate(r.Context(), data.Template.ID, in)
	}
	if err != nil {
		if !refusable(err) {
			return err
		}
		data.Form, data.Errors = form, refusal(err)
		return h.renderStatus(w, r, core.KindOf(err).HTTPStatus(), "template.html", data.Template.Name, data)
	}
	redirect(w, r, RouteTemplates, "template saved")
	return nil
}

// deleteTemplate removes a template and returns to the list.
func (h *handler) deleteTemplate(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.DeleteTemplate(r.Context(), r.PathValue("id")); err != nil {
		return err
	}
	redirect(w, r, RouteTemplates, "template deleted")
	return nil
}

// saveView is what the Save as template form draws.
type saveView struct {
	Task        core.Task
	HasSubtasks bool
	Form        saveForm
	Errors      fieldErrors
}

// saveForm is what Save as template was submitted with.
type saveForm struct {
	Name, DueIn          string
	TenantWide, Subtasks bool
}

// showSaveTemplate draws the form that saves a task as a template.
func (h *handler) showSaveTemplate(w http.ResponseWriter, r *http.Request) error {
	data, err := h.buildSaveView(r)
	if err != nil {
		return err
	}
	data.Form = saveForm{Name: data.Task.Title, Subtasks: data.HasSubtasks}
	return h.render(w, r, "tasktemplate.html", output.ShowRef(data.Task.Ref), data)
}

func (h *handler) buildSaveView(r *http.Request) (saveView, error) {
	ref, err := taskRefOf(r)
	if err != nil {
		return saveView{}, err
	}
	tree, err := h.svc.TaskTree(r.Context(), ref, 1)
	if err != nil {
		return saveView{}, err
	}
	if len(tree) == 0 {
		return saveView{}, core.NotFound("task %q", ref.String())
	}
	return saveView{Task: tree[0], HasSubtasks: len(tree) > 1}, nil
}

// saveTemplate saves the task as a template and returns to the task.
func (h *handler) saveTemplate(w http.ResponseWriter, r *http.Request) error {
	form := saveForm{
		Name: field(r, "name"), DueIn: field(r, "due_in"),
		TenantWide: field(r, "scope") == "tenant", Subtasks: checked(r, "with_subtasks"),
	}
	due, err := dueInField(form.DueIn)
	if err == nil {
		_, err = h.svc.SaveTemplate(r.Context(), core.SaveTemplateInput{
			Name: form.Name, TaskRef: r.PathValue("ref"), TenantWide: form.TenantWide,
			WithSubtasks: form.Subtasks, DueIn: due,
		})
	}
	if err != nil {
		if !refusable(err) {
			return err
		}
		data, viewErr := h.buildSaveView(r)
		if viewErr != nil {
			return viewErr
		}
		data.Form, data.Errors = form, refusal(err)
		status := core.KindOf(err).HTTPStatus()
		if dialogRequest(r) {
			status = http.StatusOK
		}
		return h.renderStatus(w, r, status, "tasktemplate.html", data.Task.Ref, data)
	}
	redirect(w, r, RouteTasks+"/"+url.PathEscape(r.PathValue("ref")), "saved as a template")
	return nil
}

// useView is what the New from template form draws.
type useView struct {
	Templates []core.TaskTemplate
	Projects  []core.Project
	// Chosen is the template the form was opened for or submitted with.
	Chosen *core.TaskTemplate
	Form   useForm
	Errors fieldErrors
}

// useForm is what New from template was submitted with.
type useForm struct {
	Template, Title, Project, DueIn, Next string
}

// showUseTemplate draws the form that creates a task from a template, with
// the template and project the address names chosen.
func (h *handler) showUseTemplate(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	form := useForm{Template: q.Get("template"), Project: q.Get("project"), Next: copyReturn(q.Get("next"))}
	data, err := h.buildUseView(r, form)
	if err != nil {
		return err
	}
	return h.render(w, r, "templatenew.html", "New from template", data)
}

func (h *handler) buildUseView(r *http.Request, form useForm) (useView, error) {
	items, err := h.allTemplates(r, "")
	if err != nil {
		return useView{}, err
	}
	projects, _, err := h.allProjects(r, false)
	if err != nil {
		return useView{}, err
	}
	data := useView{Templates: items, Projects: copyTargets(r, projects), Form: form}
	for i := range items {
		if items[i].ID == form.Template || strings.EqualFold(items[i].Name, form.Template) {
			data.Chosen = &items[i]
			data.Form.Template = items[i].ID
		}
	}
	if data.Chosen == nil && len(items) > 0 && form.Template == "" {
		data.Chosen = &items[0]
		data.Form.Template = items[0].ID
	}
	return data, nil
}

// useTemplate creates the task, then opens it or returns to the listing the
// form was opened from with a toast linking it, as adding a task does.
func (h *handler) useTemplate(w http.ResponseWriter, r *http.Request) error {
	form := useForm{
		Template: field(r, "template"), Title: field(r, "title"), Project: field(r, "project_ref"),
		DueIn: field(r, "due_in"), Next: copyReturn(field(r, "next")),
	}
	in := core.UseTemplateInput{ProjectRef: form.Project, Title: form.Title}
	var err error
	if form.DueIn != "" {
		var due core.Duration
		if due, err = dueInField(form.DueIn); err == nil {
			in.DueIn = &due
		}
	}
	var task *core.Task
	if err == nil {
		if form.Template == "" {
			err = core.Invalid("choose a template").WithDetail(core.DetailField, "template")
		} else {
			task, err = h.svc.UseTemplate(r.Context(), form.Template, in)
		}
	}
	if err != nil {
		if !refusable(err) && !core.IsKind(err, core.KindNotFound) {
			return err
		}
		data, viewErr := h.buildUseView(r, form)
		if viewErr != nil {
			return viewErr
		}
		data.Errors = refusal(err)
		if core.IsKind(err, core.KindNotFound) && core.FieldOf(err) == "" {
			data.Errors = refusal(core.OnField(err, "template"))
		}
		return h.renderStatus(w, r, core.KindOf(err).HTTPStatus(), "templatenew.html", "New from template", data)
	}
	if openAfterAdd(r) || form.Next == "" {
		redirect(w, r, RouteTasks+"/"+task.Ref, "task created from a template")
		return nil
	}
	redirect(w, r, flashRef(form.Next, task.Ref), "task created from a template")
	return nil
}

// addTemplates is the templates the add box offers on the task screen; a
// reader who cannot list them is offered none rather than refused the screen.
func (h *handler) addTemplates(r *http.Request) []core.TaskTemplate {
	items, _, err := h.svc.ListTemplates(r.Context(), core.TemplateFilter{Page: core.Page{Limit: maxAddTemplates}})
	if err != nil {
		return nil
	}
	return items
}

// maxAddTemplates is how many templates the add box menu lists before it
// points at the Templates screen for the rest.
const maxAddTemplates = 12
