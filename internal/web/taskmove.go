// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"strconv"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// moveView is what the move form draws: the task being moved and the other
// projects it may go to.
type moveView struct {
	Task     core.Task
	Projects []core.Project
	// Project is the key the form selects: the first offered one when it is
	// first drawn, the submitted one when a move was refused.
	Project string
	// Errors names the refusal and the field it is about, when there was one.
	Errors fieldErrors
}

// showMoveTask draws the form that moves a task to another project.
func (h *handler) showMoveTask(w http.ResponseWriter, r *http.Request) error {
	data, err := h.buildMoveView(r)
	if err != nil {
		return err
	}
	if len(data.Projects) > 0 {
		data.Project = data.Projects[0].Key
	}
	return h.render(w, r, "taskmove.html", output.ShowRef(data.Task.Ref), data)
}

// buildMoveView loads the task a move form is about and the projects it may
// go to: every live one the reader may create tasks in but its own.
func (h *handler) buildMoveView(r *http.Request) (moveView, error) {
	ref, err := taskRefOf(r)
	if err != nil {
		return moveView{}, err
	}
	task, err := h.svc.GetTask(r.Context(), ref)
	if err != nil {
		return moveView{}, err
	}
	projects, _, err := h.allProjects(r, false)
	if err != nil {
		return moveView{}, err
	}
	data := moveView{Task: *task}
	for _, p := range copyTargets(r, projects) {
		if p.ID != task.ProjectID {
			data.Projects = append(data.Projects, p)
		}
	}
	return data, nil
}

// moveTask moves a task from the form and opens it under its new reference.
// A refusal the reader can correct draws the form again with the message.
func (h *handler) moveTask(w http.ResponseWriter, r *http.Request) error {
	ref, err := taskRefOf(r)
	if err != nil {
		return err
	}
	in := core.MoveTaskInput{ProjectRef: field(r, "project_ref")}
	if v := field(r, "version"); v != "" {
		if in.Version, err = strconv.Atoi(v); err != nil {
			return core.Invalid("version %q is not a number", v)
		}
	}
	moved, err := h.svc.MoveTask(r.Context(), ref, in)
	if err != nil {
		return h.refuseMove(w, r, in.ProjectRef, err)
	}
	redirect(w, r, RouteTasks+"/"+moved.Ref, "task moved")
	return nil
}

// refuseMove answers a move the reader can correct by drawing the form again
// with the project they chose and the message under it.
func (h *handler) refuseMove(w http.ResponseWriter, r *http.Request, project string, err error) error {
	if !refusable(err) {
		return err
	}
	data, viewErr := h.buildMoveView(r)
	if viewErr != nil {
		return viewErr
	}
	data.Project, data.Errors = project, refusal(err)
	status := core.KindOf(err).HTTPStatus()
	if dialogRequest(r) {
		status = http.StatusOK
	}
	return h.renderStatus(w, r, status, "taskmove.html", data.Task.Ref, data)
}
