package tui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// lineWidth counts the cells a rendered line occupies, ignoring its styling.
func lineWidth(line string) int {
	return len([]rune(stripANSI(line)))
}

// detailModel opens a task with something in every section, at one size.
func detailModel(t *testing.T, width, height int) Model {
	t.Helper()
	m := boardModel(t)
	m.width, m.height = width, height
	task, _ := TaskAt(m.columns, m.sel)
	task.Body = "the body of the task"
	task.Tags = []string{"urgent"}
	m, _ = m.reduce(detailMsg{
		task:     task,
		subtasks: []core.Task{{Ref: "infra-9", Status: "todo", Title: "a subtask"}},
		deps:     []core.Dependency{{DependsOn: "infra-4"}},
		comments: []core.Comment{{Body: "a comment", CreatedAt: time.Unix(0, 0).UTC()}},
	})
	return m
}

// TestTheDetailPaneNeverEatsTheFirstColumn guards a defect where every label
// rendered one character short — "ody: none" for "body: none" — which made the
// pane look corrupted at some widths and fine at others. The grouped fields
// ("status:", "priority:" and "claim:" now share one line, "people:" and
// "time:" group what used to be separate "assignee:"/"creator:" and
// "due:"/"created:"/"updated:" lines) so the check is no longer "this label
// starts its own line", but "this label's first character is never dropped
// wherever it appears".
func TestTheDetailPaneNeverEatsTheFirstColumn(t *testing.T) {
	labels := []string{"status:", "priority:", "claim:", "people:", "time:", "body:"}
	for _, width := range []int{60, 80, 100, 120, 200} {
		t.Run(fmt.Sprintf("width %d", width), func(t *testing.T) {
			frame := detailModel(t, width, 40).View()
			for _, label := range labels {
				if !strings.Contains(frame, label) {
					t.Errorf("width %d: %q is missing from the detail pane:\n%s", width, label, frame)
					continue
				}
				// Every occurrence of the label minus its first rune must be
				// part of an occurrence of the full label: stripping the full
				// label out and still finding the truncated form is the
				// defect this test exists for.
				truncated, full := label[1:], label
				if strings.Contains(strings.ReplaceAll(frame, full, ""), truncated) {
					t.Fatalf("width %d: %q lost its first character somewhere in the pane:\n%s", width, label, frame)
				}
			}
		})
	}
}

// TestSectionHeadingsSurviveEveryWidth checks the sections below the fields,
// which are the rows the defect was reported against.
func TestSectionHeadingsSurviveEveryWidth(t *testing.T) {
	for _, width := range []int{60, 80, 120} {
		m := detailModel(t, width, 40)
		m.detailOff = 6
		frame := m.View()
		for _, heading := range []string{"custom fields", "subtasks", "dependencies", "comments", "artifacts"} {
			if strings.Contains(frame, heading[1:]+":") && !strings.Contains(frame, heading+":") {
				t.Fatalf("width %d: %q lost its first character", width, heading)
			}
		}
	}
}

// TestEveryViewRendersItsFullWidthWithoutOverflowing checks that no frame line
// is wider than the terminal, which would wrap and corrupt the display.
func TestEveryViewRendersItsFullWidthWithoutOverflowing(t *testing.T) {
	for _, width := range []int{40, 60, 80, 100, 130, 200} {
		for _, build := range []func(*testing.T, int, int) Model{
			detailModel,
			func(t *testing.T, w, h int) Model { m := boardModel(t); m.width, m.height = w, h; return m },
			func(t *testing.T, w, h int) Model { return projectListModel(t, 13, w, h) },
		} {
			m := build(t, width, 24)
			for i, line := range strings.Split(m.View(), "\n") {
				if got := lineWidth(line); got > width {
					t.Fatalf("width %d: line %d is %d cells wide: %q", width, i, got, line)
				}
			}
		}
	}
}

