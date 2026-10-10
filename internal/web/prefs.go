// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
)

// prefsView is every per-browser preference that has more than a switch: the
// column picker of each listing, the project visibility choice, and the
// list-or-board view.
//
// The settings screen and the screen each control used to live on alone now
// render the same preference, which is two controls over one cookie unless
// something stops them being two controls. This type is what stops it. One
// constructor builds it, one partial per preference renders it, one reader
// resolves the cookie, and one handler stores it, so "the two controls
// disagree" has nowhere to happen: there is no second default to drift from,
// no second validation to be laxer, and no second reading of the value.
//
// The alternative considered was a read-only summary on settings with a reset
// beside it, which cannot disagree either, but also cannot express "hide the
// Updated column on Tokens" -- and owning every preference is what the screen
// was asked for.
type prefsView struct {
	// Columns is one picker per listing whose columns can be chosen, in the
	// order the settings screen offers them.
	Columns []columnFormView

	// Projects is the project visibility control.
	Projects visibilityFormView

	// TaskView is the list-or-board switch.
	TaskView taskViewFormView
}

// columnFormView is one listing's column picker: which listing, how it is
// named, what this browser shows on it, and where the form returns to.
type columnFormView struct {
	CSRF       string
	Here       string
	ColumnPage string
	Label      string
	Columns    columnPrefs
}

// Options are the picker's checkboxes, resolved for this listing.
func (c columnFormView) Options() []columnOption { return c.Columns.Options(c.ColumnPage) }

// Configurable reports whether this listing offers a picker at all.
func (c columnFormView) Configurable() bool { return c.Columns.Configurable(c.ColumnPage) }

// visibilityFormView is the project visibility control: every project this
// browser can put away, the hidden keys no project answers to, and how many
// are away.
type visibilityFormView struct {
	CSRF    string
	Here    string
	Choices []projectChoice
	Stale   []string
	Hidden  int
	// Whole is false when the tenant has more projects than the walk behind
	// Choices covered. It lives on the control rather than on either screen,
	// because the task screen and settings render the same control and a
	// caveat held by one of them is a caveat the other silently drops.
	Whole bool
}

// taskViewFormView is the list-or-board switch. Current is what the reader
// has, which is the list whenever the cookie says anything else, so the
// switch has two positions and not three.
type taskViewFormView struct {
	CSRF    string
	Here    string
	Current string
	Choices []taskViewChoice
}

// Preference is one per-browser display preference: the cookie it is kept
// in, the route its control submits to, and the words the documentation
// table names it under.
type Preference struct {
	Cookie string
	Route  string
	Label  string
}

// Preferences is every per-browser display preference this interface has.
//
// It is the list the settings screen is held to, rather than a comment asking
// a reviewer to notice: a preference added here with no control on that
// screen fails TestSettingsOwnsEveryPreference, and one whose documentation
// row is missing fails TestEveryPreferenceIsDocumented. The session cookie,
// the CSRF token and the one-shot issued-token cookie are deliberately absent
// -- they are mechanism, carry no reader's choice, and there is nothing on a
// settings screen for them to be.
var Preferences = []Preference{
	{Cookie: ThemeCookie, Route: RouteTheme, Label: "Colour scheme"},
	{Cookie: KeySchemeCookie, Route: RouteKeyScheme, Label: "Keyboard shortcuts"},
	{Cookie: TimeFormatCookie, Route: RouteTimeFormat, Label: "Date format"},
	{Cookie: TimezoneCookie, Route: RouteTimezone, Label: "Timezone"},
	{Cookie: AdvancedCookie, Route: RouteAdvanced, Label: "Advanced screens"},
	{Cookie: DragMoveCookie, Route: RouteDragMove, Label: "Drag to move"},
	{Cookie: ColumnsCookie, Route: RouteColumns, Label: "Columns"},
	{Cookie: HiddenProjectsCookie, Route: RouteVisibility, Label: "Hidden projects"},
	{Cookie: TaskViewCookie, Route: RouteTaskView, Label: "Task view"},
}

// columnPageName names one listing whose columns can be chosen.
type columnPageName struct {
	Page  string
	Label string
}

// columnPages names every listing with a column picker, in the order the
// settings screen offers them, under the word that listing is known by on its
// own screen. columnSets decides which listings exist; this decides only how
// they are named and ordered, and TestSettingsOffersEveryListingsColumns
// holds the two together rather than a reviewer remembering to.
var columnPages = []columnPageName{
	{Page: "tasks", Label: "Tasks"},
	{Page: "projects", Label: "Projects"},
	{Page: "users", Label: "Users"},
	{Page: "tokens", Label: "API tokens"},
	{Page: "sshkeys", Label: "SSH keys"},
	{Page: "webhooks", Label: "Webhooks"},
	{Page: "domains", Label: "Domains"},
}

// columnLabel is the word a listing is offered under, falling back to its own
// name so a listing declared in columnSets and not yet named here reads as
// its key rather than as nothing. TestSettingsOffersEveryListingsColumns is
// what stops that state persisting.
func columnLabel(page string) string {
	for _, p := range columnPages {
		if p.Page == page {
			return p.Label
		}
	}
	return page
}

// newColumnForm builds one listing's picker. Every column picker in this
// interface comes through here -- the one beside a listing and the one on
// settings alike -- so neither can resolve the cookie differently from the
// other.
func newColumnForm(csrf, back, page, label string, cols columnPrefs) columnFormView {
	return columnFormView{CSRF: csrf, Here: back, ColumnPage: page, Label: label, Columns: cols}
}

// prefsFor builds every shared preference control for one request, from the
// projects the calling screen has already listed.
func (h *handler) prefsFor(r *http.Request, projects []core.Project, whole bool) prefsView {
	csrf, back := csrfFrom(r), here(r)
	cols := columnsOf(r)
	pages := make([]columnFormView, 0, len(columnPages))
	for _, p := range columnPages {
		pages = append(pages, newColumnForm(csrf, back, p.Page, p.Label, cols))
	}
	away := hiddenProjects(r)
	choices := projectChoices(projects, away)
	current := taskViewOf(r)
	return prefsView{
		Columns: pages,
		Projects: visibilityFormView{
			CSRF: csrf, Here: back, Choices: choices, Whole: whole,
			Stale: staleHiddenProjects(projects, away), Hidden: hiddenCount(choices),
		},
		TaskView: taskViewFormView{
			CSRF: csrf, Here: back, Current: current, Choices: taskViewChoices(current),
		},
	}
}

// staleHiddenProjects names the hidden keys this listing has no project for.
//
// A project deleted or archived since the choice was made leaves its key in
// the cookie, where it goes on excluding nothing visible and cannot be
// unticked, because the checkbox that would untick it has no project to
// render from. On the task screen that is a listing one project shorter with
// nothing on the page accounting for it; on settings, with no task list in
// front of the reader, it is invisible. Naming them is what lets the control
// say so, and any submission of the form drops them, since what is stored is
// derived from the projects the form offered.
func staleHiddenProjects(projects []core.Project, hidden map[string]bool) []string {
	live := make(map[string]bool, len(projects))
	for _, p := range projects {
		live[p.Key] = true
	}
	var out []string
	for _, key := range hiddenKeys(hidden) {
		if !live[key] {
			out = append(out, key)
		}
	}
	return out
}
