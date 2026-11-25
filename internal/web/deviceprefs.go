// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
)

// Browser preferences that came over from the app: the screen / opens on,
// and a reduce motion switch for a reader whose system does not ask for it.
const (
	RouteStartScreen = "/startscreen"
	RouteMotion      = "/motion"

	// StartScreenCookie remembers the screen this browser opens on.
	StartScreenCookie = "tix_start"
	// MotionCookie remembers whether this browser keeps the interface still.
	MotionCookie = "tix_motion"
)

// startScreen is one screen / may open on.
type startScreen struct {
	Value string
	Label string
	Route string
}

// startScreens are the screens on offer, the projects list first: it is what
// a browser that never chose opens on.
var startScreens = []startScreen{
	{Value: "", Label: "Projects", Route: RouteProjects},
	{Value: "tasks", Label: "Tasks", Route: "/tasks"},
	{Value: "mine", Label: "My work", Route: RouteMine},
	{Value: "inbox", Label: "Inbox", Route: RouteInbox},
}

// startChoice is one option of the start screen picker.
type startChoice struct {
	Value    string
	Label    string
	Selected bool
}

// startScreenOf is the screen this browser opens on, the projects list for
// no cookie or a value no screen answers to.
func startScreenOf(r *http.Request) startScreen {
	value := cookieValue(r, StartScreenCookie)
	for _, s := range startScreens {
		if s.Value != "" && s.Value == value {
			return s
		}
	}
	return startScreens[0]
}

// startChoices is the picker, with this browser's choice marked.
func startChoices(r *http.Request) []startChoice {
	current := startScreenOf(r).Value
	out := make([]startChoice, 0, len(startScreens))
	for _, s := range startScreens {
		out = append(out, startChoice{Value: s.Value, Label: s.Label, Selected: s.Value == current})
	}
	return out
}

// reduceMotion reports whether this browser asked for a still interface, on
// top of whatever the system's prefers-reduced-motion says.
func reduceMotion(r *http.Request) bool { return cookieValue(r, MotionCookie) == "reduce" }

// devicePrefRoutes store the two preferences.
func (h *handler) devicePrefRoutes() []route {
	return []route{
		post(RouteStartScreen, h.setStartScreen, "Logout"),
		post(RouteMotion, h.toggleMotion, "Logout"),
	}
}

// setStartScreen records the screen this browser opens on. The projects
// list stores nothing, the way the other preferences store nothing for
// their default.
func (h *handler) setStartScreen(w http.ResponseWriter, r *http.Request) error {
	value := ""
	for _, s := range startScreens {
		if s.Value == field(r, "start") {
			value = s.Value
		}
	}
	h.setPrefCookie(w, r, StartScreenCookie, value)
	// #nosec G710 -- safeNext rejects anything that is not a relative path on
	// this origin, including protocol-relative, backslash and control forms.
	http.Redirect(w, r, safeNext(field(r, "next")), http.StatusSeeOther)
	return nil
}

// toggleMotion turns reduce motion on or off for this browser.
func (h *handler) toggleMotion(w http.ResponseWriter, r *http.Request) error {
	value := "reduce"
	if reduceMotion(r) {
		value = ""
	}
	h.setPrefCookie(w, r, MotionCookie, value)
	// #nosec G710 -- safeNext rejects anything that is not a relative path on
	// this origin, including protocol-relative, backslash and control forms.
	http.Redirect(w, r, safeNext(field(r, "next")), http.StatusSeeOther)
	return nil
}

// setPrefCookie stores one display preference for a year, or clears it when
// value is empty.
func (h *handler) setPrefCookie(w http.ResponseWriter, r *http.Request, name, value string) {
	age := cookieYear
	if value == "" {
		age = -1
	}
	// #nosec G124 -- a display preference, readable by no script; Secure
	// tracks TLS like every other cookie here.
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: value, Path: "/",
		HttpOnly: true, Secure: h.secureCookie(r), SameSite: http.SameSiteLaxMode,
		MaxAge: age,
	})
}