// TestAnErrorLineKeepsItsFirstCharacter covers the status bar, which was
// reported as "rror: cannot release...".
func TestAnErrorLineKeepsItsFirstCharacter(t *testing.T) {
	for _, width := range []int{60, 80, 120} {
		m := detailModel(t, width, 40)
		m.err = "cannot release: this session does not hold a lease on infra-3"
		frame := m.View()
		if !strings.Contains(frame, "cannot release") {
			t.Fatalf("width %d: the error is missing entirely:\n%s", width, frame)
		}
		if strings.Contains(frame, "annot release") && !strings.Contains(frame, "cannot release") {
			t.Fatalf("width %d: the error lost its first character", width)
		}
	}
}

// stripANSI removes select graphic rendition sequences so a rendered line can
// be measured in the cells it actually occupies.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// TestColumnHeadingsAreNotNumbered pins the reported defect: a board too wide
// for the terminal used to stamp "[2/5]" onto every visible column heading.
// Which slice is on screen is said once, in the status bar, so the headings
// stay readable.
func TestColumnHeadingsAreNotNumbered(t *testing.T) {
	for _, width := range []int{40, 60, 80, 200} {
		m := boardModel(t)
		m.width, m.height = width, 24
		frame := m.View()
		if numbered := regexp.MustCompile(`\[\d+/\d+\]`); numbered.MatchString(frame) {
			t.Fatalf("width %d: a column heading is still numbered:\n%s", width, frame)
		}
	}
}

// TestANarrowBoardSaysWhichColumnsAreOnScreen is the other half: dropping the
// numbering must not drop the signal that the board continues off screen.
func TestANarrowBoardSaysWhichColumnsAreOnScreen(t *testing.T) {
	m := boardModel(t)
	m.width, m.height = 40, 24
	visible := LayoutFor(m.width, m.height, len(m.columns)).VisibleColumns
	if visible >= len(m.columns) {
		t.Skipf("board of %d columns fits in %d cells, nothing is hidden", len(m.columns), m.width)
	}
	if frame := m.View(); !strings.Contains(frame, "showing ") {
		t.Fatalf("a board showing %d of %d columns does not say so:\n%s", visible, len(m.columns), frame)
	}
	wide := boardModel(t)
	wide.width, wide.height = 200, 24
	if frame := wide.View(); strings.Contains(frame, "showing ") {
		t.Fatalf("a board that fits should not narrate its window:\n%s", frame)
	}
}

// TestClaimSummaryIsAlwaysRelative pins the deliberate choice that a lease's
// freshness is judged by how much is left, not by the absolute clock face it
// started at, whatever the configured output style would otherwise render.
// leaseTimeStyle's relative rendering is anchored to the real wall clock, not
// to the "now" a caller passes in (that is output.TimeStyle's own contract,
// shared with the rest of the renderer), so the fixture uses a real, nearby
// deadline with enough margin that the minute the test runs in cannot round
// the expectation away.
func TestClaimSummaryIsAlwaysRelative(t *testing.T) {
	now := time.Now()
	unclaimed := core.Task{}
	if got := claimSummary(unclaimed, now, leaseTimeStyle(), nil); got != "unclaimed" {
		t.Fatalf("claimSummary of an unclaimed task = %q", got)
	}
	expiry := now.Add(4*time.Minute + 30*time.Second)
	claimed := core.Task{ClaimedByActorID: "u1", LeaseExpiresAt: &expiry}
	got := claimSummary(claimed, now, leaseTimeStyle(), map[string]string{"u1": "alice"})
	want := "claimed by @alice, lease expires in 4 minutes"
	if got != want {
		t.Fatalf("claimSummary = %q, want %q", got, want)
	}
}

// TestActorLabelPrefersAResolvedHandle checks the fallback chain: a resolved
// handle, then a short identifier, so a lookup that could not run never
// blanks the field.
func TestActorLabelPrefersAResolvedHandle(t *testing.T) {
	actors := map[string]string{"u1": "alice"}
	if got := actorLabel("u1", actors); got != "@alice" {
		t.Fatalf("actorLabel with a resolved handle = %q", got)
	}
	if got := actorLabel("01M30NTSR60XBDNHE387GAX5JZ", actors); got != "01M30NTS" {
		t.Fatalf("actorLabel falling back to a short id = %q", got)
	}
	if got := actorLabel("", actors); got != "unknown" {
		t.Fatalf("actorLabel of an empty id = %q", got)
	}
}

