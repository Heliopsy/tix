// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// TestWrapAtItsExactFit pins the boundaries the card and the detail body both
// depend on: the word that ends exactly on the edge, the word one cell too
// long, and the word longer than the whole line.
func TestWrapAtItsExactFit(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		width int
		want  []string
	}{
		{"exactly the width", "rotate keys", 11, []string{"rotate keys"}},
		{"one cell too narrow breaks on the space", "rotate keys", 10, []string{"rotate", "keys"}},
		{"one cell to spare", "rotate keys", 12, []string{"rotate keys"}},
		{"a word exactly the width", "rotate", 6, []string{"rotate"}},
		{"a word one cell too long is broken", "rotate", 5, []string{"rotat", "e"}},
		{"a word far longer than the line", "abcdefghij", 3, []string{"abc", "def", "ghi", "j"}},
		{"runs of spaces collapse", "a   b", 10, []string{"a b"}},
		{"an embedded newline starts a line", "a\nb", 10, []string{"a", "b"}},
		{"an empty string is one empty line", "", 10, []string{""}},
		{"only spaces is one empty line", "   ", 10, []string{""}},
		{"no room", "rotate", 0, nil},
		{"negative room", "rotate", -1, nil},
		{"multibyte counts runes", "ротация ключей", 7, []string{"ротация", "ключей"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Wrap(tc.in, tc.width)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("Wrap(%q, %d) = %q, want %q", tc.in, tc.width, got, tc.want)
			}
			for _, l := range got {
				if n := len([]rune(l)); n > tc.width {
					t.Fatalf("Wrap produced a %d cell line for width %d: %q", n, tc.width, l)
				}
			}
		})
	}
}

// TestWrapTitleAtItsLineLimit pins the cut. Only the last line shown carries an
// ellipsis, and the text it carries is what the remaining lines held, not the
// fragment the old renderer re-truncated.
func TestWrapTitleAtItsLineLimit(t *testing.T) {
	tests := []struct {
		name         string
		in           string
		width, lines int
		want         []string
	}{
		{"one line is enough", "rotate keys", 20, 2, []string{"rotate keys"}},
		{"exactly the limit", "rotate the deploy keys", 11, 2, []string{"rotate the", "deploy keys"}},
		{"one line over the limit", "rotate the deploy keys now", 11, 2, []string{"rotate the", "deploy key…"}},
		{"a limit of one truncates at once", "rotate the deploy keys", 11, 1, []string{"rotate the…"}},
		{"no lines", "rotate", 10, 0, nil},
		{"no width", "rotate", 0, 2, nil},
		{"an empty title is one empty line", "", 10, 2, []string{""}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := WrapTitle(tc.in, tc.width, tc.lines)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("WrapTitle(%q, %d, %d) = %q, want %q", tc.in, tc.width, tc.lines, got, tc.want)
			}
			if len(got) > tc.lines {
				t.Fatalf("WrapTitle returned %d lines for a limit of %d", len(got), tc.lines)
			}
		})
	}
}

// TestCardWindowKeepsTheSelectedCardWhole is the boundary the board's scrolling
// turns on: cards are not all the same height, so a window counted in rows
// either clips a card in half or leaves a card's worth of blank frame.
func TestCardWindowKeepsTheSelectedCardWhole(t *testing.T) {
	tests := []struct {
		name                    string
		heights                 []int
		offset, selected, lines int
		wantStart, wantCount    int
	}{
		{"everything fits", []int{3, 3, 3}, 0, 0, 9, 0, 3},
		{"the budget is exactly the list", []int{3, 4, 3}, 0, 2, 10, 0, 3},
		{"one line short of the last card", []int{3, 4, 3}, 0, 2, 9, 1, 2},
		{"the selection pulls the window down", []int{3, 3, 3, 3}, 0, 3, 6, 2, 2},
		{"the selection pulls the window up", []int{3, 3, 3, 3}, 2, 0, 6, 0, 2},
		{"a stale offset past the end is pulled back", []int{3, 3, 3}, 2, 0, 9, 0, 3},
		{"a card taller than the budget is still drawn", []int{9}, 0, 0, 3, 0, 1},
		{"no cards", nil, 0, 0, 9, 0, 0},
		{"no room", []int{3}, 0, 0, 0, 0, 0},
		{"negative room", []int{3}, 0, 0, -1, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start, count := CardWindow(tc.heights, tc.offset, tc.selected, tc.lines)
			if start != tc.wantStart || count != tc.wantCount {
				t.Fatalf("CardWindow = %d,%d want %d,%d", start, count, tc.wantStart, tc.wantCount)
			}
			if len(tc.heights) == 0 || tc.lines <= 0 {
				return
			}
			if tc.selected < start || tc.selected >= start+count {
				t.Fatalf("the selected card %d is outside the window %d..%d", tc.selected, start, start+count)
			}
			used := 0
			for _, h := range tc.heights[start : start+count] {
				used += h
			}
			if used > tc.lines && count > 1 {
				t.Fatalf("the window takes %d lines of a %d line budget", used, tc.lines)
			}
		})
	}
}

