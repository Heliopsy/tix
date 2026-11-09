// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// projectRoutes are the project list, board and field definition screens.
func (h *handler) projectRoutes() []route {
	return []route{
		get(RouteProjects, "projects.html", h.showProjects, "ListProjects", "ListStarredProjects", "ListWorkflows",
			"ListWorkflowTemplates"),
		post(RouteProjects, h.createProject, "CreateProject"),
		get(RouteProject, "tasks.html", h.showProjectTasks, "GetProject", "ListTasks", "ListProjects", "ListTags",
			"ListGroupPrefs", "ListSavedFilters", "ListTemplates", "TrashStatus"),
		get(RouteProjectSettings, "projectsettings.html", h.showProjectSettings,
			"GetProject", "ListWorkflows", "GetHousekeeping"),
		// The project page's live fragment renders through the same template
		// set, and the same block, the page does. See showProjectTasksLive.
		get(RouteBoardLive, "tasks.html", h.showProjectTasksLive, "GetProject", "ListTasks"),
		post(RouteProjectEdit, h.updateProject, "UpdateProject"),
		post(RouteProjectLease, h.updateProjectLease, "UpdateProject"),
		getFile(RouteProjectIcon, h.showProjectIcon, "GetProjectIcon"),
		post(RouteProjectArch, h.archiveProject, "ArchiveProject"),
		post(RouteProjectStar, h.starProject, "StarProject"),
		post(RouteProjectUnstar, h.unstarProject, "UnstarProject"),
		get(RouteProjectDel, "projectdelete.html", h.showProjectDelete,
			"GetProject", "ListProjects", "PlanProjectDeletion"),
		post(RouteProjectDel, h.deleteProject, "DeleteProject"),
		post(RouteBoardMove, h.moveCard, "TransitionRoute"),
		get(RouteFields, "fields.html", h.showFields, "ListFieldDefs"),
		post(RouteFields, h.putField, "PutFieldDef"),
		post(RouteFieldDelete, h.deleteField, "DeleteFieldDef"),
	}
}

// projectsView is what the project list renders. Editing a project, and
// archiving or deleting it, happen on the project's settings screen,
// which has carried a full settings disclosure and a danger zone since the
// board rework; the listing only ever needs to link to it, not to carry a
// second copy of that form. Workflows and Colors are still needed here for
// the "New project" form below the table.
type projectsView struct {
	Projects  []core.Project
	Workflows []core.Workflow
	// Templates are the built-in workflow templates the New project picker
	// offers beside the tenant's workflows.
	Templates []templateCard
	Colors    []core.ProjectColor
	Picker    iconPicker
	Pager     pager

	// Taken is every key in use, so the key the form suggests from a name is
	// one the service will accept.
	Taken []string
	// Starred are the reader's starred projects, drawn first, on the first
	// page or when StarredOnly asks for nothing else. Projects then leaves
	// them out, so a project is listed once.
	Starred     []core.Project
	StarredOnly bool
	// Form, Errors and Refused redraw a refused create in place, holding what
	// was typed.
	Form    projectForm
	Errors  fieldErrors
	Refused bool
}

// showProjects renders the project list and the creation form.
func (h *handler) showProjects(w http.ResponseWriter, r *http.Request) error {
	data, err := h.buildProjectsView(r)
	if err != nil {
		return err
	}
	return h.render(w, r, "projects.html", "Projects", data)
}

