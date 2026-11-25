// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"strings"
	"testing"
)

// TestBlockedIsRedInTheBrowser holds the browser to the terminal, which draws
// blocked in red: every rule that colours the blocked state, the column
// heading, the chip, the badge and the status tint, uses the danger token.
func TestBlockedIsRedInTheBrowser(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")
	sheet := body(t, b.get("/assets/app.css"))
	for _, selector := range []string{
		".cat-blocked {", ".pill.status.blocked {", ".badge.blocked {", ".tint-status-blocked {",
	} {
		rule := declarations(t, sheet, selector)
		if !strings.Contains(rule, "var(--danger") || strings.Contains(rule, "var(--warn") {
			t.Errorf("%s is not drawn in the danger red: %s", selector, rule)
		}
	}
	page := b.page("/projects/infra?view=board")
	at := strings.Index(page, `data-state="blocked"`)
	if at < 0 {
		t.Fatal("the board has no blocked column")
	}
	head := page[at:]
	if end := strings.Index(head, "</h2>"); end >= 0 {
		head = head[:end]
	}
	if !strings.Contains(head, `<h2 class="cat-blocked">`) {
		t.Errorf("the blocked column heading carries no category colour: %s", head)
	}
}
