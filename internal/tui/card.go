// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"

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

// MetaSegment is one run of a card's meta line drawn in a style of its own.
// The line is dim throughout except where a deadline has something urgent to
// say, and a segment is how that run keeps its own style through the
// truncation the whole line goes through.
type MetaSegment struct {
	Text string
	Due  core.DueState
}

// Card is one task as the board draws it: the title a reader scans for,
// then the identifiers beneath it in dim text.
type Card struct {
	// Title is the wrapped title, already fitted to the text column.
	Title []string
	// Meta is the line of identifiers under the title, already fitted to the
	// text column, in the runs it is styled by.
	Meta []MetaSegment
}

// MetaText is the meta line as plain text, which is what its width is.
func (c Card) MetaText() string { return segmentText(c.Meta) }

// segmentText joins segments back into the line they draw.
func segmentText(segs []MetaSegment) string {
	var b strings.Builder
	for _, s := range segs {
		b.WriteString(s.Text)
	}
	return b.String()
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
		Meta:  TruncateSegments(CardMetaSegments(t, now, mine), text),
	}
}

// DueOverdueMarker is what a card carries once its deadline has passed, and
// DueSoonMarker once the deadline is near enough to be news.
//
// A marker was removed from this line before for appearing on every card: it
// was drawn for any due date at all, and nearly every task in a real backlog
// has one. These are drawn only for core.DueStateOf's notable states, so a
// task with no deadline and a task due next quarter both draw nothing, and
// what is left on screen is the work that is actually running out of time.
const (
	DueOverdueMarker = "due!"
	DueSoonMarker    = "due"
)

// DueMarker is the marker a due state carries, empty where it carries none.
func DueMarker(state core.DueState) string {
	switch state {
	case core.DueOverdue:
		return DueOverdueMarker
	case core.DueSoon:
		return DueSoonMarker
	default:
		return ""
	}
}

// CardMetaSegments names the task under its own title: the reference, the
// priority, the deadline where it is pressing, then whichever markers are true
// of it. The deadline sits ahead of the markers because it is the one of them
// that changes by itself, so it is the one that has to survive a narrow
// column.
func CardMetaSegments(t core.Task, now time.Time, mine string) []MetaSegment {
	out := []MetaSegment{{Text: t.Ref + CardSeparator + "P" + priorityDigit(t.Priority)}}
	state := t.DueState(now)
	if marker := DueMarker(state); marker != "" {
		out = append(out, MetaSegment{Text: CardSeparator}, MetaSegment{Text: marker, Due: state})
	}
	if flags := CardFlags(t, now, mine); flags != "" {
		out = append(out, MetaSegment{Text: CardSeparator + flags})
	}
	return out
}

// TruncateSegments fits a segmented line into width, marking the cut the way
// Truncate does. The whole line is cut once and the result handed back to the
// segments it came from, so the line drawn is exactly the line Truncate would
// have produced and a run cut in half keeps its own style, ellipsis included.
func TruncateSegments(segs []MetaSegment, width int) []MetaSegment {
	rest := Truncate(segmentText(segs), width)
	out := make([]MetaSegment, 0, len(segs))
	for _, s := range segs {
		if rest == "" {
			break
		}
		if after, ok := strings.CutPrefix(rest, s.Text); ok {
			out, rest = append(out, s), after
			continue
		}
		out = append(out, MetaSegment{Text: rest, Due: s.Due})
		rest = ""
	}
	return out
}

// CardMeta is the meta line as plain text, before it is fitted to a column.
func CardMeta(t core.Task, now time.Time, mine string) string {
	return segmentText(CardMetaSegments(t, now, mine))
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
	want := lipgloss.Width(heading)
	for _, t := range tasks {
		if n := lipgloss.Width(t.Title); n > want {
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
	return max(MinEmptyColumnWidth, min(MinColumnWidth, lipgloss.Width(heading)+BorderWidth+ColumnGutter))
}
