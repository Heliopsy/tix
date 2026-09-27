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

// historyBlock returns only the lines the history listing drew, from the
// heading naming its subject down to the status bar that counts it.
//
// Every assertion about the view goes through here rather than through the
// frame. The title bar carries the word "history", the board behind the view
// still holds the cards and their refs, and the footer names the keys, so an
// assertion read off the whole frame would be satisfied by a line the view did
// not draw.
func historyBlock(t *testing.T, frame string) string {
	t.Helper()
	lines := strings.Split(frame, "\n")
	start := -1
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "history of ") {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("the frame draws no history heading:\n%s", frame)
	}
	for i := start; i < len(lines); i++ {
		if strings.Contains(lines[i], "entries, newest first") {
			return strings.Join(lines[start:i], "\n")
		}
	}
	t.Fatalf("the history view drew no count, so its block has no end:\n%s", frame)
	return ""
}

// historyEntryLine returns the one line of the listing carrying a word, so an
// assertion about one entry cannot be satisfied by another entry or by the
// heading above them.
func historyEntryLine(t *testing.T, frame, want string) string {
	t.Helper()
	var found []string
	for _, l := range strings.Split(historyBlock(t, frame), "\n") {
		if strings.Contains(l, want) {
			found = append(found, l)
		}
	}
	if len(found) != 1 {
		t.Fatalf("the history listing draws %d lines carrying %q, want exactly one:\n%s",
			len(found), want, historyBlock(t, frame))
	}
	return found[0]
}

// auditWhen is the instant the scripted log entries happened at.
func auditWhen() time.Time { return time.Date(2026, 3, 4, 9, 30, 0, 0, time.UTC) }

// historyService scripts a durable log for one task, written by an actor whose
// handle the view has to resolve.
func historyService() *fakeService {
	svc := newFakeService()
	svc.actors = map[string]*core.Actor{"a-ada": {ID: "a-ada", Handle: "ada"}}
	svc.audit = []core.AuditEntry{
		{Seq: 7, ActorID: "a-ada", Action: "task.created", SubjectType: "task",
			SubjectID: "t1", Source: core.SourceCLI, OccurredAt: auditWhen()},
		{Seq: 9, ActorID: "a-ada", Action: "task.transitioned", SubjectType: "task",
			SubjectID: "t1", Source: core.SourceWeb, OccurredAt: auditWhen()},
	}
	return svc
}

// historyModel opens the history view the way a reader does: from the board,
// with the key, draining the read the key asked for.
func historyModel(t *testing.T) (Model, *fakeService) {
	t.Helper()
	m := boardModel(t)
	svc := historyService()
	m.svc = svc
	m, cmd := m.reduce(pressKey("H"))
	if cmd == nil {
		t.Fatal("H asked for no read")
	}
	m, _ = m.reduce(run(t, cmd))
	if m.view != viewHistory {
		t.Fatalf("H did not open the history view; view = %v", m.view)
	}
	return m, svc
}

func TestTheHistoryViewReadsTheStoredLogForWhatIsSelected(t *testing.T) {
	m, svc := historyModel(t)
	task, _ := TaskAt(m.columns, m.sel)

	if len(svc.auditFilter) != 1 {
		t.Fatalf("the view made %d audit reads", len(svc.auditFilter))
	}
	got := svc.auditFilter[0]
	if got.SubjectType != "task" || got.SubjectID != task.ID {
		t.Errorf("the history was read for subject %q/%q, not for the selected task %q",
			got.SubjectType, got.SubjectID, task.ID)
	}
	if got.Page.Limit != historyLimit {
		t.Errorf("the history read asked for %d entries, not the page it draws", got.Page.Limit)
	}

	frame := m.Frame()
	if head := historyBlock(t, frame); !strings.Contains(head, task.Ref) {
		t.Errorf("the history does not name whose history it is:\n%s", head)
	}
	if line := historyEntryLine(t, frame, "task.transitioned"); !strings.Contains(line, "@ada") ||
		!strings.Contains(line, "#9") || !strings.Contains(line, "via web") {
		t.Errorf("the entry line says %q, without the actor, the sequence or the source", line)
	}
}

