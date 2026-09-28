// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"fmt"
	"strings"
)

// LayoutMode names how much of the board fits on the terminal.
type LayoutMode int

// Layout modes, in descending order of available room.
const (
	LayoutBoard LayoutMode = iota
	LayoutSingleColumn
	LayoutTooSmall
)

// Layout sizes. A column is drawn inside a box, so BorderWidth of its width
// and BorderHeight of its height are spent on the frame rather than content.
const (
	MinColumnWidth = 20
	MinWidth       = 24
	MinHeight      = 8
	ChromeHeight   = 5
	BorderWidth    = 2
	BorderHeight   = 2
)

// Layout is the geometry one frame is drawn with.
type Layout struct {
	Mode           LayoutMode
	Width          int
	Height         int
	ColumnWidth    int
	VisibleColumns int
	BodyHeight     int
}

// InnerWidth is the room inside a column's border.
func (l Layout) InnerWidth() int { return max(1, l.ColumnWidth-BorderWidth) }

// CardRows is how many cards fit inside a column's border, leaving a line for
// the column's own heading.
func (l Layout) CardRows() int { return max(1, l.BodyHeight-BorderHeight-1) }

// LayoutFor decides the geometry for a terminal size and a column count.
func LayoutFor(width, height, columns int) Layout {
	l := Layout{Width: width, Height: height, BodyHeight: bodyHeight(height)}
	if width < MinWidth || height < MinHeight {
		l.Mode = LayoutTooSmall
		return l
	}
	if columns < 1 {
		columns = 1
	}
	fit := (width + 1) / (MinColumnWidth + 1)
	if fit < 2 {
		l.Mode = LayoutSingleColumn
		l.VisibleColumns = 1
		l.ColumnWidth = width
		return l
	}
	if fit > columns {
		fit = columns
	}
	l.Mode = LayoutBoard
	l.VisibleColumns = fit
	l.ColumnWidth = (width - (fit - 1)) / fit
	return l
}

// bodyHeight is the room left for content after the frame's chrome.
func bodyHeight(height int) int {
	if height <= ChromeHeight {
		return 1
	}
	return height - ChromeHeight
}

// VisibleRange returns the window of columns that keeps selected in view.
func VisibleRange(total, visible, selected int) (int, int) {
	if total <= 0 || visible <= 0 {
		return 0, 0
	}
	if visible >= total {
		return 0, total
	}
	start := selected - visible/2
	if start < 0 {
		start = 0
	}
	if start+visible > total {
		start = total - visible
	}
	return start, start + visible
}

// WindowLines returns the slice of lines visible at an offset.
func WindowLines(lines []string, offset, height int) []string {
	if height <= 0 || len(lines) == 0 {
		return nil
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(lines)-1 {
		offset = len(lines) - 1
	}
	end := offset + height
	if end > len(lines) {
		end = len(lines)
	}
	return lines[offset:end]
}

// ScrollOffset keeps a selected row inside a window of the given height.
func ScrollOffset(offset, selected, height int) int {
	if height <= 0 {
		return 0
	}
	if selected < offset {
		return selected
	}
	if selected >= offset+height {
		return selected - height + 1
	}
	return offset
}

// Truncate shortens s to width, marking the cut with an ellipsis.
func Truncate(s string, width int) string {
	r := []rune(s)
	if width <= 0 {
		return ""
	}
	if len(r) <= width {
		return s
	}
	if width == 1 {
		return string(r[:1])
	}
	return string(r[:width-1]) + "…"
}

// ScrollWindow keeps a selected row inside a window of the given height and,
// unlike ScrollOffset, never leaves the window hanging past the end of a list
// that has shrunk. It is what every scrolling view in the interface uses.
func ScrollWindow(offset, selected, height, total int) int {
	if height <= 0 || total <= 0 {
		return 0
	}
	if total <= height {
		return 0
	}
	offset = ScrollOffset(offset, clamp(selected, 0, total-1), height)
	return clamp(offset, 0, total-height)
}

// VisibleRows is how many rows a scrolling list may draw into a body of the
// given height. A list that does not fit gives up one row to the hint that
// says so, rather than drawing over its own last row.
func VisibleRows(height, total int) int {
	if total > height {
		return max(1, height-1)
	}
	return max(1, height)
}

// MoreAbove reports whether a scrolled window hides rows before it.
func MoreAbove(offset int) bool { return offset > 0 }

// MoreBelow reports whether a scrolled window hides rows after it.
func MoreBelow(offset, height, total int) bool {
	return height > 0 && offset+height < total
}

// ScrollHint states that a window hides rows, so a list that runs off the
// screen never looks like the whole list. It is empty when nothing is hidden.
func ScrollHint(offset, height, total int) string {
	above, below := offset, 0
	if MoreBelow(offset, height, total) {
		below = total - offset - height
	}
	switch {
	case MoreAbove(offset) && below > 0:
		return fmt.Sprintf("↑ %d more   ↓ %d more", above, below)
	case MoreAbove(offset):
		return fmt.Sprintf("↑ %d more", above)
	case below > 0:
		return fmt.Sprintf("↓ %d more", below)
	default:
		return ""
	}
}

// Card geometry. A card leads with its title, which may run to CardTitleLines
// lines before it is cut, and closes with one dim line of identifiers. The
// bar down its left edge takes CardBarWidth cells, and CardGap separates one
// card from the next.
const (
	CardTitleLines = 2
	CardBarWidth   = 2
	CardGap        = 1
	ColumnGutter   = 1
)

// MinEmptyColumnWidth is the floor a column with nothing in it is allowed to
// shrink to. It is narrower than MinColumnWidth because an empty column has
// only its heading and the word "empty" to carry, and every cell it does not
// take goes to a column that has work in it.
const MinEmptyColumnWidth = 15

// MaxCardWidth caps how much width one column may ask for on the strength of a
// long title. Past this a title wraps rather than taking the board with it.
const MaxCardWidth = 38

// Wrap breaks s into lines of at most width cells, preferring a space. A word
// longer than the line is broken at the line's edge rather than overflowing,
// because a line wider than its column corrupts every column beside it.
func Wrap(s string, width int) []string {
	if width <= 0 {
		return nil
	}
	var out []string
	for _, para := range strings.Split(s, "\n") {
		out = append(out, wrapLine(para, width)...)
	}
	return out
}

// wrapLine wraps one paragraph, which never contains a newline.
func wrapLine(s string, width int) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var out []string
	line := ""
	for _, w := range words {
		switch {
		case line == "":
			line = w
		case len([]rune(line))+1+len([]rune(w)) <= width:
			line += " " + w
		default:
			out = append(out, line)
			line = w
		}
		for len([]rune(line)) > width {
			r := []rune(line)
			out = append(out, string(r[:width]))
			line = string(r[width:])
		}
	}
	return append(out, line)
}

