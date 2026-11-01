// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
)

// prefsView is every per-browser preference that has more than a switch: the
// column picker of each listing, the project visibility choice, and the
// task view switch.
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

	// TaskView is the list, board or lanes switch.
	TaskView taskViewFormView

	// ListGroup is whether the list draws a heading per project or state.
	ListGroup listGroupFormView
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

// taskViewFormView is the task view switch. Current is what the reader
// has, which is the list whenever the cookie names no view this build draws,
// so the switch has one position per view and no unset one.
type taskViewFormView struct {
	CSRF    string
	Here    string
	Current string
	Choices []taskViewChoice
}

// themeToggleView is the sidebar's colour scheme switch: the scheme this
// browser reads under, and the one a press moves it to.
//
// It is the whole of the colour scheme preference. The settings screen names
// the current scheme and points at the switch instead of rendering a second
// control, for the reason prefsView above exists: one cookie behind two
// controls is two defaults, two validations and two readings of one value.
// The switch is the one that stayed, because the scheme is the preference a
// reader changes on whatever screen they are already on.
type themeToggleView struct {
	CSRF  string
	Here  string
	Theme string
}

// Label names the scheme in force.
func (t themeToggleView) Label() string { return ThemeLabel(t.Theme) }

// Next is the scheme one press moves to.
func (t themeToggleView) Next() string { return nextTheme(t.Theme) }

// NextLabel names the scheme one press moves to.
func (t themeToggleView) NextLabel() string { return ThemeLabel(t.Next()) }

// Preference is one per-browser display preference: the cookie it is kept
// in, the route its control submits to, the words the documentation table
// names it under, and whether its one control is the sidebar's rather than a
// form on the settings screen.
type Preference struct {
	Cookie  string
	Route   string
	Label   string
	Sidebar bool
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
//
// Sidebar marks the ones whose control lives in the sidebar instead, which is
// what keeps that guard honest: the sidebar renders inside the settings page
// too, so a test reading the whole document would pass on a preference that
// has no control on the screen at all.
var Preferences = []Preference{
	{Cookie: ThemeCookie, Route: RouteTheme, Label: "Colour scheme", Sidebar: true},
	{Cookie: KeySchemeCookie, Route: RouteKeyScheme, Label: "Keyboard shortcuts"},
	{Cookie: TimeFormatCookie, Route: RouteTimeFormat, Label: "Date format"},
	{Cookie: TimezoneCookie, Route: RouteTimezone, Label: "Timezone"},
	{Cookie: AdvancedCookie, Route: RouteAdvanced, Label: "Advanced screens"},
	{Cookie: DragMoveCookie, Route: RouteDragMove, Label: "Drag to move"},
	{Cookie: OpenAfterAddCookie, Route: RouteOpenAfterAdd, Label: "Open a new task"},
	{Cookie: HoverCookie, Route: RouteHover, Label: "Hover effects"},
	{Cookie: ColumnsCookie, Route: RouteColumns, Label: "Columns"},
	{Cookie: HiddenProjectsCookie, Route: RouteVisibility, Label: "Hidden projects"},
	{Cookie: TaskViewCookie, Route: RouteTaskView, Label: "Task view"},
	{Cookie: ListGroupCookie, Route: RouteListGroup, Label: "List groups"},
	{Cookie: TaskOpenCookie, Route: RouteTaskOpen, Label: "Task opens in"},
	{Cookie: TitleClickCookie, Route: RouteTitleClick, Label: "Clicking a title"},
	{Cookie: TintCookie, Route: RouteTint, Label: "Colour rows by"},
	{Cookie: LanguageCookie, Route: RouteLanguage, Label: "Language"},
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
		ListGroup: newListGroupForm(csrf, back, listGroupOf(r)),
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
