// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// copyView is what the copy form draws: the task being copied and the
// projects a copy may land in.
type copyView struct {
	Task     core.Task
	Projects []core.Project
	// Home is the key of the task's own project, the one the form selects.
	Home string
	// Next is the listing the form was opened from, where the copy returns
	// the reader; empty when it was opened from the task screen.
	Next string
	// Form is what the form holds: the task's own values when it is first
	// drawn, and everything the reader submitted when a copy was refused.
	Form copyForm
	// Errors names the refusal and the field it is about, when there was one.
	Errors fieldErrors
}

// copyForm is what the copy form was submitted with.
type copyForm struct {
	Title, Project, Next                               string
	KeepStatus, Comments, Subtasks, Dependencies, Link bool
}

// readCopyForm reads a submitted copy form.
func readCopyForm(r *http.Request) copyForm {
	return copyForm{
		Title: field(r, "title"), Project: field(r, "project_ref"), Next: copyReturn(field(r, "next")),
		KeepStatus: checked(r, "keep_status"), Comments: checked(r, "with_comments"),
		Subtasks: checked(r, "with_subtasks"), Dependencies: checked(r, "with_dependencies"),
		Link: checked(r, "link"),
	}
}

// input is what the service copies the task with.
func (f copyForm) input() core.CopyTaskInput {
	return core.CopyTaskInput{
		ProjectRef: f.Project, Title: f.Title, KeepStatus: f.KeepStatus,
		WithComments: f.Comments, WithSubtasks: f.Subtasks, WithDependencies: f.Dependencies,
		Link: f.Link,
	}
}

// copyReturn is the listing a copy form was opened from, or "" when the
// request names none this trusts.
func copyReturn(next string) string {
	if next == "" {
		return ""
	}
	back := safeNext(next)
	path, query, _ := strings.Cut(back, "?")
	if !isListing(path) {
		return ""
	}
	values, err := url.ParseQuery(query)
	if err != nil {
		return path
	}
	return withoutFlash(path, values)
}

// showCopyTask draws the form that copies a task to a project.
func (h *handler) showCopyTask(w http.ResponseWriter, r *http.Request) error {
	data, err := h.buildCopyView(r)
	if err != nil {
		return err
	}
	data.Form = copyForm{Title: data.Task.Title, Project: data.Home, Next: copyReturn(r.URL.Query().Get("next")), Link: true}
	data.Next = data.Form.Next
	return h.render(w, r, "taskcopy.html", data.Task.Ref, data)
}

// buildCopyView loads the task a copy form is about and where it may go.
func (h *handler) buildCopyView(r *http.Request) (copyView, error) {
	ref, err := taskRefOf(r)
	if err != nil {
		return copyView{}, err
	}
	task, err := h.svc.GetTask(r.Context(), ref)
	if err != nil {
		return copyView{}, err
	}
	projects, _, err := h.allProjects(r, false)
	if err != nil {
		return copyView{}, err
	}
	data := copyView{Task: *task, Home: ref.ProjectKey, Projects: copyTargets(r, projects)}
	for _, p := range data.Projects {
		if p.ID == task.ProjectID {
			data.Home = p.Key
		}
	}
	return data, nil
}

// refuseCopy answers a copy the reader can correct by drawing the form again,
// every value they chose still in it and the message under the field it is
// about. Anything else goes to the error screen.
func (h *handler) refuseCopy(w http.ResponseWriter, r *http.Request, form copyForm, err error) error {
	if !refusable(err) {
		return err
	}
	data, viewErr := h.buildCopyView(r)
	if viewErr != nil {
		return viewErr
	}
	data.Form, data.Next, data.Errors = form, form.Next, refusal(err)
	status := core.KindOf(err).HTTPStatus()
	// The dialog draws only a successful answer and reads anything else as
	// the error screen, so the form it is to draw again is sent as one.
	if dialogRequest(r) {
		status = http.StatusOK
	}
	return h.renderStatus(w, r, status, "taskcopy.html", data.Task.Ref, data)
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
	form := readCopyForm(r)
	copied, err := h.svc.CopyTask(r.Context(), ref, form.input())
	if err != nil {
		return h.refuseCopy(w, r, form, err)
	}
	if openAfterAdd(r) {
		redirect(w, r, RouteTasks+"/"+copied.Ref, "task copied")
		return nil
	}
	source := RouteTasks + "/" + url.PathEscape(r.PathValue("ref"))
	if form.Next != "" {
		source = form.Next
	}
	redirect(w, r, flashRef(source, copied.Ref), "task copied")
	return nil
}
