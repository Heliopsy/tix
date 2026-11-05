// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"strings"
	"testing"
)

const touchOnly = "@media (hover: none) and (pointer: coarse)"

// ruleIn is the bodies of the rules styling exactly selector inside media
// ("" for none), so an assertion reads those rules rather than the sheet. A
// selector is matched whole or as one entry of a list.
func ruleIn(t *testing.T, sheet, media, selector string) string {
	t.Helper()
	var bodies []string
	for _, r := range cssRules(sheet) {
		if r.media != media {
			continue
		}
		if strings.TrimSpace(r.selector) == selector {
			bodies = append(bodies, r.body)
			continue
		}
		if strings.Contains(r.selector, "(") {
			continue
		}
		for _, one := range strings.Split(r.selector, ",") {
			if strings.TrimSpace(one) == selector {
				bodies = append(bodies, r.body)
			}
		}
	}
	if len(bodies) == 0 {
		t.Fatalf("the stylesheet has no %q rule under %q", selector, media)
	}
	return strings.Join(bodies, "\n")
}

// Drag to move is not offered where the main input is a finger: the
// setting's row is marked for the touch-only rule, the rule hides it, and a
// card is not lifted by a long press there. Every card keeps its Move menu.
func TestTouchDevicesAreNotOfferedDragToMove(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	sheet := body(t, b.get("/assets/app.css"))

	row := prefForm(t, contentOf(t, b.page("/settings")), "/dragmove")
	if !strings.HasPrefix(row, `<form class="setrow needs-drag"`) {
		t.Errorf("the Drag to move row is not marked as needing a dragging pointer: %s", row)
	}
	if body := ruleIn(t, sheet, touchOnly, ".needs-drag"); !strings.Contains(body, "display: none") {
		t.Errorf("the touch-only rule does not hide .needs-drag: %s", body)
	}
	card := ruleIn(t, sheet, touchOnly, `.card[draggable="true"]`)
	if !strings.Contains(card, "-webkit-user-drag: none") {
		t.Errorf("a card can still be lifted on a touch-only device: %s", card)
	}

	b.createTask("infra", "movable")
	board := b.page("/projects/infra")
	if !strings.Contains(board, `draggable="true"`) || !strings.Contains(board, `<details class="card-move">`) {
		t.Error("the board lost its cards' Move control or its drag on a desktop")
	}
}

// A toolbar's (i) note opens in the flow on a narrow or touch-only screen,
// as a heading's does, and keeps floating on a wide screen with a mouse.
func TestToolbarNotesFlowOnPhones(t *testing.T) {
	t.Parallel()
	sheet := body(t, newFixture(t).as("alice").get("/assets/app.css"))
	const narrow = "@media (max-width: 899px), (hover: none) and (pointer: coarse)"
	const bars = ":is(.viewswitch, .pagehead-tools, .lanesbar)"
	note := ruleIn(t, sheet, narrow, bars+" > .tip > p")
	if !strings.Contains(note, "position: static") || !strings.Contains(note, "flex: 1 0 100%") {
		t.Errorf("a toolbar's note still floats on a phone: %s", note)
	}
	if tip := ruleIn(t, sheet, narrow, bars+" > .tip"); !strings.Contains(tip, "display: contents") {
		t.Errorf("a toolbar's tip keeps its box on a phone: %s", tip)
	}
	if wide := ruleIn(t, sheet, "", ".lanesbar .tip > p"); !strings.Contains(wide, "right: 0") {
		t.Errorf("the lanes bar's note opens off the right edge on a wide screen: %s", wide)
	}
}

// The (i) beside a heading stands on the heading's baseline. Centred, it
// sat on the middle of the heading's margin box, below the text.
func TestAHeadingTipStandsOnItsLine(t *testing.T) {
	t.Parallel()
	sheet := body(t, newFixture(t).as("alice").get("/assets/app.css"))
	if head := ruleIn(t, sheet, "", ".headtip"); !strings.Contains(head, "align-items: baseline") {
		t.Errorf("the heading tip is not aligned to the heading's baseline: %s", head)
	}
}
