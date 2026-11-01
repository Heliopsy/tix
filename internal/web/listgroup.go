// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "net/http"

// ListGroupCookie remembers whether the task screen's list draws its rows
// under a heading per project or per state. Kept per browser beside the view
// switch, for the same reason: it says how one set of tasks is drawn, never
// which tasks are selected. The arrangement of the groups themselves is the
// reader's own and lives on the server (groups.go).
const ListGroupCookie = "tix_list_group"

// How the list groups its rows. Ungrouped stores nothing.
const (
	ListGroupNone    = ""
	ListGroupProject = "project"
	ListGroupStatus  = "status"
)

// listGroupLabels name each choice on the control, in the order it offers them.
var listGroupLabels = []taskViewChoice{
	{Value: ListGroupNone, Label: "No groups"},
	{Value: ListGroupProject, Label: "By project"},
	{Value: ListGroupStatus, Label: "By status"},
}

// listGroupOf reports how this browser groups the list.
func listGroupOf(r *http.Request) string {
	switch v := cookieValue(r, ListGroupCookie); v {
	case ListGroupProject, ListGroupStatus:
		return v
	}
	return ListGroupNone
}

// listGroupFormView is the grouping control, the same form on the View menu
// and on the settings screen.
type listGroupFormView struct {
	CSRF    string
	Here    string
	Choices []taskViewChoice
}

func newListGroupForm(csrf, here, current string) listGroupFormView {
	out := listGroupFormView{CSRF: csrf, Here: here}
	for _, c := range listGroupLabels {
		c.Selected = c.Value == current
		out.Choices = append(out.Choices, c)
	}
	return out
}

// setListGroup records how this browser groups the list.
func (h *handler) setListGroup(w http.ResponseWriter, r *http.Request) error {
	value := field(r, "group")
	if value != ListGroupProject && value != ListGroupStatus {
		value = ListGroupNone
	}
	age := cookieYear
	if value == ListGroupNone {
		age = -1
	}
	// #nosec G124 -- a display preference, readable by no script; Secure
	// tracks TLS like every other cookie here.
	http.SetCookie(w, &http.Cookie{
		Name: ListGroupCookie, Value: value, Path: "/",
		HttpOnly: true, Secure: h.secureCookie(r), SameSite: http.SameSiteLaxMode,
		MaxAge: age,
	})
	// #nosec G710 -- safeNext rejects anything that is not a relative path on
	// this origin, including protocol-relative, backslash and control forms.
	http.Redirect(w, r, safeNext(field(r, "next")), http.StatusSeeOther)
	return nil
}
