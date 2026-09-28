// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/capability"
	"github.com/heliopsy/tix/internal/core"
)

// cardLineFor returns the one card the board drew for a reference, so an
// assertion about a card's markers cannot be satisfied by the title bar, the
// status bar or another column's card carrying the same text. It is the card's
// meta line: the reference and the markers live there, under the title.
func cardLineFor(t *testing.T, frame, ref string) string {
	t.Helper()
	var found []string
	for _, l := range strings.Split(frame, "\n") {
		if strings.Contains(l, ref+CardSeparator+"P") {
			found = append(found, l)
		}
	}
	if len(found) != 1 {
		t.Fatalf("the board draws %d cards for %q, want exactly one:\n%s", len(found), ref, frame)
	}
	return found[0]
}

// footerLine returns the line advertising the view's keys, which is the last
// non-empty line of the frame. Reading the footer rather than the frame is what
// keeps "the footer offers u" from passing on a card whose title holds one.
func footerLine(t *testing.T, frame string) string {
	t.Helper()
	lines := strings.Split(frame, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return lines[i]
		}
	}
	t.Fatalf("the frame has no footer:\n%s", frame)
	return ""
}

// deletedBoard is a board the is:deleted filter revealed a deleted task on,
// which is the only place the restore key is offered.
func deletedBoard(t *testing.T) (Model, *fakeService) {
	t.Helper()
	m := boardModel(t)
	svc := newFakeService()
	m.svc = svc
	gone := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	tasks := make([]core.Task, len(m.tasks))
	copy(tasks, m.tasks)
	tasks[0].DeletedAt = &gone
	m = m.applyFilterText("is:deleted")
	if m.filterErr != "" {
		t.Fatalf("the board refused is:deleted: %s", m.filterErr)
	}
	m, _ = m.installTasks(tasks)
	return m, svc
}

// TestTheBoardsOwnFilterIsHowDeletedTasksAreSeen states where the terminal
// equivalent of `tix task ls --include-deleted` lives: the same expression, in
// the same parser, reaching the same service call.
func TestTheBoardsOwnFilterIsHowDeletedTasksAreSeen(t *testing.T) {
	m, _ := deletedBoard(t)
	if !m.filter.IncludeDeleted {
		t.Fatal("is:deleted on the board did not ask the service for deleted tasks")
	}
	if got := m.taskFilter("infra"); !got.IncludeDeleted {
		t.Fatal("the filter the board sends drops the deleted tasks the reader asked for")
	}
}

func TestADeletedCardIsMarkedAndOffersOnlyRestore(t *testing.T) {
	m, _ := deletedBoard(t)
	frame := m.Frame()
	deleted, _ := TaskAt(m.columns, m.sel)
	if deleted.DeletedAt == nil {
		t.Fatal("the selected card is not the deleted one")
	}

	if line := cardLineFor(t, frame, deleted.Ref); !strings.Contains(line, DeletedMarker) {
		t.Errorf("the deleted card carries no marker: %q", line)
	}
	if !slices.Contains(CardLegend, DeletedMarker+" deleted") {
		t.Errorf("the legend does not explain the deleted marker: %v", CardLegend)
	}

	footer := footerLine(t, frame)
	if !strings.Contains(footer, "restore") {
		t.Errorf("the footer of a deleted card does not offer a restore: %q", footer)
	}
	for _, refused := range []string{"claim", "edit title", "delete"} {
		if strings.Contains(footer, refused) {
			t.Errorf("the footer of a deleted card offers %q, which the service refuses: %q",
				refused, footer)
		}
	}
}

func TestALiveCardOffersNoRestore(t *testing.T) {
	m := boardModel(t)
	footer := footerLine(t, m.Frame())
	if strings.Contains(footer, "restore") {
		t.Errorf("the footer of a live card offers a restore: %q", footer)
	}
	live, _ := TaskAt(m.columns, m.sel)
	if line := cardLineFor(t, m.Frame(), live.Ref); strings.Contains(line, DeletedMarker) {
		t.Errorf("a live card is marked deleted: %q", line)
	}
}

