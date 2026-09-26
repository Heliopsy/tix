// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

// ViewAccess reports which of the terminal interface's views a reader may
// enter, keyed by the view name the capability registry binds operations to.
//
// It is supplied by the caller rather than worked out here. The authority
// belongs to the authorization policy, which only internal/service may ask, so
// a table in this package would be a second opinion about permission that
// nothing keeps in step with the first. The caller resolves it once per session
// from capability.TUIAccess, which derives it from the scopes the registry
// records and the service enforces, and a keystroke then costs no call.
//
// A nil ViewAccess offers nothing beyond the views that need no authority at
// all. A caller that forgets to supply one loses navigation, which is visible;
// the other default would hand a reader screens the service will refuse.
type ViewAccess map[string]bool

// localOnlyViews need no authority because they read nothing from the service:
// the display preferences, and the help overlay describing the keys.
var localOnlyViews = map[viewKind]bool{
	viewSettings: true,
	viewHelp:     true,
	// The project list is where every session starts and where going back ends
	// up, so there is nowhere to send a reader who is refused it. A reader who
	// cannot list projects is told so by the list's own empty state.
	viewProjects: true,
}

// canReach reports whether this session may enter a view. It is the one door:
// every entry goes through enterView, so a view added later is gated by
// existing and not by its author remembering to ask.
func (m Model) canReach(v viewKind) bool {
	if localOnlyViews[v] {
		return true
	}
	return m.access[viewName(v)]
}

// offersView is canReach as the predicate the help overlay filters with.
func (m Model) offersView() func(viewKind) bool { return m.canReach }
