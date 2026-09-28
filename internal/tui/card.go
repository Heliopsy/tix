// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// The bar drawn down a card's left edge. It carries the card's state as a
// shape rather than as a colour, so a selected card is still the selected card
// on a terminal getting no escapes at all: NO_COLOR, TIX_NO_COLOR, a dumb
// terminal and a colourless theme all keep the heavy bar. Both glyphs occupy
// one cell, so the text beside them never shifts.
const (
	CardBarSelected = "┃"
	CardBarPlain    = "│"
)

// CardBar is the glyph a card's left edge carries.
func CardBar(selected bool) string {
	if selected {
		return CardBarSelected
	}
	return CardBarPlain
}

// CardSeparator joins the identifiers on a card's meta line. The priority used
// to be glued to the due-date marker as "P2*", which reads as one token that
// nothing in the interface explained.
const CardSeparator = " · "

// Card is one task as the board draws it: the title a reader scans for,
// then the identifiers beneath it in dim text.
type Card struct {
	// Title is the wrapped title, already fitted to the text column.
	Title []string
	// Meta is the dim line of identifiers under the title.
	Meta string
}

// Lines is how many lines the card occupies, including its meta line.
func (c Card) Lines() int { return len(c.Title) + 1 }

// CardOf renders one task into the lines a column draws for it. width is the
// whole card, bar included, so the text column is width less the bar.
func CardOf(t core.Task, width int, now time.Time, mine string) Card {
	text := max(1, width-CardBarWidth)
	title := t.Title
	if strings.TrimSpace(title) == "" {
		title = t.Ref
	}
	return Card{
		Title: WrapTitle(title, text, CardTitleLines),
		Meta:  Truncate(CardMeta(t, now, mine), text),
	}
}

// CardMeta names the task under its own title: the reference, the priority,
// then whichever markers are true of it.
func CardMeta(t core.Task, now time.Time, mine string) string {
	parts := []string{t.Ref, "P" + priorityDigit(t.Priority)}
	if flags := CardFlags(t, now, mine); flags != "" {
		parts = append(parts, flags)
	}
	return strings.Join(parts, CardSeparator)
}

// CardHeights is what each task in a column costs in lines, which is what the
// column's window is counted in.
func CardHeights(tasks []core.Task, width int, now time.Time, mine string) []int {
	out := make([]int, len(tasks))
	for i, t := range tasks {
		out[i] = CardOf(t, width, now, mine).Lines() + CardGap
	}
	return out
}

// ColumnDemand is how wide a column would like to be: enough for its heading,
// and enough for its widest title, capped so one long title cannot take the
// whole board. A column with nothing in it asks only for its heading.
func ColumnDemand(heading string, tasks []core.Task) int {
	want := len([]rune(heading))
	for _, t := range tasks {
		if n := len([]rune(t.Title)); n > want {
			want = n
		}
	}
	return min(want, MaxCardWidth) + CardBarWidth + BorderWidth + ColumnGutter
}

// ColumnFloor is the narrowest a column may be drawn. A column holding work
// keeps the board's minimum; an empty one may shrink, because every cell it
// gives up goes to a column that has something to show.
func ColumnFloor(heading string, tasks []core.Task) int {
	if len(tasks) > 0 {
		return MinColumnWidth
	}
	return max(MinEmptyColumnWidth, min(MinColumnWidth, len([]rune(heading))+BorderWidth+ColumnGutter))
}