// buildProjectsView answers the project list, for the page and for a refused
// create drawn over it.
func (h *handler) buildProjectsView(r *http.Request) (projectsView, error) {
	size := h.rowsPerPage(r)
	read := func(cursor string, limit int) ([]core.Project, string, error) {
		return h.svc.ListProjects(r.Context(), core.ProjectFilter{
			IncludeArchived: true, Page: core.Page{Cursor: cursor, Limit: limit}})
	}
	pos, err := locate(r, size, walkOf(read))
	if err != nil {
		return projectsView{}, err
	}
	projects, next, err := read(pos.Cursor, size)
	if err != nil {
		return projectsView{}, err
	}
	workflows, err := h.svc.ListWorkflows(r.Context())
	if err != nil {
		return projectsView{}, err
	}
	templates, err := h.svc.ListWorkflowTemplates(r.Context())
	if err != nil {
		return projectsView{}, err
	}
	all, _, err := h.allProjects(r, true)
	if err != nil {
		return projectsView{}, err
	}
	taken := make([]string, 0, len(all))
	for _, p := range all {
		taken = append(taken, p.Key)
	}
	starredOnly := r.URL.Query().Get(StarredParam) == "1"
	var starred []core.Project
	if starredOnly || pos.Cursor == "" {
		if starred, err = h.svc.ListStarredProjects(r.Context()); err != nil {
			return projectsView{}, err
		}
	}
	rest := make([]core.Project, 0, len(projects))
	for _, p := range projects {
		if !p.Starred && !starredOnly {
			rest = append(rest, p)
		}
	}
	pg := newPager(r, RouteProjects, next, len(projects), "projects", SizeParam).counted(pos, size)
	if starredOnly {
		pg = pager{}
	}
	return projectsView{Projects: rest, Workflows: workflows,
		Templates: templateCards(langOf(r), templates, workflows),
		Colors:    core.ProjectColors(), Taken: taken,
		Starred: starred, StarredOnly: starredOnly,
		Picker: newIconPicker(core.Project{}),
		Pager:  pg}, nil
}

// StarredParam asks the projects screen for the reader's starred projects
// alone.
const StarredParam = "starred"

// starProject sets the reader's own star on a project.
func (h *handler) starProject(w http.ResponseWriter, r *http.Request) error {
	return h.toggleProjectStar(w, r, h.svc.StarProject, "starred ")
}

// unstarProject clears the reader's own star on a project.
func (h *handler) unstarProject(w http.ResponseWriter, r *http.Request) error {
	return h.toggleProjectStar(w, r, h.svc.UnstarProject, "unstarred ")
}

// toggleProjectStar applies one star call and sends the reader back to the
// screen the toggle was on, at the project's row. The page is boosted, so
// with scripting the answer swaps in place, sidebar included.
func (h *handler) toggleProjectStar(w http.ResponseWriter, r *http.Request, call func(context.Context, string) error, verb string) error {
	key := r.PathValue("key")
	if err := call(r.Context(), key); err != nil {
		return err
	}
	back := safeNext(field(r, "next"))
	if back == RouteProjects && field(r, "next") != RouteProjects {
		back = strings.Replace(RouteProject, "{key}", key, 1)
	}
	if (back == RouteProjects || strings.HasPrefix(back, RouteProjects+"?")) && !strings.Contains(back, "#") {
		back += "#p-" + url.PathEscape(strings.ToUpper(key))
	}
	redirectTo(w, r, back, "", verb+strings.ToUpper(key))
	return nil
}

// projectStar is what the "project-star" partial draws: one project's star
// toggle, the state it shows and where pressing it returns the reader to.
type projectStar struct {
	P       core.Project
	Key     string
	Name    string
	Starred bool
	Next    string
	CSRF    string
}

// Action is the route the toggle posts to: a starred project's unstars it.
func (s projectStar) Action() string {
	route := RouteProjectStar
	if s.Starred {
		route = RouteProjectUnstar
	}
	return strings.Replace(route, "{key}", s.Key, 1)
}

// newProjectStar builds a project's star toggle for a template.
func newProjectStar(p core.Project, next, csrf string) projectStar {
	return projectStar{P: p, Key: p.Key, Name: p.Name, Starred: p.Starred, Next: next, CSRF: csrf}
}

// projectScanPages bounds how many pages allProjects will walk. At
// core.MaxPageLimit a page it covers far more projects than a tenant has, and
// it is what stops a request walking an unbounded table; a caller that hits it
// is told the set is short rather than being handed a truncated one it
// believes is whole.
const projectScanPages = 40

