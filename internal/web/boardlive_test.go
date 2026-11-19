// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"strings"
	"testing"
)

// TestABoardCardHoldsNoSelectToFreezeTheRefresh pins what is left of the fix
// for a board that never live-updated.
//
// A <select> whose options name none as selected is still presented with the
// first one selected, because the browser picks it. The DOM then reports
// option.selected true against option.defaultSelected false, which is what a
// reader's own choice looks like, so tix.fieldDirty called every card dirty,
// regionBusy called #project-board permanently mid-edit, and live.js deferred
// the refresh on every pass. The card's Move control was that select. It is
// now the Move section of the card's three dots menu, a button per route, so
// a card has no select at all and nothing in it can read as mid-edit.
func TestABoardCardHoldsNoSelectToFreezeTheRefresh(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "a card that moves")

	// The page a reader lands on and the fragment a refresh swaps into the
	// live region.
	for _, source := range []struct {
		name string
		path string
	}{
		{"board", "/projects/infra"},
		{"live fragment", "/projects/infra/live"},
	} {
		t.Run(source.name, func(t *testing.T) {
			page := b.page(source.path)
			card := cardBlock(t, page, ref)
			if strings.Contains(card, "<select") {
				t.Errorf("the card holds a select, which a live refresh reads as mid-edit:\n%s", card)
			}
			if len(moveOptions(t, page, ref)) == 0 {
				t.Errorf("the card's menu offers no move:\n%s", card)
			}
		})
	}
}
