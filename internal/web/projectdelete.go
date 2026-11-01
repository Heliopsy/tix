// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// The choices the deletion screen offers for a project's tasks.
const (
	choiceMove   = "move"
	choiceDelete = "delete"
	intentDelete = "delete"
)

// projectDeleteView is what the deletion confirmation renders.
type projectDeleteView struct {
	Project core.Project
	// Plan counts the tasks; Mapping is the plan for the selected target.
	Plan    core.ProjectDeletion
	Mapping *core.ProjectDeletion
	Targets []core.Project
	MoveTo  string
	Choice  string
	Errors  fieldErrors
}

// showProjectDelete renders the deletion confirmation.
func (h *handler) showProjectDelete(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	form := projectDeleteView{MoveTo: q.Get("move_to"), Choice: q.Get("choice")}
	return h.renderProjectDelete(w, r, http.StatusOK, form)
}

// renderProjectDelete draws the confirmation with whatever the reader had
// chosen, and the refusal when there is one.
func (h *handler) renderProjectDelete(w http.ResponseWriter, r *http.Request, status int, form projectDeleteView) error {
	key := r.PathValue("key")
	project, err := h.svc.GetProject(r.Context(), key)
	if err != nil {
		return err
	}
	plan, err := h.svc.PlanProjectDeletion(r.Context(), project.Key, core.DeleteProjectInput{})
	if err != nil {
		return err
	}
	projects, _, err := h.allProjects(r, false)
	if err != nil {
		return err
	}
	form.Project, form.Plan = *project, *plan
	for _, p := range projects {
		if p.ID != project.ID {
			form.Targets = append(form.Targets, p)
		}
	}
	if form.Errors == nil {
		form.Errors = fieldErrors{}
	}
	if len(form.Targets) > 0 && plan.Total() > 0 {
		if !hasProject(form.Targets, form.MoveTo) {
			form.MoveTo = form.Targets[0].Key
		}
		mapping, err := h.svc.PlanProjectDeletion(r.Context(), project.Key,
			core.DeleteProjectInput{MoveTasksTo: form.MoveTo})
		switch {
		case err == nil:
			form.Mapping = mapping
		case correctable(err) || core.KindOf(err) == core.KindForbidden:
			if _, held := form.Errors["move_to"]; !held {
				form.Errors["move_to"] = safeMessage(err, core.KindOf(err))
			}
		default:
			return err
		}
	}
	return h.renderStatus(w, r, status, "projectdelete.html", "Delete project", form)
}

// hasProject reports whether key names one of the projects.
func hasProject(projects []core.Project, key string) bool {
	for _, p := range projects {
		if strings.EqualFold(p.Key, key) {
			return true
		}
	}
	return false
}

// deleteProject performs the deletion the confirmation asked for, or sends
// the reader back to it with the mapping for another target.
//
// Only a submission carrying intent=delete, which the delete buttons send,
// deletes. Anything else redraws the confirmation, so a submission that lost
// its submitter on the way, as a script-driven submit through htmx does,
// previews rather than destroys.
func (h *handler) deleteProject(w http.ResponseWriter, r *http.Request) error {
	key := r.PathValue("key")
	form := projectDeleteView{MoveTo: field(r, "move_to"), Choice: field(r, "choice")}
	if field(r, "intent") != intentDelete {
		if form.Choice == "" {
			form.Choice = choiceMove
		}
		q := url.Values{"move_to": {form.MoveTo}, "choice": {form.Choice}}
		redirect(w, r, RouteProjects+"/"+url.PathEscape(key)+"/delete?"+q.Encode(), "")
		return nil
	}
	in := core.DeleteProjectInput{}
	switch form.Choice {
	case choiceMove:
		in.MoveTasksTo = form.MoveTo
	case choiceDelete:
		if !strings.EqualFold(field(r, "confirm"), key) {
			err := core.Invalid("type the project key %s to confirm", output.ShowKey(key)).WithDetail(core.DetailField, "confirm")
			return h.refuseProjectDelete(w, r, form, err)
		}
		in.DeleteTasks = true
	}
	done, err := h.svc.DeleteProject(r.Context(), key, in)
	if err != nil {
		return h.refuseProjectDelete(w, r, form, err)
	}
	if done.MoveTo != "" && done.Total() > 0 {
		redirect(w, r, RouteProjects+"/"+url.PathEscape(done.MoveTo), "project deleted, its tasks moved here")
		return nil
	}
	redirect(w, r, RouteProjects, "project deleted")
	return nil
}

// refuseProjectDelete draws the confirmation again with the refusal beside
// the control it is about.
func (h *handler) refuseProjectDelete(w http.ResponseWriter, r *http.Request, form projectDeleteView, err error) error {
	if !correctable(err) {
		return err
	}
	form.Errors = refusal(err)
	if core.KindOf(err) == core.KindPrecondition && core.FieldOf(err) == "" {
		form.Errors = fieldErrors{"choice": safeMessage(err, core.KindOf(err))}
	}
	return h.renderProjectDelete(w, r, core.KindOf(err).HTTPStatus(), form)
}
