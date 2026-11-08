// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// taskScope narrows the task screen to one project: the project page is the
// task screen with project:KEY folded into its filter and its own path in
// every link it writes, not a second listing.
type taskScope struct {
	Project  core.Project
	Workflow core.Workflow

	// Export is the Export dialog, nil for a reader who may not export.
	Export *exportForm
	// Import offers the import steps to a reader who may import.
	Import bool
}

// route is the screen's own path: the project page, or the task list when
// there is no scope.
func (s *taskScope) route() string {
	if s == nil {
		return RouteTasks
	}
	return strings.Replace(RouteProject, "{key}", url.PathEscape(s.Project.Key), 1)
}

// liveRoute is where the scoped screen fetches its live fragment.
func (s *taskScope) liveRoute() string {
	return boardLiveRoute(s.Project.Key)
}

// boardView is the arrangement the board is kept under. A project's board
// keeps the one its own page always had, so columns a reader arranged there
// stay arranged.
func (s *taskScope) boardView() string {
	if s == nil {
		return core.ViewBoard
	}
	return core.ProjectView(s.Project.Key)
}

// scopedRequest is the request with the project control fixed to key. A
// project parameter the address carries from the task list is replaced, not
// added to: the path already chose the project.
func scopedRequest(r *http.Request, key string) *http.Request {
	q := r.URL.Query()
	q.Set(ProjectParam, key)
	out := r.Clone(r.Context())
	u := *r.URL
	u.RawQuery = q.Encode()
	out.URL = &u
	return out
}

// showProjectTasks renders a project's page: the task screen narrowed to it,
// drawn as a list, a board or lanes. The board is what it opens on.
func (h *handler) showProjectTasks(w http.ResponseWriter, r *http.Request) error {
	scope, err := h.projectScope(r)
	if err != nil {
		return err
	}
	if redirectOldKey(w, r, r.PathValue("key"), &scope.Project) {
		return nil
	}
	data, err := h.buildTaskScreen(r, false, scope)
	if err != nil {
		return err
	}
	return h.render(w, r, "tasks.html", scope.Project.Name, data)
}

// showProjectTasksLive renders the regions a live update of the project page
// replaces, from the same view the page is built from.
func (h *handler) showProjectTasksLive(w http.ResponseWriter, r *http.Request) error {
	scope, err := h.projectScope(r)
	if err != nil {
		return err
	}
	if redirectOldKey(w, r, r.PathValue("key"), &scope.Project) {
		return nil
	}
	data, err := h.buildTaskScreen(r, false, scope)
	if err != nil {
		return err
	}
	return h.renderFragment(w, r, "tasks.html", "tasks-live", scope.Project.Name, data)
}

// projectScope reads the project the path names and what its header offers.
func (h *handler) projectScope(r *http.Request) (*taskScope, error) {
	project, err := h.svc.GetProject(r.Context(), r.PathValue("key"))
	if err != nil {
		return nil, err
	}
	workflow, err := h.workflowOf(r, project)
	if err != nil {
		return nil, err
	}
	scope := &taskScope{Project: *project, Workflow: *workflow}
	actor, ok := core.ActorFrom(r.Context())
	if ok && actor.HasScope(core.ScopeExport) {
		defs, err := h.svc.ListFieldDefs(r.Context(), project.Key)
		if err != nil {
			return nil, err
		}
		scope.Export = newExportForm(defs)
	}
	scope.Import = ok && actor.HasScope(core.ScopeImport)
	return scope, nil
}