// allProjects is every project in the tenant, walked to the end of the
// listing, and whether the walk finished.
//
// The task screen's visibility control, the accent map, the workflow lookup
// and the tenant diagram's count all need the whole set: each of them is
// wrong, silently, on a page of it. One page was what they had, because a
// ProjectFilter carrying no Limit is capped at core.DefaultPageLimit and its
// cursor was dropped on the floor. Raising the limit only moves where that
// starts to hurt, so the cursor is walked instead.
func (h *handler) allProjects(r *http.Request, includeArchived bool) ([]core.Project, bool, error) {
	filter := core.ProjectFilter{
		IncludeArchived: includeArchived,
		Page:            core.Page{Limit: core.MaxPageLimit},
	}
	var out []core.Project
	for range projectScanPages {
		found, next, err := h.svc.ListProjects(r.Context(), filter)
		if err != nil {
			return nil, false, err
		}
		out = append(out, found...)
		if next == "" {
			return out, true, nil
		}
		filter.Page.Cursor = next
	}
	return out, false, nil
}

// createProject creates a project from the list screen's form.
func (h *handler) createProject(w http.ResponseWriter, r *http.Request) error {
	form := readProjectForm(r)
	workflow, template := splitWorkflowChoice(form.Workflow)
	in := core.CreateProjectInput{
		Key:              form.Key,
		Name:             form.Name,
		Description:      form.Description,
		WorkflowKey:      workflow,
		WorkflowTemplate: template,
		Color:            form.Color,
	}
	choice, err := iconChoiceOf(r, false)
	if err != nil {
		return h.refuseProject(w, r, form, core.OnField(err, "icon"))
	}
	if choice.Icon != nil {
		in.Icon = *choice.Icon
	}
	if choice.IconName != nil {
		in.IconName = *choice.IconName
	}
	in.IconFile = choice.IconFile
	project, err := h.svc.CreateProject(r.Context(), in)
	if err != nil {
		return h.refuseProject(w, r, form, err)
	}
	redirect(w, r, RouteProjects+"/"+project.Key, "project created")
	return nil
}

// refuseProject answers a create the reader can correct by drawing the
// projects screen again with the form in place, holding what was typed, and
// the message beside the field it is about. An uploaded image cannot be
// handed back to a file input, so the picker reopens on the text or built-in
// choice the form carried.
func (h *handler) refuseProject(w http.ResponseWriter, r *http.Request, form projectForm, err error) error {
	if !refusable(err) {
		return err
	}
	clone := r.Clone(r.Context())
	clone.URL = &url.URL{Path: RouteProjects}
	data, viewErr := h.buildProjectsView(clone)
	if viewErr != nil {
		return viewErr
	}
	data.Form, data.Errors, data.Refused = form, refusal(err), true
	data.Picker = newIconPicker(core.Project{Icon: form.Icon, IconName: field(r, "icon_name")})
	return h.renderStatus(w, r, core.KindOf(err).HTTPStatus(), "projects.html", "Projects", data)
}

// updateProject changes a project's mutable fields.
// A changed key is asked about first; see confirmKeyChange.
func (h *handler) updateProject(w http.ResponseWriter, r *http.Request) error {
	key := r.PathValue("key")
	form := readProjectForm(r)
	project, err := h.svc.GetProject(r.Context(), key)
	if err != nil {
		return err
	}
	asked, err := h.confirmKeyChange(w, r, project, form.Key)
	switch {
	case asked:
		return err
	case err != nil:
		return h.refuseSettings(w, r, form, core.OnField(err, "key"))
	}
	in, err := iconChoiceOf(r, project.IconImage != "")
	if err != nil {
		return h.refuseSettings(w, r, form, core.OnField(err, "icon"))
	}
	in.Name, in.Description, in.Color = &form.Name, &form.Description, &form.Color
	if form.Key != "" {
		in.Key = &form.Key
	}
	if form.Workflow != "" {
		in.WorkflowKey = &form.Workflow
	}
	updated, err := h.svc.UpdateProject(r.Context(), project.ID, in)
	if err != nil {
		return h.refuseSettings(w, r, form, err)
	}
	redirect(w, r, settingsPath(updated.Key), "project updated")
	return nil
}

