// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/heliopsy/tix/internal/core"
)

// dueNow is the present every card below is drawn against.
var dueNow = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// dueTask is one task with a deadline at an offset from dueNow.
func dueTask(id string, offset time.Duration) core.Task {
	t := task(id, "todo", 1, core.PriorityNormal)
	at := dueNow.Add(offset)
	t.DueAt = &at
	return t
}

// dueModel is a board drawn in colour, since the marker's styling is half of
// what is under test and a monochrome model renders every style the same.
func dueModel(t *testing.T, tasks []core.Task) Model {
	t.Helper()
	m := New(Config{
		Access: fullAccess(), Actor: fullActor(), Color: boolPtr(true),
		Now: func() time.Time { return dueNow },
	})
	m.width, m.height = 120, 40
	m, _ = m.reduce(boardMsg{
		project:  core.Project{ID: "p1", Key: "infra", Name: "Infrastructure"},
		workflow: testWorkflow(),
		tasks:    tasks,
	})
	return m
}

// TestOnlyAPressingDeadlineReachesACard is the constraint the previous marker
// failed. It was drawn for any due date at all, so it appeared on nearly every
// card in a real backlog and was removed rather than fixed. A deadline next
// quarter and no deadline at all must draw the same thing: nothing.
func TestOnlyAPressingDeadlineReachesACard(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		task   core.Task
		marker string
	}{
		{"no deadline", task("a", "todo", 1, core.PriorityNormal), ""},
		{"next quarter", dueTask("b", 90*24*time.Hour), ""},
		{"a day past the window", dueTask("c", core.DueSoonWindow+24*time.Hour), ""},
		{"inside the window", dueTask("d", 48*time.Hour), DueSoonMarker},
		{"overdue", dueTask("e", -48*time.Hour), DueOverdueMarker},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			meta := CardOf(tc.task, 60, dueNow, "").MetaText()
			head := tc.task.Ref + CardSeparator + "P3"
			want := head
			if tc.marker != "" {
				want = head + CardSeparator + tc.marker
			}
			if meta != want {
				t.Fatalf("card meta = %q, want %q", meta, want)
			}
		})
	}
}

// TestTheDueMarkerIsDrawnByTheThemeNotByALiteral is the styling guard. It does
// not look for a colour: a card is compared against the string the theme's own
// Due style renders, so a marker written with a hard-wired escape, or with the
// dim style every other identifier uses, fails. The second half proves the
// comparison can tell those apart, since a Due style equal to Dim would make
// the first half pass while the marker read as ordinary text.
func TestTheDueMarkerIsDrawnByTheThemeNotByALiteral(t *testing.T) {
	t.Parallel()
	m := dueModel(t, []core.Task{dueTask("a", -48*time.Hour), dueTask("b", 48*time.Hour)})
	line := strings.Join(m.cardLines(dueTask("a", -48*time.Hour), core.CategoryTodo, 40, false), "\n")

	overdue := m.theme.Due(core.DueOverdue).Inherit(m.theme.Dim)
	if !strings.Contains(line, overdue.Render(DueOverdueMarker)) {
		t.Fatalf("the overdue marker is not drawn in the theme's overdue style:\n%q", line)
	}
	if strings.Contains(line, m.theme.Dim.Render(DueOverdueMarker)) {
		t.Fatalf("the overdue marker is drawn dim, like every other identifier:\n%q", line)
	}
	// The three styles have to differ for the assertions above to mean
	// anything, and they have to differ from each other so "soon" and "late"
	// are not one marker in two words.
	dim := m.theme.Dim.Render(DueOverdueMarker)
	soon := m.theme.Due(core.DueSoon).Inherit(m.theme.Dim).Render(DueOverdueMarker)
	late := overdue.Render(DueOverdueMarker)
	if dim == soon || dim == late || soon == late {
		t.Fatalf("the theme draws dim, soon and overdue alike: %q %q %q", dim, soon, late)
	}
	if m.theme.Due(core.DueLater).Render("x") != m.theme.Style().Render("x") {
		t.Fatal("a deadline still some way off carries a style of its own")
	}
}

// TestTheDueMarkerSurvivesAColourlessTerminal is the other half of "colour is
// never the only cue". Every shipped theme can be asked for no colour at all,
// and the marker has to keep its meaning there, which is why it is a word and
// not a hue.
func TestTheDueMarkerSurvivesAColourlessTerminal(t *testing.T) {
	t.Parallel()
	m := New(Config{
		Access: fullAccess(), Actor: fullActor(), Environ: []string{"NO_COLOR=1"},
		Now: func() time.Time { return dueNow },
	})
	m.width, m.height = 120, 40
	line := strings.Join(m.cardLines(dueTask("a", -48*time.Hour), core.CategoryTodo, 40, false), "\n")
	if strings.Contains(line, "\x1b[") {
		t.Fatalf("a colourless card carries an escape sequence: %q", line)
	}
	if !strings.Contains(line, DueOverdueMarker) {
		t.Fatalf("the overdue marker is gone without colour, so colour was the marker: %q", line)
	}
}

