// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// inboxBlock returns only the lines the inbox listing drew, from its heading
// down to the status bar that counts it, so an assertion cannot be satisfied
// by the board behind the view or the footer naming the keys.
func inboxBlock(t *testing.T, frame string) string {
	t.Helper()
	lines := strings.Split(frame, "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "inbox" {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("the frame draws no inbox heading:\n%s", frame)
	}
	for i := start; i < len(lines); i++ {
		if strings.Contains(lines[i], " unread") && strings.Contains(lines[i], "items,") {
			return strings.Join(lines[start:i], "\n")
		}
	}
	t.Fatalf("the inbox drew no count, so its block has no end:\n%s", frame)
	return ""
}

func inboxModel(t *testing.T) (Model, *fakeService, core.Task) {
	t.Helper()
	m := boardModel(t)
	svc := newFakeService()
	task, _ := TaskAt(m.columns, m.sel)
	svc.tasks = []core.Task{task}
	read := auditWhen()
	svc.inbox = []core.InboxItem{
		{ID: "i2", Kind: core.InboxStatus, TaskID: task.ID, TaskRef: task.Ref, TaskTitle: task.Title,
			ActorHandle: "ada", FromStatus: "todo", ToStatus: "doing", CreatedAt: auditWhen()},
		{ID: "i1", Kind: core.InboxMention, TaskID: task.ID, TaskRef: task.Ref, TaskTitle: task.Title,
			ActorHandle: "bob", CreatedAt: auditWhen(), ReadAt: &read},
	}
	m.svc = svc
	m, cmd := m.reduce(pressKey("I"))
	if cmd == nil {
		t.Fatal("I asked for no read")
	}
	m, _ = m.reduce(run(t, cmd))
	if m.view != viewInbox {
		t.Fatalf("I did not open the inbox; view = %v", m.view)
	}
	return m, svc, task
}

func TestTheInboxViewListsMarksAndOpens(t *testing.T) {
	m, svc, task := inboxModel(t)
	block := inboxBlock(t, m.Frame())
	if !strings.Contains(block, "● ") || !strings.Contains(block, "ada moved it from todo to doing") ||
		!strings.Contains(block, "bob mentioned you") || !strings.Contains(block, task.Ref) {
		t.Fatalf("the inbox does not draw its items:\n%s", block)
	}
	if strings.Count(block, "● ") != 1 {
		t.Fatalf("only the unread item should carry a dot:\n%s", block)
	}

	m, cmd := m.reduce(pressKey(" "))
	m, _ = m.reduce(run(t, cmd))
	if n, _ := svc.CountInbox(nil); n.Unread != 0 || !m.inbox[0].Read() {
		t.Fatalf("space did not mark the selected item read: unread = %d", n.Unread)
	}
	m, cmd = m.reduce(pressKey(" "))
	m, _ = m.reduce(run(t, cmd))
	if n, _ := svc.CountInbox(nil); n.Unread != 1 {
		t.Fatal("space on a read item did not mark it unread")
	}

	m, cmd = m.reduce(pressKey("enter"))
	m, cmd = m.reduce(run(t, cmd))
	if n, _ := svc.CountInbox(nil); n.Unread != 0 {
		t.Fatal("opening an unread item did not mark it read")
	}
	if cmd == nil {
		t.Fatal("opening an item asked for no task")
	}
	m, _ = m.reduce(run(t, cmd))
	if m.view != viewDetail || m.detail == nil || m.detail.task.ID != task.ID {
		t.Fatalf("enter did not open the item's task; view = %v", m.view)
	}
}

func TestReadAllMarksEveryInboxItem(t *testing.T) {
	m, svc, _ := inboxModel(t)
	svc.inbox[1].ReadAt = nil
	m, cmd := m.reduce(pressKey("a"))
	m, _ = m.reduce(run(t, cmd))
	if n, _ := svc.CountInbox(nil); n.Unread != 0 {
		t.Fatalf("a left %d unread", n.Unread)
	}
	if strings.Contains(inboxBlock(t, m.Frame()), "● ") {
		t.Fatal("the listing still draws an unread dot after marking everything read")
	}
}

func TestTheInboxIsNeitherOpenedNorReadWithoutAccess(t *testing.T) {
	m := boardModel(t)
	svc := newFakeService()
	m.svc = svc
	m.access = ViewAccess{"board": true, "detail": true}
	next, cmd := m.reduce(pressKey("I"))
	if next.view == viewInbox {
		t.Fatal("a reader refused the inbox was shown it")
	}
	if cmd != nil {
		next, _ = next.reduce(run(t, cmd))
	}
	if svc.inboxReads != 0 {
		t.Fatalf("the inbox was read %d times for a reader refused it", svc.inboxReads)
	}
}