// TestTheStoredHistoryIsNotTheLiveTail is the decision this view exists for,
// stated as a guard. Activity draws the subscription this session has seen;
// history draws what the store recorded before it. Folding one into the other
// would have this assertion fail in both directions at once.
func TestTheStoredHistoryIsNotTheLiveTail(t *testing.T) {
	m := boardModel(t)
	svc := historyService()
	m.svc = svc
	m = m.recordActivity(core.Event{Seq: 40, Type: core.EventTaskCreated,
		SubjectType: "task", SubjectID: "t1", OccurredAt: auditWhen()})

	m, cmd := m.reduce(pressKey("H"))
	m, _ = m.reduce(run(t, cmd))
	stored := historyBlock(t, m.Frame())
	if !strings.Contains(stored, "task.transitioned") {
		t.Errorf("the history view does not draw the stored log:\n%s", stored)
	}
	if strings.Contains(stored, "#40") {
		t.Errorf("the history view drew a live event it never read:\n%s", stored)
	}

	m, _ = m.reduce(pressKey("v"))
	if m.view != viewActivity {
		t.Fatalf("v did not open the activity view; view = %v", m.view)
	}
	tail := m.Frame()
	if !strings.Contains(tail, "created") {
		t.Errorf("the activity view does not draw the live event:\n%s", tail)
	}
	if strings.Contains(tail, "task.transitioned") {
		t.Errorf("the activity view drew a stored entry the subscription never sent:\n%s", tail)
	}
}

// TestHistoryIsNeitherOpenedNorReadWithoutAuditRead watches both halves of the
// gate. A keystroke that is refused while the read still goes out is the shape
// a previous mutation found: the view never opened, and the service was asked
// all the same.
func TestHistoryIsNeitherOpenedNorReadWithoutAuditRead(t *testing.T) {
	m := boardModel(t)
	svc := historyService()
	m.svc = svc
	m.access = ViewAccess{"board": true, "detail": true}

	next, cmd := m.reduce(pressKey("H"))
	if next.view == viewHistory {
		t.Fatal("a reader refused the history view was shown it")
	}
	if cmd != nil {
		next, _ = next.reduce(run(t, cmd))
	}
	if len(svc.auditFilter) != 0 {
		t.Fatalf("a reader refused the history view read the audit log anyway: %+v", svc.auditFilter)
	}
	if next.view == viewHistory {
		t.Fatal("the refused read still opened the view")
	}
}

// TestTheHelpOverlayOffersHistoryOnlyWhereItIsReachable keeps the overlay and
// the key in step: a documented key is a promise that pressing it does
// something.
func TestTheHelpOverlayOffersHistoryOnlyWhereItIsReachable(t *testing.T) {
	keys := DefaultKeyMap()
	offered := func(v viewKind) bool { return v == viewHistory }
	withIt := keys.GlobalHelp(offered)
	if !slices.ContainsFunc(withIt, func(e HelpEntry) bool { return e.Keys == "H" }) {
		t.Fatalf("a reader who may read the history is not told about H: %+v", withIt)
	}
	without := keys.GlobalHelp(func(viewKind) bool { return false })
	if slices.ContainsFunc(without, func(e HelpEntry) bool { return e.Keys == "H" }) {
		t.Fatalf("a reader who may not read the history is told about H: %+v", without)
	}
}

// TestEverySchemeDrivesTheHistoryView proves the view is usable under all five
// presets rather than under the shipped one alone.
func TestEverySchemeDrivesTheHistoryView(t *testing.T) {
	for _, scheme := range Schemes() {
		t.Run(string(scheme), func(t *testing.T) {
			m := boardModel(t)
			m.svc = historyService()
			m = m.installScheme(string(scheme))
			if m.err != "" {
				t.Fatalf("installing %s: %s", scheme, m.err)
			}
			m, cmd := m.reduce(keyMsgFor(m.keys.History.Keys()[0]))
			if cmd == nil {
				t.Fatalf("%s: the history key asked for no read", scheme)
			}
			m, _ = m.reduce(run(t, cmd))
			if m.view != viewHistory {
				t.Fatalf("%s: the history key opened %v", scheme, viewName(m.view))
			}
			down, _ := m.reduce(keyMsgFor(m.keys.Down.Keys()[0]))
			if down.historySel != 1 {
				t.Errorf("%s: the down key left the selection on %d", scheme, down.historySel)
			}
			back, _ := down.reduce(keyMsgFor(m.keys.Back.Keys()[0]))
			if back.view == viewHistory {
				t.Errorf("%s: the back key did not leave the history view", scheme)
			}
		})
	}
}

