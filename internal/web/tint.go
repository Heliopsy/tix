// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "net/http"

// TintCookie remembers what the colour down the side of a row, and across the
// top of a board card, is saying.
//
// It was always the project, decided for everybody by whoever set the
// project's colour. But what a reader wants marked out is a property of how
// they are working, not of the tenant: somebody triaging wants to see
// priority at a glance, somebody watching a board wants status, somebody
// running several projects at once wants the project, and somebody who finds
// all of it noisy wants none of it. One tenant, several readers, several
// answers.
//
// A cookie beside the theme, the column picker and the task view, for the
// same reason those are: it is a property of the reader.
const TintCookie = "tix_tint"

// The things a row or card can be coloured by.
const (
	TintProject  = "project"
	TintStatus   = "status"
	TintPriority = "priority"
	TintNone     = "none"
)

// Tints are the choices the control offers, in the order it offers them.
var Tints = []string{TintProject, TintStatus, TintPriority, TintNone}

// tintLabels name each choice on the control.
var tintLabels = map[string]string{
	TintProject:  "Project colour",
	TintStatus:   "Status",
	TintPriority: "Priority",
	TintNone:     "Nothing",
}

// tintChoice is one option of the picker.
type tintChoice struct {
	Value    string
	Label    string
	Selected bool
}

// tintChoices is the picker, with the reader's current choice marked.
func tintChoices(current string) []tintChoice {
	out := make([]tintChoice, 0, len(Tints))
	for _, t := range Tints {
		out = append(out, tintChoice{Value: t, Label: tintLabels[t], Selected: t == current})
	}
	return out
}

// tintOf reports what this browser colours its rows and cards by. Anything
// unrecognised -- no cookie, a stale value, something another program left on
// the host -- is the project, which is what every install has shown so far.
func tintOf(r *http.Request) string {
	switch cookieValue(r, TintCookie) {
	case TintStatus:
		return TintStatus
	case TintPriority:
		return TintPriority
	case TintNone:
		return TintNone
	default:
		return TintProject
	}
}

// setTint records what this browser colours by.
//
// The project stores nothing rather than storing "project": it is the
// default, so an absent cookie and an empty one mean the same thing, and
// going back to the default leaves nothing behind. The task view switch and
// the column picker do the same.
func (h *handler) setTint(w http.ResponseWriter, r *http.Request) error {
	value := ""
	switch field(r, "tint") {
	case TintStatus:
		value = TintStatus
	case TintPriority:
		value = TintPriority
	case TintNone:
		value = TintNone
	}
	age := cookieYear
	if value == "" {
		age = -1
	}
	// #nosec G124 -- a display preference, readable by no script; Secure
	// tracks TLS like every other cookie here.
	http.SetCookie(w, &http.Cookie{
		Name: TintCookie, Value: value, Path: "/",
		HttpOnly: true, Secure: h.secureCookie(r), SameSite: http.SameSiteLaxMode,
		MaxAge: age,
	})
	// #nosec G710 -- safeNext rejects anything that is not a relative path on
	// this origin, including protocol-relative, backslash and control forms.
	http.Redirect(w, r, safeNext(field(r, "next")), http.StatusSeeOther)
	return nil
}