// updateProjectLease saves the board's Claims panel: the project's lease
// default and maximum, each blank to inherit the tenant's.
func (h *handler) updateProjectLease(w http.ResponseWriter, r *http.Request) error {
	var in core.UpdateProjectInput
	var err error
	if in.LeaseDefault, err = namedDuration(r, "lease_default"); err != nil {
		return err
	}
	if in.LeaseMax, err = namedDuration(r, "lease_max"); err != nil {
		return err
	}
	updated, err := h.svc.UpdateProject(r.Context(), r.PathValue("key"), in)
	if err != nil {
		return err
	}
	redirect(w, r, settingsPath(updated.Key), "claim length saved")
	return nil
}

// namedDuration reads a duration field only when the form named it, so a
// form that does not carry it leaves it alone. Blank is zero, which inherits.
func namedDuration(r *http.Request, name string) (*core.Duration, error) {
	if _, named := r.PostForm[name]; !named {
		return nil, nil
	}
	d, err := durationField(r, name)
	if err != nil {
		return nil, core.OnField(err, name)
	}
	return &d, nil
}

// refuseSettings answers a settings save the reader can correct by drawing
// the project's settings screen again, holding what was typed.
func (h *handler) refuseSettings(w http.ResponseWriter, r *http.Request, form projectForm, err error) error {
	if !refusable(err) {
		return err
	}
	data, viewErr := h.buildSettingsView(r)
	if viewErr != nil {
		return viewErr
	}
	if form.Key == "" {
		form.Key = data.Project.Key
	}
	data.Form, data.Errors, data.Refused = form, refusal(err), true
	if field(r, "icon_kind") == iconKindText {
		typed := data.Project
		typed.Icon, typed.IconName, typed.IconImage = form.Icon, "", ""
		data.Picker = newIconPicker(typed)
	}
	return h.renderStatus(w, r, core.KindOf(err).HTTPStatus(), "projectsettings.html", data.Project.Name, data)
}

// archiveProject archives a project.
func (h *handler) archiveProject(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.ArchiveProject(r.Context(), r.PathValue("key")); err != nil {
		return err
	}
	redirect(w, r, RouteProjects, "project archived")
	return nil
}

// card is one task on the board with the routes it may take, adjacent states
// and the ones reachable through them.
type card struct {
	Task   core.Task
	Routes []flowRoute
}

// column is one workflow state and the tasks sitting in it.
type column struct {
	State core.State
	Tasks []card
	// Head is the column's heading as the reader arranged it.
	Head groupHead
}

// projectSettingsView is what a project's settings screen renders.
type projectSettingsView struct {
	Project   core.Project
	Workflow  core.Workflow
	Workflows []core.Workflow
	Colors    []core.ProjectColor
	Picker    iconPicker

	// Form is what the settings form holds: the stored project, or what a
	// refused save was submitted with, in which case Errors sits beside the
	// fields.
	Form    projectForm
	Errors  fieldErrors
	Refused bool

	// KeyChange, when set, turns the screen into the confirmation of a new
	// key and nothing else.
	KeyChange *keyChange

	// Housekeeping is the project's cleanup overrides, nil for a reader who
	// may not change them.
	Housekeeping *housekeepingForm
}

// showProjectSettings renders a project's settings, its claims, its
// housekeeping and its danger zone, as the headed sections of one screen.
func (h *handler) showProjectSettings(w http.ResponseWriter, r *http.Request) error {
	data, err := h.buildSettingsView(r)
	if err != nil {
		return err
	}
	if redirectOldKey(w, r, r.PathValue("key"), &data.Project) {
		return nil
	}
	if data.Housekeeping, err = h.projectHousekeeping(r, data.Project.Key); err != nil {
		return err
	}
	return h.render(w, r, "projectsettings.html", data.Project.Name, data)
}

// buildSettingsView reads what a project's settings screen draws.
func (h *handler) buildSettingsView(r *http.Request) (projectSettingsView, error) {
	project, err := h.svc.GetProject(r.Context(), r.PathValue("key"))
	if err != nil {
		return projectSettingsView{}, err
	}
	workflows, err := h.svc.ListWorkflows(r.Context())
	if err != nil {
		return projectSettingsView{}, err
	}
	workflow, err := workflowFor(project, workflows)
	if err != nil {
		return projectSettingsView{}, err
	}
	return projectSettingsView{Project: *project, Workflow: *workflow, Workflows: workflows,
		Colors: core.ProjectColors(),
		Picker: newIconPicker(*project),
		Form:   formOf(*project, workflow.Key),
	}, nil
}

