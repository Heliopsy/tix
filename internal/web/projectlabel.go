// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// ProjectLabelCookie remembers how a project is named where one is chosen: the
// add box, Add with details, Copy to, New from template, a heading menu's add
// field, the move target of a project delete and the statistics filter.
//
// A cookie beside the tint and the task view rather than a row on the server:
// every other display preference is one, the settings screen says so in its
// first line, and a reader who knows projects by key on one machine may know
// them by name on another.
const ProjectLabelCookie = "tix_project_label"

// The ways a project can be named in a picker.
const (
	ProjectLabelNameKey = "name-key"
	ProjectLabelKeyName = "key-name"
	ProjectLabelName    = "name"
	ProjectLabelKey     = "key"
)

// ProjectLabels are the choices the control offers, in the order it offers
// them. The first is the default.
var ProjectLabels = []string{ProjectLabelNameKey, ProjectLabelKeyName, ProjectLabelName, ProjectLabelKey}

// projectLabelNames name each choice on the control, as an example of what it
// draws.
var projectLabelNames = map[string]string{
	ProjectLabelNameKey: "Name (KEY)",
	ProjectLabelKeyName: "KEY · Name",
	ProjectLabelName:    "Name",
	ProjectLabelKey:     "KEY",
}

// labelNameLimit is the most of a project's name a picker shows. A native
// select cuts its text at the end, which is where the key sits by default, so
// the name is shortened here and the key never is.
const labelNameLimit = 24

// projectLabelChoice is one option of the picker.
type projectLabelChoice struct {
	Value    string
	Label    string
	Selected bool
}

// projectLabelChoices is the picker, with the reader's current choice marked.
func projectLabelChoices(current string) []projectLabelChoice {
	out := make([]projectLabelChoice, 0, len(ProjectLabels))
	for _, l := range ProjectLabels {
		out = append(out, projectLabelChoice{Value: l, Label: projectLabelNames[l], Selected: l == current})
	}
	return out
}

// knownProjectLabel reports the style s names, or "" for anything else.
func knownProjectLabel(s string) string {
	for _, l := range ProjectLabels[1:] {
		if s == l {
			return l
		}
	}
	return ""
}

// projectLabelOf reports how this browser names projects in a picker. No
// cookie, or one naming no style this build draws, is the name with the key.
func projectLabelOf(r *http.Request) string {
	if l := knownProjectLabel(cookieValue(r, ProjectLabelCookie)); l != "" {
		return l
	}
	return ProjectLabelNameKey
}

// projectLabel is the text of one project's option in a picker, in style. The
// option's value stays the stored key; only what the reader reads changes. A
// text icon leads when the project has one, since an option holds text only.
func projectLabel(style string, p core.Project) string {
	key := output.ShowKey(p.Key)
	name := shortName(strings.TrimSpace(p.Name))
	var label string
	switch {
	case name == "" || style == ProjectLabelKey:
		label = key
	case style == ProjectLabelKeyName:
		label = key + " · " + name
	case style == ProjectLabelName:
		label = name
	default:
		label = name + " (" + key + ")"
	}
	if icon := strings.TrimSpace(p.Icon); icon != "" {
		label = icon + " " + label
	}
	return label
}

// projectShortLabel is what a compact picker switches to on a narrow screen,
// where a native select keeps only the start of its text and is capped to a
// few characters: the key alone, so the name is what goes and the key never
// is. It is empty when the label is already the key or holds none.
func projectShortLabel(style string, p core.Project) string {
	if strings.TrimSpace(p.Name) == "" || style == ProjectLabelKey || style == ProjectLabelName {
		return ""
	}
	return output.ShowKey(p.Key)
}

// shortName cuts a name to labelNameLimit characters, marking the cut.
func shortName(name string) string {
	runes := []rune(name)
	if len(runes) <= labelNameLimit {
		return name
	}
	return strings.TrimSpace(string(runes[:labelNameLimit-1])) + "…"
}

// setProjectLabel records how this browser names projects in a picker. The
// default stores nothing, like the tint and the task view.
func (h *handler) setProjectLabel(w http.ResponseWriter, r *http.Request) error {
	value := knownProjectLabel(field(r, "label"))
	age := cookieYear
	if value == "" {
		age = -1
	}
	// #nosec G124 -- a display preference, readable by no script; Secure
	// tracks TLS like every other cookie here.
	http.SetCookie(w, &http.Cookie{
		Name: ProjectLabelCookie, Value: value, Path: "/",
		HttpOnly: true, Secure: h.secureCookie(r), SameSite: http.SameSiteLaxMode,
		MaxAge: age,
	})
	// #nosec G710 -- safeNext rejects anything that is not a relative path on
	// this origin, including protocol-relative, backslash and control forms.
	http.Redirect(w, r, safeNext(field(r, "next")), http.StatusSeeOther)
	return nil
}
