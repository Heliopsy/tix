// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// snoozedBoard is the three task board with task b snoozed for a day.
func snoozedBoard(t *testing.T) (Model, *fakeService) {
	t.Helper()
	m := boardModel(t)
	svc := newFakeService()
	m.svc = svc
	until := m.now().Add(24 * time.Hour)
	tasks := slices.Clone(m.tasks)
	for i := range tasks {
		if tasks[i].ID == "b" {
			tasks[i].SnoozedUntil = &until
		}
	}
	m, _ = m.installTasks(tasks)
	return m, svc
}

func shownRefs(m Model) []string {
	var out []string
	for _, c := range m.columns {
		out = append(out, refsOf(c.Tasks)...)
	}
	return out
}

func TestASnoozedCardIsOffTheBoardUntilAFilterAsks(t *testing.T) {
	m, _ := snoozedBoard(t)
	if got := shownRefs(m); slices.Contains(got, "infra-b") {
		t.Fatalf("the board shows a snoozed card: %v", got)
	}
	m = m.applyFilterText("is:snoozed")
	if m.filterErr != "" {
		t.Fatalf("is:snoozed refused: %s", m.filterErr)
	}
	if got := shownRefs(m); !slices.Equal(got, []string{"infra-b"}) {
		t.Fatalf("is:snoozed shows %v, want only infra-b", got)
	}
	if line := cardLineFor(t, m.Frame(), "infra-b"); !strings.Contains(line, SnoozedMarker) {
		t.Errorf("the snoozed card carries no marker: %q", line)
	}
	m = m.applyFilterText("snoozed:any")
	if got := shownRefs(m); len(got) != 3 {
		t.Fatalf("snoozed:any shows %v, want all three", got)
	}
	if line := cardLineFor(t, m.Frame(), "infra-a"); strings.Contains(line, SnoozedMarker) {
		t.Errorf("an awake card carries the snoozed marker: %q", line)
	}
}

func TestTheSnoozeKeyAsksWhenAndSnoozes(t *testing.T) {
	m, svc := snoozedBoard(t)
	m.sel = PreserveSelection(m.columns, m.sel, "a")
	next, cmd := typeInto(t, m, "s", "tomorrow")
	msg := run(t, cmd)
	if len(svc.snoozes) != 1 {
		t.Fatalf("snoozes = %+v, want one", svc.snoozes)
	}
	want := time.Date(2026, 1, 2, 9, 0, 0, 0, time.Local)
	if !svc.snoozes[0].Until.Equal(want) {
		t.Fatalf("snoozed until %v, want %v", svc.snoozes[0].Until, want)
	}
	after, reload := next.reduce(msg)
	if after.err != "" || reload == nil {
		t.Fatalf("the snooze did not reload the board: err %q", after.err)
	}
	if !strings.Contains(after.status, "snoozed infra-a until") {
		t.Errorf("status = %q, want it to say until when", after.status)
	}
}

func TestAnUnreadableTimeIsSaidAndNotSent(t *testing.T) {
	m, svc := snoozedBoard(t)
	m.sel = PreserveSelection(m.columns, m.sel, "a")
	next, cmd := typeInto(t, m, "s", "someday")
	after, _ := next.reduce(run(t, cmd))
	if len(svc.snoozes) != 0 {
		t.Fatalf("an unreadable time reached the service: %+v", svc.snoozes)
	}
	if !strings.Contains(after.err, "someday") {
		t.Fatalf("err = %q, want it to name what it could not read", after.err)
	}
}

func TestTheSnoozeKeyUnsnoozesASnoozedCard(t *testing.T) {
	m, svc := snoozedBoard(t)
	m = m.applyFilterText("is:snoozed")
	m.sel = PreserveSelection(m.columns, m.sel, "b")
	next, cmd := m.reduce(pressKey("s"))
	if next.prompt != promptNone {
		t.Fatal("s on a snoozed card asked when, rather than unsnoozing")
	}
	run(t, cmd)
	if !slices.Equal(svc.unsnooze, []core.TaskRef{{ID: "b"}}) || len(svc.snoozes) != 0 {
		t.Fatalf("unsnoozed %v, snoozed %v; want one unsnooze of b", svc.unsnooze, svc.snoozes)
	}
}

func TestTheSnoozeKeyIsNeitherOfferedNorSentWithoutTheAuthority(t *testing.T) {
	m, svc := snoozedBoard(t)
	m.actor = &core.Actor{ID: "u-read", TenantID: "t", Kind: core.ActorAgent, Scopes: []core.Scope{core.ScopeTaskRead, core.ScopeProjectRead}}
	m.allowed = resolveActions(m.actor)
	if m.mayPerform("SnoozeTask") {
		t.Fatal("the reader may snooze, so this test proves nothing")
	}
	help := m.keys.ViewHelp(viewBoard, m.permits())
	if slices.ContainsFunc(help, func(e HelpEntry) bool { return e.Keys == "s" }) {
		t.Errorf("a reader who may not snooze is told about s: %+v", help)
	}
	next, _ := m.reduce(pressKey("s"))
	if next.prompt == promptSnooze {
		t.Fatal("a reader who may not snooze was asked when")
	}
	if len(svc.snoozes)+len(svc.unsnooze) != 0 {
		t.Fatalf("a reader who may not snooze called the service")
	}
}

func TestTheDetailViewSaysUntilWhen(t *testing.T) {
	until := time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC)
	task := core.Task{SnoozedUntil: &until, SnoozedByActorID: "u1"}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got := snoozeText(task, now, leaseTimeStyle(), map[string]string{"u1": "alice"})
	if !strings.Contains(got, "snoozed until") || !strings.Contains(got, "alice") {
		t.Fatalf("snoozeText = %q", got)
	}
	if snoozeText(task, until.Add(time.Second), leaseTimeStyle(), nil) != "" {
		t.Fatal("a passed snooze is still described")
	}
}