// settingsPath is a project's settings screen.
func settingsPath(key string) string { return RouteProjects + "/" + key + "/settings" }

// workflowOf resolves the workflow a project is assigned.
func (h *handler) workflowOf(r *http.Request, project *core.Project) (*core.Workflow, error) {
	workflows, err := h.svc.ListWorkflows(r.Context())
	if err != nil {
		return nil, err
	}
	return workflowFor(project, workflows)
}

// workflowFor picks the workflow a project is assigned out of a known list.
func workflowFor(project *core.Project, workflows []core.Workflow) (*core.Workflow, error) {
	for i := range workflows {
		if workflows[i].ID == project.WorkflowID {
			return &workflows[i], nil
		}
	}
	return nil, core.NotFound("workflow for project %q", project.Key)
}

// moveCard applies a board move, which is a route of one hop or of several.
func (h *handler) moveCard(w http.ResponseWriter, r *http.Request) error {
	ref, err := core.ParseTaskRef(field(r, "ref"))
	if err != nil {
		return err
	}
	steps, err := transitionSteps(r)
	if err != nil {
		return err
	}
	in := core.RouteInput{Route: steps, Comment: field(r, "comment")}
	result, err := h.svc.TransitionRoute(r.Context(), ref, in)
	if err != nil {
		return err
	}
	redirect(w, r, RouteProjects+"/"+r.PathValue("key"), "task "+ref.String()+" "+result.Sentence())
	return nil
}

// fieldsView is what the field definition editor renders.
type fieldsView struct {
	Project core.Project
	Fields  []core.FieldDef
	Types   []core.FieldType
}

// showFields renders a project's typed custom field definitions.
func (h *handler) showFields(w http.ResponseWriter, r *http.Request) error {
	key := r.PathValue("key")
	project, err := h.svc.GetProject(r.Context(), key)
	if err != nil {
		return err
	}
	if redirectOldKey(w, r, key, project) {
		return nil
	}
	defs, err := h.svc.ListFieldDefs(r.Context(), key)
	if err != nil {
		return err
	}
	return h.render(w, r, "fields.html", "Fields",
		fieldsView{Project: *project, Fields: defs, Types: core.FieldTypes})
}

// putField defines or redefines one custom field.
func (h *handler) putField(w http.ResponseWriter, r *http.Request) error {
	key := r.PathValue("key")
	position, err := intField(r, "position")
	if err != nil {
		return h.refuseIn(w, r, "field-new", RouteProjects+"/"+key+"/fields", core.OnField(err, "position"), h.showFields)
	}
	in := core.FieldDefInput{
		Key:         field(r, "key"),
		Label:       field(r, "label"),
		Type:        core.FieldType(field(r, "type")),
		Required:    checked(r, "required"),
		EnumOptions: fieldList(r, "enum_options"),
		Position:    position,
	}
	def := core.FieldDef{Key: in.Key, Label: in.Label, Type: in.Type, EnumOptions: in.EnumOptions}
	if def.Label == "" {
		def.Label = in.Key
	}
	if in.Default, err = coerceField(def, field(r, "default")); err != nil {
		return h.refuseIn(w, r, "field-new", RouteProjects+"/"+key+"/fields", core.OnField(err, "default"), h.showFields)
	}
	if _, err := h.svc.PutFieldDef(r.Context(), key, in); err != nil {
		return h.refuseIn(w, r, "field-new", RouteProjects+"/"+key+"/fields", err, h.showFields)
	}
	redirect(w, r, RouteProjects+"/"+key+"/fields", "field saved")
	return nil
}

// deleteField removes one custom field definition.
func (h *handler) deleteField(w http.ResponseWriter, r *http.Request) error {
	key := r.PathValue("key")
	if err := h.svc.DeleteFieldDef(r.Context(), key, field(r, "key")); err != nil {
		return err
	}
	redirect(w, r, RouteProjects+"/"+key+"/fields", "field deleted")
	return nil
}
