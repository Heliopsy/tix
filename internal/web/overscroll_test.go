// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"strings"
	"testing"
)

// TestScrollingBoxesHandTheWheelBackToThePage pins the fix for a board, lanes
// or task list that swallowed the mouse wheel.
//
// Columns, lanes and the list scroll inside the page, and each carried
// overscroll-behavior: contain. With that, a wheel turned over one of them
// never reached the page, even when the box had nothing left to scroll or
// nothing to scroll at all, so the page could only be scrolled with the
// pointer somewhere else. Only a modal may keep the wheel to itself.
func TestScrollingBoxesHandTheWheelBackToThePage(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sheet := body(t, f.as("").get("/assets/app.css"))

	allowed := map[string]bool{".taskdlg-body": true}
	const rule = "overscroll-behavior: contain;"
	rest := sheet
	for {
		i := strings.Index(rest, rule)
		if i < 0 {
			break
		}
		open := strings.LastIndex(rest[:i], "{")
		start := strings.LastIndexAny(rest[:open], "}\n")
		selector := strings.TrimSpace(rest[start+1 : open])
		if !allowed[selector] {
			t.Errorf("%s keeps the wheel from the page: overscroll-behavior: contain is for modals only", selector)
		}
		rest = rest[i+len(rule):]
	}
	for _, selector := range []string{".column {", ".lane {", "ul.tasklist {"} {
		if got := declarations(t, sheet, selector); !strings.Contains(got, "overflow-y: auto") {
			t.Errorf("%s no longer scrolls its own content, so this test no longer guards it:\n%s", selector, got)
		}
	}
}
