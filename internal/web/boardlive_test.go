// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"strings"
	"testing"
)

// TestTheMoveSelectDeclaresItsOwnDefault pins the markup half of the fix for a
// board that never live-updated.
//
// A <select> whose options name none as selected is still presented with the
// first one selected, because the browser picks it. The DOM then reports
// option.selected true against option.defaultSelected false, which is what a
// reader's own choice looks like, so tix.fieldDirty called every card dirty,
// regionBusy called #project-board permanently mid-edit, and live.js deferred
// the refresh on every pass. The board is the only screen with a select inside
// a live region, which is why it was the only one that froze.
//
// The markup declaring its own default is what makes the browser's pick
// indistinguishable from nothing having happened.
func TestTheMoveSelectDeclaresItsOwnDefault(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "a card that moves")

	// The page a reader lands on and the fragment a refresh swaps into the
	// live region. A default declared on one and not the other freezes the
	// board from the first update rather than from the first paint.
	for _, source := range []struct {
		name string
		path string
	}{
		{"board", "/projects/infra"},
		{"live fragment", "/projects/infra/live"},
	} {
		t.Run(source.name, func(t *testing.T) {
			sel := moveSelect(t, b.page(source.path), ref)
			options := strings.Split(sel, "<option ")[1:]
			if len(options) == 0 {
				t.Fatalf("the Move control offers no route:\n%s", sel)
			}
			var selected []int
			for i, opt := range options {
				attrs, _, ok := strings.Cut(opt, ">")
				if !ok {
					t.Fatalf("option %d of the Move control is not a tag:\n%s", i, sel)
				}
				if strings.Contains(attrs, "selected") {
					selected = append(selected, i)
				}
			}
			if len(selected) == 0 {
				t.Fatalf("no Move option is selected, so the browser picks the first "+
					"one and the DOM reports a choice nobody made; the board then "+
					"reads as permanently mid-edit and never refreshes:\n%s", sel)
			}
			if len(selected) > 1 {
				t.Fatalf("%d Move options are selected at once:\n%s", len(selected), sel)
			}
			if selected[0] != 0 {
				t.Errorf("the Move control opens on option %d rather than the first "+
					"route, so it reads as a reader's choice:\n%s", selected[0], sel)
			}
		})
	}
}

// moveSelect returns one card's Move control and nothing else. The board also
// carries the project's workflow picker and its own filter selects, and a test
// about the card's menu that reads the page reads those instead.
func moveSelect(t *testing.T, page, ref string) string {
	t.Helper()
	sel := between(t, page, `<select id="to-`+ref+`"`, "</select>")
	if sel == "" {
		t.Fatalf("the board renders no Move control for %s", ref)
	}
	return sel
}