// TestDistributeWidthWeightsByWhatAColumnHasToShow pins the sharing, including
// the case where the floors alone fill the width and the case where they do
// not fit at all.
func TestDistributeWidthWeightsByWhatAColumnHasToShow(t *testing.T) {
	tests := []struct {
		name            string
		avail           int
		demands, floors []int
		want            []int
	}{
		{"a demand nobody can grow into keeps the floor", 40, []int{10, 10}, []int{20, 20}, []int{20, 20}},
		{"the busy column takes the spare", 60, []int{40, 16}, []int{20, 15}, []int{44, 16}},
		{"two busy columns share in proportion", 70, []int{40, 30}, []int{20, 20}, []int{40, 30}},
		{"an exact fit at the demands", 55, []int{40, 15}, []int{20, 15}, []int{40, 15}},
		{"the floors alone fill the width", 35, []int{40, 20}, []int{20, 15}, []int{20, 15}},
		{"the floors do not fit and it splits evenly", 30, []int{40, 40}, []int{20, 20}, []int{15, 15}},
		{"one column takes everything", 50, []int{40}, []int{20}, []int{50}},
		{"no columns", 50, nil, nil, nil},
		{"no width", 0, []int{40}, []int{20}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DistributeWidth(tc.avail, tc.demands, tc.floors)
			if len(got) != len(tc.want) {
				t.Fatalf("DistributeWidth = %v, want %v", got, tc.want)
			}
			sum := 0
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("DistributeWidth = %v, want %v", got, tc.want)
				}
				sum += got[i]
			}
			if len(got) > 0 && sum != tc.avail {
				t.Fatalf("DistributeWidth handed out %d cells of %d", sum, tc.avail)
			}
		})
	}
}

// TestColumnDemandAndFloorReadTheColumn holds the two figures the sharing is
// driven by against what a column actually carries.
func TestColumnDemandAndFloorReadTheColumn(t *testing.T) {
	overhead := CardBarWidth + BorderWidth + ColumnGutter
	long := "a title very much longer than the cap this function applies to one"
	tests := []struct {
		name       string
		heading    string
		titles     []string
		wantDemand int
		wantFloor  int
	}{
		{"an empty column asks for its heading", "Cancelled (0)", nil, 13 + overhead, 13 + BorderWidth + ColumnGutter},
		{"an empty column with a short heading floors at the minimum", "Done (0)", nil, 8 + overhead, MinEmptyColumnWidth},
		{"a busy column asks for its widest title", "Doing (2)", []string{"short", "a longer title"},
			14 + overhead, MinColumnWidth},
		{"a heading wider than every title wins", "A Very Long State Name (1)", []string{"x"},
			26 + overhead, MinColumnWidth},
		{"one long title cannot take the board", "Doing (1)", []string{long},
			MaxCardWidth + overhead, MinColumnWidth},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tasks := tasksWithTitles(tc.titles)
			if got := ColumnDemand(tc.heading, tasks); got != tc.wantDemand {
				t.Errorf("ColumnDemand = %d, want %d", got, tc.wantDemand)
			}
			if got := ColumnFloor(tc.heading, tasks); got != tc.wantFloor {
				t.Errorf("ColumnFloor = %d, want %d", got, tc.wantFloor)
			}
		})
	}
}

// tasksWithTitles is a column's worth of tasks carrying the given titles.
func tasksWithTitles(titles []string) []core.Task {
	out := make([]core.Task, 0, len(titles))
	for _, title := range titles {
		out = append(out, core.Task{Title: title})
	}
	return out
}