// WrapTitle wraps a title into at most lines lines, truncating only the last
// one shown. The defect it exists for is a title that wrapped into an
// unindented, re-truncated fragment: the continuation is a whole line of the
// same width, and nothing but the final line ever carries an ellipsis.
func WrapTitle(s string, width, lines int) []string {
	if width <= 0 || lines <= 0 {
		return nil
	}
	wrapped := Wrap(s, width)
	if len(wrapped) == 0 {
		return []string{""}
	}
	if len(wrapped) <= lines {
		return wrapped
	}
	out := wrapped[:lines]
	rest := strings.Join(wrapped[lines-1:], " ")
	out[lines-1] = Truncate(rest, width)
	return out
}

// CardWindow returns the first card visible in a column and how many fit,
// keeping the selected card wholly on screen. Cards are not all the same
// height, so a window counted in rows would either clip a card in half or
// leave a card's worth of blank frame below the last one.
func CardWindow(heights []int, offset, selected, lines int) (int, int) {
	if len(heights) == 0 || lines <= 0 {
		return 0, 0
	}
	start := clamp(offset, 0, len(heights)-1)
	selected = clamp(selected, 0, len(heights)-1)
	if selected < start {
		start = selected
	}
	for start < selected && selected >= start+cardsFitting(heights, start, lines) {
		start++
	}
	for start > 0 && start-1+cardsFitting(heights, start-1, lines) >= len(heights) {
		start--
	}
	return start, cardsFitting(heights, start, lines)
}

// cardsFitting counts the cards from start that fit in a line budget, always
// at least one so a card taller than the column is clipped rather than hidden.
func cardsFitting(heights []int, start, lines int) int {
	used, n := 0, 0
	for i := start; i < len(heights); i++ {
		if used+heights[i] > lines && n > 0 {
			break
		}
		used += heights[i]
		n++
	}
	return max(1, n)
}

// DistributeWidth shares a width among columns by what each has to show.
//
// An equal split spends the same cells on a column holding one word as on the
// column holding the work, so the busy columns wrap every title while the idle
// ones hold nothing. Each column is first given the smaller of its demand and
// its floor, and whatever is left over is shared in proportion to the room
// each still wants, so a column that asked for nothing more keeps its floor.
func DistributeWidth(avail int, demands, floors []int) []int {
	n := len(demands)
	if n == 0 || avail <= 0 {
		return nil
	}
	out := make([]int, n)
	total := 0
	for i := range out {
		out[i] = max(1, min(demands[i], floors[i]))
		total += out[i]
	}
	if total > avail {
		return evenWidths(avail, n)
	}
	spare := avail - total
	want := make([]int, n)
	wanted := 0
	var growable []int
	for i := range want {
		want[i] = max(0, demands[i]-out[i])
		wanted += want[i]
		if want[i] > 0 {
			growable = append(growable, i)
		}
	}
	for i := 0; i < n && spare > 0 && wanted > 0; i++ {
		out[i] += want[i] * spare / wanted
	}
	// Whatever integer division left over goes to the columns that asked for
	// more room, not to the ones already holding everything they have.
	if len(growable) == 0 {
		return spreadRemainder(out, avail, indices(n))
	}
	return spreadRemainder(out, avail, growable)
}

// indices is every position of a slice of n, for the case where no column
// asked for more room than its floor and the leftover has to go somewhere.
func indices(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

// evenWidths is the fallback when the floors alone do not fit: every column
// gets the same, which is at least drawable.
func evenWidths(avail, n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = max(1, avail/n)
	}
	return spreadRemainder(out, avail, indices(n))
}

// spreadRemainder hands the cells left over by integer division to the columns
// named by among, so the board always occupies the width it was given exactly.
func spreadRemainder(widths []int, avail int, among []int) []int {
	used := 0
	for _, w := range widths {
		used += w
	}
	for i := 0; used < avail; i = (i + 1) % len(among) {
		widths[among[i]]++
		used++
	}
	for i := 0; used > avail; i = (i + 1) % len(widths) {
		if widths[i] > 1 {
			widths[i]--
			used--
		}
	}
	return widths
}
