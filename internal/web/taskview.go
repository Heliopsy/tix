// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/url"
)

// TaskViewCookie remembers whether the task screen draws its tasks as a list
// of rows, a board of columns or one lane per project. It is the same set of
// tasks every way -- one filter, one page, one visibility choice -- so this
// records how they are drawn and never which of them are selected.
//
// A cookie beside the theme and the column picker, for the reason those are:
// how somebody reads a shared queue is a property of the reader, not of the
// tenant, and two people signed into one account want different answers.
const TaskViewCookie = "tix_task_view"

// The ways the task screen draws one set of tasks.
const (
	TaskViewList  = "list"
	TaskViewBoard = "board"
	TaskViewLanes = "lanes"
)

// TaskViews are the views the switch offers, in the order it offers them.
var TaskViews = []string{TaskViewList, TaskViewBoard, TaskViewLanes}

// taskViewLabels name each view on the control.
var taskViewLabels = map[string]string{TaskViewList: "List", TaskViewBoard: "Board", TaskViewLanes: "Lanes"}

// knownTaskView reports whether this build draws the named view.
func knownTaskView(v string) bool {
	_, ok := taskViewLabels[v]
	return ok
}

// taskViewChoice is one position of the view switch.
type taskViewChoice struct {
	Value    string
	Label    string
	Selected bool
}

// taskViewChoices is the switch, with the reader's current view marked.
func taskViewChoices(current string) []taskViewChoice {
	out := make([]taskViewChoice, 0, len(TaskViews))
	for _, v := range TaskViews {
		out = append(out, taskViewChoice{Value: v, Label: taskViewLabels[v], Selected: v == current})
	}
	return out
}

// taskViewOf reports which view this browser asked for. Anything this build
// does not draw -- no cookie, a stale value, something another program left
// on the host -- is the list, which is what an untouched install has always
// shown.
func taskViewOf(r *http.Request) string {
	if v := cookieValue(r, TaskViewCookie); knownTaskView(v) {
		return v
	}
	return TaskViewList
}

// setTaskView records how this browser draws the task screen.
//
// The list stores nothing rather than storing "list": the default is the
// list, so an empty value and an absent cookie mean the same thing, and
// going back to the default leaves nothing behind, the way "Show all"
// projects and "Reset" columns do.
func (h *handler) setTaskView(w http.ResponseWriter, r *http.Request) error {
	h.writeTaskView(w, r, field(r, "view"))
	// #nosec G710 -- safeNext rejects anything that is not a relative path on
	// this origin, including protocol-relative, backslash and control forms.
	http.Redirect(w, r, safeNext(field(r, "next")), http.StatusSeeOther)
	return nil
}

// writeTaskView stores the remembered view, clearing the cookie for the list.
func (h *handler) writeTaskView(w http.ResponseWriter, r *http.Request, value string) {
	if !knownTaskView(value) || value == TaskViewList {
		value = ""
	}
	age := cookieYear
	if value == "" {
		age = -1
	}
	// #nosec G124 -- a display preference, readable by no script; Secure
	// tracks TLS like every other cookie here.
	http.SetCookie(w, &http.Cookie{
		Name: TaskViewCookie, Value: value, Path: "/",
		HttpOnly: true, Secure: h.secureCookie(r), SameSite: http.SameSiteLaxMode,
		MaxAge: age,
	})
}

// ViewParam names the drawing in the address. It wins over the remembered
// choice, so a link to a filter on the board opens on the board.
const ViewParam = "view"

// viewParam is the view the address names, or "" when it names none this
// build draws.
func viewParam(r *http.Request) string {
	if v := r.URL.Query().Get(ViewParam); knownTaskView(v) {
		return v
	}
	return ""
}

// screenView is the drawing a task screen request asks for: the address
// first, then the remembered choice on the task list, or the board on a
// project's page, which is what that page has always opened on.
func screenView(r *http.Request, scope *taskScope) string {
	if v := viewParam(r); v != "" {
		return v
	}
	if scope != nil {
		return TaskViewBoard
	}
	return taskViewOf(r)
}

// rememberTaskView stores the view a task list address names, so the next
// visit without one opens the same way.
func (h *handler) rememberTaskView(w http.ResponseWriter, r *http.Request) {
	if v := viewParam(r); v != "" && v != taskViewOf(r) {
		h.writeTaskView(w, r, v)
	}
}

// viewLink is one position of the view switch drawn on a task screen.
type viewLink struct {
	Value    string
	Label    string
	Href     string
	Selected bool
}

// viewDropped are the parameters a switch link leaves behind. A cursor and
// its trail address a position in one drawing's walk, and the list narrows to
// open work where the board does not; the flash and an open dialog belong to
// the page they were drawn on. Everything else stays, including a filter the
// target view does not use, so switching back restores it.
var viewDropped = []string{CursorParam, TrailParam, PageParam, "flash", "flashref", FlashUndoParam, DialogParam}

// viewLinks is the view switch: one link per drawing to route, each carrying
// the request's query with its own view.
func viewLinks(r *http.Request, route, current string) []viewLink {
	base := r.URL.Query()
	for _, k := range viewDropped {
		base.Del(k)
	}
	out := make([]viewLink, 0, len(TaskViews))
	for _, v := range TaskViews {
		q := url.Values{}
		for k, vals := range base {
			q[k] = append([]string(nil), vals...)
		}
		q.Set(ViewParam, v)
		out = append(out, viewLink{Value: v, Label: taskViewLabels[v], Href: route + "?" + q.Encode(), Selected: v == current})
	}
	return out
}
