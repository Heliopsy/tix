package web

import (
	"net/http"
	"sort"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// HiddenProjectsCookie remembers which projects the task screen leaves out.
//
// It is deliberately a cookie of its own rather than another entry inside
// tix_columns. The column preference is checked against a vocabulary this
// build declares, and discards anything it does not recognise; project keys
// are tenant data, unknown when the binary is built, so they could not survive
// that check. It also records the projects that are HIDDEN rather than the
// ones shown, which is what makes a project created tomorrow appear on its
// own instead of waiting for somebody to tick it.
const HiddenProjectsCookie = "tix_hidden_projects"

// maxVisibilityValue bounds the cookie this package will read, so a value
// another program left behind cannot make parsing walk a large string.
const maxVisibilityValue = 1024

// hiddenProjects reads the projects this browser has put away.
func hiddenProjects(r *http.Request) map[string]bool {
	return parseHiddenProjects(cookieValue(r, HiddenProjectsCookie))
}

// parseHiddenProjects reads the compact cookie form, "key.key". A malformed
// or oversized value hides nothing at all, so the failure mode is the
// untouched default rather than a task list that looks empty for no stated
// reason.
func parseHiddenProjects(raw string) map[string]bool {
	out := map[string]bool{}
	if raw == "" || len(raw) > maxVisibilityValue {
		return out
	}
	for _, key := range strings.Split(raw, ".") {
		if projectKeyShape(key) {
			out[key] = true
		}
	}
	return out
}

// projectKeyShape reports whether a value has the shape of a project key, so
// nothing else can be carried in the cookie.
func projectKeyShape(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

// encodeHiddenProjects renders the hidden keys back into the cookie's form.
func encodeHiddenProjects(hidden map[string]bool) string {
	keys := make([]string, 0, len(hidden))
	for key := range hidden {
		if hidden[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return strings.Join(keys, ".")
}

// projectChoice is one project in the visibility control.
type projectChoice struct {
	Key   string
	Name  string
	Color core.ProjectColor
	Icon  string
	Shown bool
}

// projectChoices pairs every project with whether the task list shows it.
func projectChoices(projects []core.Project, hidden map[string]bool) []projectChoice {
	out := make([]projectChoice, 0, len(projects))
	for _, p := range projects {
		out = append(out, projectChoice{Key: p.Key, Name: p.Name, Color: p.Color,
			Icon: p.Icon, Shown: !hidden[p.Key]})
	}
	return out
}

// shownKeys names the projects the task list may draw from.
func shownKeys(choices []projectChoice) []string {
	out := make([]string, 0, len(choices))
	for _, c := range choices {
		if c.Shown {
			out = append(out, c.Key)
		}
	}
	return out
}

// hiddenCount is how many projects are put away, which is what the screen
// says rather than leaving a shorter task list unexplained.
func hiddenCount(choices []projectChoice) int {
	out := 0
	for _, c := range choices {
		if !c.Shown {
			out++
		}
	}
	return out
}

// setVisibility records which projects the task screen shows for this
// browser. The form submits the projects to show, and what is stored is
// everything else, so a project created after the choice was made is visible
// without being ticked.
func (h *handler) setVisibility(w http.ResponseWriter, r *http.Request) error {
	projects, _, err := h.svc.ListProjects(r.Context(), core.ProjectFilter{})
	if err != nil {
		return err
	}
	hidden := map[string]bool{}
	if !checked(r, "reset") {
		shown := map[string]bool{}
		for _, key := range r.PostForm["project"] {
			shown[key] = true
		}
		for _, p := range projects {
			if !shown[p.Key] {
				hidden[p.Key] = true
			}
		}
	}
	value := encodeHiddenProjects(hidden)
	age := cookieYear
	if value == "" {
		age = -1
	}
	// #nosec G124 -- a display preference, readable by no script; Secure
	// tracks TLS like every other cookie here.
	http.SetCookie(w, &http.Cookie{
		Name: HiddenProjectsCookie, Value: value, Path: "/",
		HttpOnly: true, Secure: h.secureCookie(r), SameSite: http.SameSiteLaxMode,
		MaxAge: age,
	})
	// #nosec G710 -- safeNext rejects anything that is not a relative path on
	// this origin, including protocol-relative, backslash and control forms.
	http.Redirect(w, r, safeNext(field(r, "next")), http.StatusSeeOther)
	return nil
}