func TestRestoreBringsTheDeletedTaskBack(t *testing.T) {
	m, svc := deletedBoard(t)
	deleted, _ := TaskAt(m.columns, m.sel)

	next, cmd := m.reduce(pressKey("u"))
	if cmd == nil {
		t.Fatalf("u asked for nothing; err = %q", next.err)
	}
	next, _ = next.reduce(run(t, cmd))
	if len(svc.restored) != 1 || svc.restored[0].ID != deleted.ID {
		t.Fatalf("restored = %+v, want the selected task %q", svc.restored, deleted.ID)
	}
	if !strings.Contains(next.status, "restored "+deleted.Ref) {
		t.Errorf("the status bar says %q", next.status)
	}
}

// TestRestoreRefusesATaskThatWasNeverDeleted keeps the key from sending a call
// the service would refuse, and points at where the deleted tasks are.
func TestRestoreRefusesATaskThatWasNeverDeleted(t *testing.T) {
	m := boardModel(t)
	svc := newFakeService()
	m.svc = svc
	next, cmd := m.reduce(pressKey("u"))
	if cmd != nil {
		next, _ = next.reduce(run(t, cmd))
	}
	if len(svc.restored) != 0 {
		t.Fatalf("a live task was restored: %+v", svc.restored)
	}
	if !strings.Contains(next.err, "is:deleted") {
		t.Errorf("the refusal does not say where the deleted tasks are: %q", next.err)
	}
}

// TestRestoreIsNeitherOfferedNorSentWithoutTaskDelete watches both halves: the
// keystroke is refused and no call goes out, which a gate that only hid the
// footer entry would not give.
func TestRestoreIsNeitherOfferedNorSentWithoutTaskDelete(t *testing.T) {
	m, svc := deletedBoard(t)
	m.actor = &core.Actor{ID: "u-viewer", TenantID: "t", Kind: core.ActorUser, Role: core.RoleViewer}
	m.allowed = resolveActions(m.actor)

	if m.mayPerform("RestoreTask") {
		t.Fatal("a viewer holds task:delete, so this test proves nothing")
	}
	if footer := footerLine(t, m.Frame()); strings.Contains(footer, "restore") {
		t.Errorf("a reader who may not restore is offered the key: %q", footer)
	}
	next, cmd := m.reduce(pressKey("u"))
	if cmd != nil {
		next, _ = next.reduce(run(t, cmd))
	}
	if len(svc.restored) != 0 {
		t.Fatalf("a reader who may not restore restored: %+v", svc.restored)
	}
	if next.err != "" {
		t.Errorf("a refused keystroke reported %q rather than doing nothing", next.err)
	}
}

func TestEverySchemeRestoresADeletedCard(t *testing.T) {
	for _, scheme := range Schemes() {
		t.Run(string(scheme), func(t *testing.T) {
			m, svc := deletedBoard(t)
			m = m.installScheme(string(scheme))
			if m.err != "" {
				t.Fatalf("installing %s: %s", scheme, m.err)
			}
			next, cmd := m.reduce(keyMsgFor(m.keys.Restore.Keys()[0]))
			if cmd == nil {
				t.Fatalf("%s: the restore key asked for nothing; err = %q", scheme, next.err)
			}
			next.reduce(run(t, cmd))
			if len(svc.restored) != 1 {
				t.Fatalf("%s: restored = %+v", scheme, svc.restored)
			}
		})
	}
}

func TestTheRegistryBindsRestoreToTheBoard(t *testing.T) {
	op, ok := capability.ByMethod("RestoreTask")
	if !ok {
		t.Fatal("the registry declares no restore")
	}
	if op.TUI != viewName(viewBoard) {
		t.Fatalf("restore is bound to view %q", op.TUI)
	}
}
