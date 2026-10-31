// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/url"

	"github.com/heliopsy/tix/internal/core"
)

// copyView is what the copy form draws: the task being copied and the
// projects a copy may land in.
type copyView struct {
	Task     core.Task
	Projects []core.Project
	// Home is the key of the task's own project, the one the form selects.
	Home string
}

// showCopyTask draws the form that copies a task to a project.
func (h *handler) showCopyTask(w http.ResponseWriter, r *http.Request) error {
	ref, err := taskRefOf(r)
	if err != nil {
		return err
	}
	task, err := h.svc.GetTask(r.Context(), ref)
	if err != nil {
		return err
	}
	projects, _, err := h.allProjects(r, false)
	if err != nil {
		return err
	}
	data := copyView{Task: *task, Home: ref.ProjectKey, Projects: copyTargets(r, projects)}
	for _, p := range data.Projects {
		if p.ID == task.ProjectID {
			data.Home = p.Key
		}
	}
	return h.render(w, r, "taskcopy.html", task.Ref, data)
}

// copyTargets are the projects the reader may create a task in. A reader
// pinned to one project is offered that project alone, since the service
// refuses a copy anywhere else.
func copyTargets(r *http.Request, projects []core.Project) []core.Project {
	actor, err := core.RequireActor(r.Context())
	if err != nil || !actor.ScopedToProject() {
		return projects
	}
	out := make([]core.Project, 0, 1)
	for _, p := range projects {
		if p.ID == actor.ProjectID {
			out = append(out, p)
		}
	}
	return out
}

// copyTask copies a task from the form, then either opens the copy or
// returns to the source with a toast linking it, as adding a task does.
func (h *handler) copyTask(w http.ResponseWriter, r *http.Request) error {
	ref, err := taskRefOf(r)
	if err != nil {
		return err
	}
	in := core.CopyTaskInput{
		ProjectRef:       field(r, "project_ref"),
		Title:            field(r, "title"),
		KeepStatus:       checked(r, "keep_status"),
		WithComments:     checked(r, "with_comments"),
		WithSubtasks:     checked(r, "with_subtasks"),
		WithDependencies: checked(r, "with_dependencies"),
		Link:             checked(r, "link"),
	}
	copied, err := h.svc.CopyTask(r.Context(), ref, in)
	if err != nil {
		return err
	}
	if openAfterAdd(r) {
		redirect(w, r, RouteTasks+"/"+copied.Ref, "task copied")
		return nil
	}
	source := RouteTasks + "/" + url.PathEscape(r.PathValue("ref"))
	redirect(w, r, flashRef(source, copied.Ref), "task copied")
	return nil
}
