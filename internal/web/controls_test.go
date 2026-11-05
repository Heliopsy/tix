// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// controlSelectors are the controls and control-like marks whose label is one
// unit: buttons, links dressed as buttons, menu items, segments, tabs, badges,
// pills and chips, and the summaries drawn as buttons.
var controlSelectors = []string{
	"button", ".backlink a", ".pager-step", ".flowgo", ".ghostlink", ".segmented .segment",
	"nav.tabs a", ".badge", ".pill", ".role", ".state", ".evchip", ".chip", ".qchip",
	".statuspill", ".navbadge", "details.panel.addmore > summary", "td > details.panel > summary",
}

func readSheet(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("assets/app.css")
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// declared reports whether body sets property to value.
func declared(body, property, value string) bool {
	re := regexp.MustCompile(`(?:^|;)\s*` + regexp.QuoteMeta(property) + `\s*:\s*` + regexp.QuoteMeta(value) + `\s*(?:;|$)`)
	return re.MatchString(body)
}

// TestControlsKeepTheirLabelOnOneLine pins the repair of a Save button drawn
// as "Sav" over "e".
//
// td carries overflow-wrap: anywhere so a ULID cannot push a table wider than
// the page, and the button rule said nothing about wrapping, so a button in a
// cell inherited it and a squeezed column broke its label between letters.
// Every control states its own wrapping in one rule outside any media query.
func TestControlsKeepTheirLabelOnOneLine(t *testing.T) {
	t.Parallel()
	rules := cssRules(readSheet(t))
	for _, sel := range controlSelectors {
		found := false
		for _, r := range rules {
			if r.media != "" {
				continue
			}
			listed := false
			for _, one := range strings.Split(r.selector, ",") {
				listed = listed || strings.Join(strings.Fields(one), " ") == sel
			}
			if !listed {
				continue
			}
			if declared(r.body, "white-space", "nowrap") && declared(r.body, "overflow-wrap", "normal") &&
				declared(r.body, "word-break", "normal") {
				found = true
			}
		}
		if !found {
			t.Errorf("no rule gives %s white-space: nowrap, overflow-wrap: normal and word-break: normal", sel)
		}
	}
}

var wrappingControl = regexp.MustCompile(`(^|[^-\w])(button|summary)(\b|$)|\.(flowgo|ghostlink|segment|badge|pill|chip|qchip|statuspill|pager-step|evchip)(\b|$)`)

// TestNoLaterRuleLetsAControlBreakItsLabel fails on a rule that styles a
// control and undoes the one above: a white-space other than nowrap, or a
// break anywhere or between any two letters.
func TestNoLaterRuleLetsAControlBreakItsLabel(t *testing.T) {
	t.Parallel()
	for _, r := range cssRules(readSheet(t)) {
		styled := false
		for _, s := range subjects(r.selector) {
			if strings.Contains(s, "::") {
				continue
			}
			styled = styled || wrappingControl.MatchString(s)
		}
		if !styled {
			continue
		}
		for _, bad := range [][2]string{{"overflow-wrap", "anywhere"}, {"overflow-wrap", "break-word"},
			{"word-break", "break-all"}, {"word-break", "break-word"}, {"white-space", "normal"},
			{"white-space", "pre-wrap"}, {"white-space", "pre-line"}} {
			if declared(r.body, bad[0], bad[1]) {
				t.Errorf("%s sets %s: %s, which lets a control's label break", r.selector, bad[0], bad[1])
			}
		}
	}
}