// TestTheBoardDrawsThreeDeadlinesApart is the reader's actual question: on one
// board, an overdue card, a card not yet due and an undated one have to be
// distinguishable, and only the first two by a marker.
func TestTheBoardDrawsThreeDeadlinesApart(t *testing.T) {
	t.Parallel()
	m := dueModel(t, []core.Task{
		dueTask("a", -48*time.Hour),
		dueTask("b", 48*time.Hour),
		task("c", "todo", 3, core.PriorityNormal),
	})
	lines := columnBlockLines(t, m, 0)
	var metas []string
	for _, l := range lines {
		if strings.Contains(l, "infra-") {
			metas = append(metas, strings.TrimSpace(l))
		}
	}
	if len(metas) != 3 {
		t.Fatalf("the column drew %d meta lines, want 3:\n%s", len(metas), strings.Join(lines, "\n"))
	}
	// The lines carry the column's own border and padding, so the marker is
	// looked for with the separator that introduces it rather than at the end
	// of the string.
	if !strings.Contains(metas[0], CardSeparator+DueOverdueMarker) {
		t.Errorf("the overdue card is %q, want it to carry %q", metas[0], DueOverdueMarker)
	}
	if !strings.Contains(metas[1], CardSeparator+DueSoonMarker) || strings.Contains(metas[1], DueOverdueMarker) {
		t.Errorf("the card due this week is %q, want it to carry %q and not %q",
			metas[1], DueSoonMarker, DueOverdueMarker)
	}
	if strings.Contains(metas[2], "due") {
		t.Errorf("the undated card carries a deadline marker: %q", metas[2])
	}
}

// TestTheDetailNamesTheDeadlineState covers the other place a date is drawn. A
// bare date left the reader comparing it with a calendar, which is the work the
// state exists to save them.
func TestTheDetailNamesTheDeadlineState(t *testing.T) {
	t.Parallel()
	m := dueModel(t, nil)
	for _, tc := range []struct {
		name   string
		offset time.Duration
		want   string
	}{
		{"overdue", -48 * time.Hour, "(overdue)"},
		{"this week", 48 * time.Hour, "(due soon)"},
		{"next quarter", 90 * 24 * time.Hour, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := stripANSI(m.dueText(dueTask("a", tc.offset), dueNow))
			if !strings.Contains(got, "due ") {
				t.Fatalf("the detail does not name the deadline at all: %q", got)
			}
			if tc.want == "" {
				if strings.Contains(got, "(") {
					t.Fatalf("a deadline next quarter is named as pressing: %q", got)
				}
				return
			}
			if !strings.Contains(got, tc.want) {
				t.Fatalf("the detail reads %q, want it to say %q", got, tc.want)
			}
		})
	}
	if got := m.dueText(task("a", "todo", 1, core.PriorityNormal), dueNow); got != "" {
		t.Fatalf("a task with no deadline drew %q", got)
	}
}

// TestACardCarryingADeadlineStillFitsItsColumn extends the cell-measuring
// guard to the marker. Anything added to a card costs cells, and a meta line
// wider than its column is re-wrapped by lipgloss inside the border, which is
// what pushed the frame past the terminal the last time this was got wrong.
func TestACardCarryingADeadlineStillFitsItsColumn(t *testing.T) {
	t.Parallel()
	tasks := []core.Task{dueTask("a", -48*time.Hour), dueTask("b", 48*time.Hour)}
	tasks[0].Ref, tasks[1].Ref = "インフラ-1", "infra-14829"
	tasks[0].Blocked, tasks[1].Blocked = true, true
	tasks[0].DependsOn = []string{"infra-2"}
	for _, width := range []int{ColumnFloor("todo", tasks), MinColumnWidth, MinEmptyColumnWidth, 24, 40, 80} {
		for _, task := range tasks {
			card := CardOf(task, width, dueNow, "")
			text := max(1, width-CardBarWidth)
			if n := lipgloss.Width(card.MetaText()); n > text {
				t.Fatalf("at column width %d the meta line %q draws %d cells, %d too many",
					width, card.MetaText(), n, n-text)
			}
			for _, line := range card.Title {
				if n := lipgloss.Width(line); n > text {
					t.Fatalf("at column width %d a title line draws %d cells", width, n)
				}
			}
		}
	}
}

// TestTruncateSegmentsCutsTheWholeLineOnce is the primitive the line above is
// fitted with. A segmented line cut segment by segment loses the ellipsis, and
// a cut run has to keep the style it was cut out of.
func TestTruncateSegmentsCutsTheWholeLineOnce(t *testing.T) {
	t.Parallel()
	segs := []MetaSegment{
		{Text: "infra-1 · P2"},
		{Text: CardSeparator},
		{Text: DueOverdueMarker, Due: core.DueOverdue},
	}
	whole := segmentText(segs)
	for width := 1; width <= lipgloss.Width(whole)+2; width++ {
		got := TruncateSegments(segs, width)
		if n := lipgloss.Width(segmentText(got)); n > width {
			t.Fatalf("TruncateSegments(..., %d) draws %d cells", width, n)
		}
		if want := Truncate(whole, width); segmentText(got) != want {
			t.Fatalf("TruncateSegments(..., %d) = %q, Truncate = %q", width, segmentText(got), want)
		}
	}
	// The run the cut lands in keeps its own state, so half a marker is still
	// drawn as a marker rather than reverting to dim mid-word.
	cut := TruncateSegments(segs, lipgloss.Width(whole)-1)
	last := cut[len(cut)-1]
	if last.Due != core.DueOverdue {
		t.Fatalf("the cut run lost its state: %+v", cut)
	}
}
