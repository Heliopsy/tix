// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "net/http"

// TaskOpenCookie remembers whether following a task link opens the task
// screen as a page or in a dialog over the page it was followed from.
const TaskOpenCookie = "tix_task_open"

// TitleClickCookie remembers whether clicking a task's title in a listing
// opens the task or edits the title where it is.
const TitleClickCookie = "tix_title_click"

// DialogHeader marks a request for the task screen drawn for the dialog: the
// screen's content alone, without the layout around it. assets/taskopen.js
// sends it, and the dialog element carries it on every request made from
// inside it, so a save's redirect is answered the same way.
const DialogHeader = "X-Tix-Dialog"

// taskDialogMode reports whether this browser opens tasks in a dialog. Only
// "dialog" does; absent or anything else is the page, which is what the
// interface did before the preference existed.
func taskDialogMode(r *http.Request) bool {
	return cookieValue(r, TaskOpenCookie) == "dialog"
}

// titleEditMode reports whether clicking a title in a listing edits it. Only
// "edit" does.
func titleEditMode(r *http.Request) bool {
	return cookieValue(r, TitleClickCookie) == "edit"
}

// dialogRequest reports whether the request asked for the dialog's drawing of
// a screen.
func dialogRequest(r *http.Request) bool {
	return r != nil && r.Header.Get(DialogHeader) == "1"
}

// toggleTaskOpen moves this browser between opening a task as a page and in a
// dialog. Both states are written explicitly so a stored choice never depends
// on how an absent cookie is read.
func (h *handler) toggleTaskOpen(w http.ResponseWriter, r *http.Request) error {
	value := "dialog"
	if taskDialogMode(r) {
		value = "page"
	}
	h.setPreference(w, r, TaskOpenCookie, value)
	return nil
}

// toggleTitleClick moves this browser between a title click opening the task
// and editing the title.
func (h *handler) toggleTitleClick(w http.ResponseWriter, r *http.Request) error {
	value := "edit"
	if titleEditMode(r) {
		value = "open"
	}
	h.setPreference(w, r, TitleClickCookie, value)
	return nil
}

// setPreference stores one display preference for a year and returns to the
// page the form named.
func (h *handler) setPreference(w http.ResponseWriter, r *http.Request, name, value string) {
	// #nosec G124 -- a display preference, readable by no script.
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: value, Path: "/",
		HttpOnly: true, Secure: h.secureCookie(r), SameSite: http.SameSiteLaxMode,
		MaxAge: cookieYear,
	})
	// #nosec G710 -- safeNext rejects anything that is not a relative path on
	// this origin, including protocol-relative, backslash and control forms.
	http.Redirect(w, r, safeNext(field(r, "next")), http.StatusSeeOther)
}
