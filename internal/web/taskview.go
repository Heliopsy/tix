// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "net/http"

// TaskViewCookie remembers whether the task screen draws its tasks as a list
// of rows or as a board of columns. It is the same set of tasks either way --
// one filter, one page, one visibility choice -- so this records how they are
// drawn and never which of them are selected.
//
// A cookie beside the theme and the column picker, for the reason those are:
// how somebody reads a shared queue is a property of the reader, not of the
// tenant, and two people signed into one account want different answers.
const TaskViewCookie = "tix_task_view"

// The two ways the task screen draws one set of tasks.
const (
	TaskViewList  = "list"
	TaskViewBoard = "board"
)

// TaskViews are the views the switch offers, in the order it offers them.
var TaskViews = []string{TaskViewList, TaskViewBoard}

// taskViewLabels name each view on the control.
var taskViewLabels = map[string]string{TaskViewList: "List", TaskViewBoard: "Board"}

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

// taskViewOf reports which view this browser asked for. Anything other than
// the board -- no cookie, a stale value, something another program left on
// the host -- is the list, which is what an untouched install has always
// shown.
func taskViewOf(r *http.Request) string {
	if cookieValue(r, TaskViewCookie) == TaskViewBoard {
		return TaskViewBoard
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
	value := ""
	if field(r, "view") == TaskViewBoard {
		value = TaskViewBoard
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
	// #nosec G710 -- safeNext rejects anything that is not a relative path on
	// this origin, including protocol-relative, backslash and control forms.
	http.Redirect(w, r, safeNext(field(r, "next")), http.StatusSeeOther)
	return nil
}