// TestPeopleTextNamesAnUnassignedTaskInWords checks the quiet fallback for a
// task nobody is assigned to, rather than repeating "none".
func TestPeopleTextNamesAnUnassignedTaskInWords(t *testing.T) {
	task := core.Task{CreatorActorID: "creator"}
	got := peopleText(task, map[string]string{"creator": "root"})
	want := "people: created by @root   assigned to unassigned"
	if got != want {
		t.Fatalf("peopleText = %q, want %q", got, want)
	}
}

// TestTimeTextDropsARepeatedUpdatedStamp checks that a task edited the same
// moment it was created does not repeat its own timestamp.
func TestTimeTextDropsARepeatedUpdatedStamp(t *testing.T) {
	style, err := output.NewTimeStyle("iso", "utc")
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 9, 21, 8, 59, 0, 0, time.UTC)
	same := core.Task{CreatedAt: stamp, UpdatedAt: stamp}
	if got := timeText(same, style); got != "time: created 2026-09-21 08:59" {
		t.Fatalf("timeText with no edit = %q", got)
	}
	later := stamp.Add(2 * time.Hour)
	due := stamp.Add(48 * time.Hour)
	edited := core.Task{CreatedAt: stamp, UpdatedAt: later, DueAt: &due}
	want := "time: created 2026-09-21 08:59   updated 2026-09-21 10:59   due 2026-09-23 08:59"
	if got := timeText(edited, style); got != want {
		t.Fatalf("timeText with an edit and a due date = %q, want %q", got, want)
	}
}

// TestTagsTextIsEmptyWithoutTags checks the caller can tell "no tags" apart
// from "some tags" without a sentinel string.
func TestTagsTextIsEmptyWithoutTags(t *testing.T) {
	if got := tagsText(nil); got != "" {
		t.Fatalf("tagsText of no tags = %q, want empty", got)
	}
	if got := tagsText([]string{"urgent", "ops"}); got != "tags: urgent, ops" {
		t.Fatalf("tagsText = %q", got)
	}
}

// TestAnEmptyTaskIsShort is the other half of the detail redesign: a task
// with nothing in any optional section says so once, rather than spending a
// line on every section it lacks.
func TestAnEmptyTaskIsShort(t *testing.T) {
	m := boardModel(t)
	task, _ := TaskAt(m.columns, m.sel)
	m, _ = m.reduce(detailMsg{task: task})
	frame := m.View()
	if !strings.Contains(frame, "nothing else recorded on this task") {
		t.Fatalf("an empty task does not say it has nothing else:\n%s", frame)
	}
	for _, heading := range []string{"body:", "custom fields", "subtasks", "dependencies", "artifacts", "comments"} {
		if strings.Contains(frame, heading) {
			t.Fatalf("an empty task still spent a line on %q:\n%s", heading, frame)
		}
	}
}

// TestABusyTaskDoesNotSayNothingIsRecorded is the complement: a task with
// content in any section never shows the empty-task line, and does show the
// sections it has content for.
func TestABusyTaskDoesNotSayNothingIsRecorded(t *testing.T) {
	frame := detailModel(t, 100, 40).View()
	if strings.Contains(frame, "nothing else recorded") {
		t.Fatalf("a busy task claims to have nothing recorded:\n%s", frame)
	}
	for _, want := range []string{"body:", "subtasks (1):", "dependencies (1):", "comments (1):"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("a busy task is missing %q:\n%s", want, frame)
		}
	}
}

// TestActorHandlesReplaceRawIdentifiers checks that a resolved handle, not a
// raw actor id, is what reaches the screen.
func TestActorHandlesReplaceRawIdentifiers(t *testing.T) {
	m := boardModel(t)
	task, _ := TaskAt(m.columns, m.sel)
	task.CreatorActorID = "01M30NTSR60XBDNHE387GAX5JZ"
	m, _ = m.reduce(detailMsg{task: task, actors: map[string]string{task.CreatorActorID: "root"}})
	frame := m.View()
	if !strings.Contains(frame, "@root") {
		t.Fatalf("a resolved creator handle is missing:\n%s", frame)
	}
	if strings.Contains(frame, "01M30NTSR60XBDNHE387GAX5JZ") {
		t.Fatalf("the raw creator id leaked onto the screen:\n%s", frame)
	}
}