func TestTheHistorySubjectFollowsWhatTheReaderIsLookingAt(t *testing.T) {
	board := boardModel(t)
	task, _ := TaskAt(board.columns, board.sel)

	projects := board.rootView()
	projects.projects = []core.Project{{ID: "p1", Key: "infra"}}

	setup := board
	setup.view = viewProject
	setup.setup = &projectMsg{project: core.Project{ID: "p9", Key: "web"}}

	tests := []struct {
		name string
		m    Model
		want HistorySubject
	}{
		{"a selected card", board, HistorySubject{Type: "task", ID: task.ID, Label: task.Ref}},
		{"the project screen", setup, HistorySubject{Type: "project", ID: "p9", Label: "project web"}},
		{"the project list", projects, HistorySubject{Type: "project", ID: "p1", Label: "project infra"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.m.historySubject(); got != tc.want {
				t.Fatalf("historySubject() = %+v, want %+v", got, tc.want)
			}
		})
	}

	empty := New(Config{Access: fullAccess(), Actor: fullActor()})
	if got := empty.historySubject(); got.Type != "" || got.ID != "" {
		t.Fatalf("a session looking at nothing scoped its history to %+v", got)
	}
	if got := (HistorySubject{}).Filter(); got.SubjectType != "" || got.SubjectID != "" {
		t.Fatalf("a subjectless history narrowed the log to %+v", got)
	}
}

func TestAnEmptyHistorySaysSoRatherThanDrawingNothing(t *testing.T) {
	m := boardModel(t)
	m.svc = newFakeService()
	m, cmd := m.reduce(pressKey("H"))
	m, _ = m.reduce(run(t, cmd))

	block := historyBlock(t, m.Frame())
	task, _ := TaskAt(m.columns, m.sel)
	if !strings.Contains(block, "Nothing is recorded against "+task.Ref) {
		t.Errorf("an empty history draws no explanation:\n%s", block)
	}
}

// TestRefreshingTheHistoryReadsItsOwnSubjectAgain keeps the refresh key on the
// screen the reader is looking at. The history is read once when the view opens,
// so a refresh that reloaded the board would leave a stale log on screen and
// report nothing.
func TestRefreshingTheHistoryReadsItsOwnSubjectAgain(t *testing.T) {
	m, svc := historyModel(t)
	subject := m.historySubj

	next, cmd := m.reduce(pressKey("r"))
	if cmd == nil {
		t.Fatal("refreshing the history asked for nothing")
	}
	next.reduce(run(t, cmd))
	if len(svc.auditFilter) != 2 {
		t.Fatalf("the refresh made %d audit reads in total", len(svc.auditFilter))
	}
	if got := svc.auditFilter[1]; got.SubjectID != subject.ID {
		t.Fatalf("the refresh read the log for %q, not for the subject on screen %q",
			got.SubjectID, subject.ID)
	}
}

func TestHistoryCountNamesTheBoundWhenTheLogRunsPastThePage(t *testing.T) {
	if got := HistoryCount(3, false); got != "3 entries, newest first" {
		t.Errorf("a whole log counts itself as %q", got)
	}
	got := HistoryCount(100, true)
	if !strings.Contains(got, "100 entries") || !strings.Contains(got, "tix audit ls") {
		t.Errorf("a truncated log counts itself as %q, without saying where the rest is", got)
	}
}

// TestTheRegistryBindsTheAuditListingToTheHistoryView keeps the screen and the
// registry's claim about it together.
func TestTheRegistryBindsTheAuditListingToTheHistoryView(t *testing.T) {
	op, ok := capability.ByMethod("ListAudit")
	if !ok {
		t.Fatal("the registry declares no audit listing")
	}
	if op.TUI != viewName(viewHistory) {
		t.Fatalf("the audit listing is bound to view %q, not to the history view", op.TUI)
	}
	if op.Scope != core.ScopeAuditRead {
		t.Fatalf("the history view is gated on %q", op.Scope)
	}
	subscribe, _ := capability.ByMethod("Subscribe")
	if subscribe.TUI == op.TUI {
		t.Fatal("the live tail and the stored log claim one view, so one of them gates the other")
	}
}
