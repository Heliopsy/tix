// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
)

// projectRoutes are the project list, board and field definition screens.
func (h *handler) projectRoutes() []route {
	return []route{
		get(RouteProjects, "projects.html", h.showProjects, "ListProjects", "ListWorkflows"),
		post(RouteProjects, h.createProject, "CreateProject"),
		get(RouteProject, "board.html", h.showBoard, "GetProject"),
		post(RouteProjectEdit, h.updateProject, "UpdateProject"),
		post(RouteProjectArch, h.archiveProject, "ArchiveProject"),
		post(RouteProjectDel, h.deleteProject, "DeleteProject"),
		post(RouteBoardMove, h.moveCard, "TransitionTask"),
		get(RouteFields, "fields.html", h.showFields, "ListFieldDefs"),
		post(RouteFields, h.putField, "PutFieldDef"),
		post(RouteFieldDelete, h.deleteField, "DeleteFieldDef"),
	}
}

// projectsView is what the project list renders. Editing a project, and
// archiving or deleting it, happen on the project's own page (board.html),
// which has carried a full settings disclosure and a danger zone since the
// board rework; the listing only ever needs to link to it, not to carry a
// second copy of that form. Workflows and Colors are still needed here for
// the "New project" form below the table.
type projectsView struct {
	Projects   []core.Project
	Workflows  []core.Workflow
	Colors     []core.ProjectColor
	NextCursor string
}

// showProjects renders the project list and the creation form.
func (h *handler) showProjects(w http.ResponseWriter, r *http.Request) error {
	filter := core.ProjectFilter{
		IncludeArchived: true,
		Page:            core.Page{Cursor: r.URL.Query().Get("cursor")},
	}
	projects, next, err := h.svc.ListProjects(r.Context(), filter)
	if err != nil {
		return err
	}
	workflows, err := h.svc.ListWorkflows(r.Context())
	if err != nil {
		return err
	}
	return h.render(w, r, "projects.html", "Projects",
		projectsView{Projects: projects, Workflows: workflows,
			Colors: core.ProjectColors(), NextCursor: next})
}

// createProject creates a project from the list screen's form.
func (h *handler) createProject(w http.ResponseWriter, r *http.Request) error {
	in := core.CreateProjectInput{
		Key:         field(r, "key"),
		Name:        field(r, "name"),
		Description: field(r, "description"),
		WorkflowKey: field(r, "workflow_key"),
		Color:       field(r, "color"),
		Icon:        field(r, "icon"),
	}
	project, err := h.svc.CreateProject(r.Context(), in)
	if err != nil {
		return err
	}
	redirect(w, r, RouteProjects+"/"+project.Key, "project created")
	return nil
}

// updateProject changes a project's mutable fields.
func (h *handler) updateProject(w http.ResponseWriter, r *http.Request) error {
	key := r.PathValue("key")
	name, description := field(r, "name"), field(r, "description")
	color, icon := field(r, "color"), field(r, "icon")
	in := core.UpdateProjectInput{Name: &name, Description: &description,
		Color: &color, Icon: &icon}
	if wf := field(r, "workflow_key"); wf != "" {
		in.WorkflowKey = &wf
	}
	if _, err := h.svc.UpdateProject(r.Context(), key, in); err != nil {
		return err
	}
	redirect(w, r, RouteProjects+"/"+key, "project updated")
	return nil
}

// archiveProject archives a project.
func (h *handler) archiveProject(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.ArchiveProject(r.Context(), r.PathValue("key")); err != nil {
		return err
	}
	redirect(w, r, RouteProjects, "project archived")
	return nil
}

// deleteProject deletes a project.
func (h *handler) deleteProject(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.DeleteProject(r.Context(), r.PathValue("key")); err != nil {
		return err
	}
	redirect(w, r, RouteProjects, "project deleted")
	return nil
}

// card is one task on the board with the states it may move to.
type card struct {
	Task    core.Task
	Targets []core.State
}

// column is one workflow state and the tasks sitting in it.
type column struct {
	State core.State
	Tasks []card
}

// boardView is what the project board renders.
type boardView struct {
	Project   core.Project
	Workflow  core.Workflow
	Workflows []core.Workflow
	Colors    []core.ProjectColor
	Columns   []column
}

// showBoard renders one column per workflow state.
func (h *handler) showBoard(w http.ResponseWriter, r *http.Request) error {
	key := r.PathValue("key")
	project, err := h.svc.GetProject(r.Context(), key)
	if err != nil {
		return err
	}
	workflows, err := h.svc.ListWorkflows(r.Context())
	if err != nil {
		return err
	}
	workflow, err := workflowFor(project, workflows)
	if err != nil {
		return err
	}
	page, err := h.svc.ListTasks(r.Context(), core.TaskFilter{
		ProjectKeys: []string{project.Key},
		Page:        core.Page{Limit: core.MaxPageLimit},
	})
	if err != nil {
		return err
	}
	return h.render(w, r, "board.html", project.Name,
		boardView{Project: *project, Workflow: *workflow, Workflows: workflows, Colors: core.ProjectColors(),
			Columns: buildColumns(workflow.Definition, page.Tasks)})
}

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

// buildColumns groups tasks into the workflow's states, in declared order.
func buildColumns(def core.WorkflowDefinition, tasks []core.Task) []column {
	columns := make([]column, 0, len(def.States))
	for _, state := range def.States {
		col := column{State: state}
		for _, task := range tasks {
			if task.Status == state.Key {
				col.Tasks = append(col.Tasks, card{Task: task, Targets: targetsFrom(def, task.Status)})
			}
		}
		columns = append(columns, col)
	}
	return columns
}

// targetsFrom lists the states a task in the given state may move to.
func targetsFrom(def core.WorkflowDefinition, from string) []core.State {
	var out []core.State
	for _, t := range def.Transitions {
		if t.From != from {
			continue
		}
		if state, ok := def.State(t.To); ok {
			out = append(out, state)
		}
	}
	return out
}

// moveCard applies a board move, which is an ordinary transition.
func (h *handler) moveCard(w http.ResponseWriter, r *http.Request) error {
	ref, err := core.ParseTaskRef(field(r, "ref"))
	if err != nil {
		return err
	}
	in := core.TransitionInput{To: field(r, "to"), Comment: field(r, "comment")}
	if _, err := h.svc.TransitionTask(r.Context(), ref, in); err != nil {
		return err
	}
	redirect(w, r, RouteProjects+"/"+r.PathValue("key"), "task "+ref.String()+" moved")
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
		return err
	}
	in := core.FieldDefInput{
		Key:         field(r, "key"),
		Label:       field(r, "label"),
		Type:        core.FieldType(field(r, "type")),
		Required:    checked(r, "required"),
		EnumOptions: fieldList(r, "enum_options"),
		Indexed:     checked(r, "indexed"),
		Position:    position,
	}
	if _, err := h.svc.PutFieldDef(r.Context(), key, in); err != nil {
		return err
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
