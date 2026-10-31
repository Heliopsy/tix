// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"strings"
	"testing"
)

// What assets/feel.js adds to the stylesheet moves things: a pending row
// fades, a star pops, a menu drops in, a toast leaves, a screen cross-fades.
// Each movement is declared only for a reader who has not asked for less
// motion, and each hover state only where hover effects are on.
func TestActingInPlaceMovesOnlyWhereMotionIsWelcome(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	resp := f.as("alice").get("/assets/app.css")
	wantStatus(t, resp, http.StatusOK)
	rules := cssRules(body(t, resp))

	moving := []string{".is-pending", "button.star svg", ".flowmenu:popover-open", ".flash.is-leaving"}
	for _, subject := range moving {
		found := false
		for _, rule := range rules {
			if !hasSelector(rule.selector, subject) {
				continue
			}
			found = found || strings.Contains(rule.body, "transition") || strings.Contains(rule.body, "animation")
			if (strings.Contains(rule.body, "transition") || strings.Contains(rule.body, "animation:")) &&
				!strings.Contains(rule.media, "prefers-reduced-motion: no-preference") {
				t.Errorf("%s moves outside the reduced-motion gate (%q): %s", subject, rule.media, rule.body)
			}
		}
		if !found {
			t.Errorf("%s has no movement at all, so this guard reads nothing", subject)
		}
	}

	stilled := false
	for _, rule := range rules {
		if strings.Contains(rule.media, "prefers-reduced-motion: reduce") &&
			strings.Contains(rule.selector, "::view-transition-old") && strings.Contains(rule.body, "animation: none") {
			stilled = true
		}
	}
	if !stilled {
		t.Error("a reader who asked for less motion still gets the cross-fade between screens")
	}

	for _, subject := range []string{".flashundo:hover", ".kbdhint:hover"} {
		for _, rule := range rules {
			for _, sel := range strings.Split(rule.selector, ",") {
				if strings.Contains(sel, subject) && !strings.Contains(sel, `:root:not([data-hover="off"])`) {
					t.Errorf("%q is a hover effect not gated on the preference", strings.TrimSpace(sel))
				}
			}
		}
	}
}

// hasSelector reports whether a selector list names subject as one of its
// selectors, after any gate in front of it.
func hasSelector(list, subject string) bool {
	for _, sel := range strings.Split(list, ",") {
		if strings.HasSuffix(strings.TrimSpace(sel), subject) {
			return true
		}
	}
	return false
}
