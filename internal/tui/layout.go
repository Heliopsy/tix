// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import "fmt"

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
