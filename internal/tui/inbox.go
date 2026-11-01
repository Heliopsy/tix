// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// inboxLimit is how much of the inbox one screen reads. The rest is a
// question for `tix inbox ls --cursor`, and the view says so when there is more.
const inboxLimit = 100

// inboxMsg carries one page of the reader's inbox.
type inboxMsg struct {
	items []core.InboxItem
	more  bool
	err   error
}

// inboxMarkMsg reports one item marked read or unread, or everything read.
type inboxMarkMsg struct {
	id     string
	read   bool
	all    bool
	open   bool
	taskID string
	err    error
}

// InboxRow renders one item in words: when, who, what happened, to which task.
// It is the same reduction `tix inbox ls` prints, the unread ones marked.
func InboxRow(it core.InboxItem, style output.TimeStyle) string {
	mark := "  "
	if !it.Read() {
		mark = "● "
	}
	by := it.ActorHandle
	if by == "" {
		by = "somebody"
	}
	var what string
	switch it.Kind {
	case core.InboxMention:
		what = by + " mentioned you"
	case core.InboxAssigned:
		what = by + " assigned you"
	case core.InboxStatus:
		what = by + " moved it from " + it.FromStatus + " to " + it.ToStatus
	default:
		what = by + " commented"
	}
	task := it.TaskRef
	if it.TaskGone || task == "" {
		task = "(deleted task)"
	} else if it.TaskTitle != "" {
		task += " " + it.TaskTitle
	}
	return mark + strings.Join([]string{style.Format(it.CreatedAt), what, task}, "  ")
}

// InboxEmptyState is what the inbox says when it holds nothing.
func InboxEmptyState(items int) EmptyState {
	if items > 0 {
		return EmptyState{}
	}
	return EmptyState{
		Title: "Your inbox is empty.",
		Hint:  "Mentions, assignments, comments on tasks you follow and status changes land here.",
	}
}

// InboxCount is what the status bar says the view is holding.
func InboxCount(items []core.InboxItem, more bool) string {
	unread := 0
	for _, it := range items {
		if !it.Read() {
			unread++
		}
	}
	count := strconv.Itoa(len(items)) + " items, " + strconv.Itoa(unread) + " unread"
	if more {
		return count + "; older ones with tix inbox ls"
	}
	return count
}

// openInbox reads the reader's inbox and opens the view on it.
func (m Model) openInbox() (Model, tea.Cmd) {
	if !m.canReach(viewInbox) {
		return m, nil
	}
	return m, m.loadInbox()
}

func (m Model) loadInbox() tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx := m.svc, m.ctx
	return func() tea.Msg {
		items, next, err := svc.ListInbox(ctx, core.InboxFilter{Page: core.Page{Limit: inboxLimit}})
		return inboxMsg{items: items, more: next != "", err: err}
	}
}

// onInbox installs the listing. A read that failed opens nothing; the status
// bar carries the reason.
func (m Model) onInbox(msg inboxMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.err = "reading the inbox: " + msg.err.Error()
		return m, nil
	}
	m.inbox, m.inboxMore = msg.items, msg.more
	m.inboxSel = clamp(m.inboxSel, 0, len(m.inbox)-1)
	m.err = ""
	if m.view == viewInbox {
		return m, nil
	}
	m.inboxSel, m.inboxOff = 0, 0
	return m.enterView(viewInbox), nil
}

// markInbox marks one item read or unread. open carries on to the task once
// the mark has landed.
func (m Model) markInbox(it core.InboxItem, read, open bool) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx := m.svc, m.ctx
	return func() tea.Msg {
		mark := svc.ReadInboxItem
		if !read {
			mark = svc.UnreadInboxItem
		}
		_, err := mark(ctx, it.ID)
		return inboxMarkMsg{id: it.ID, read: read, open: open, taskID: it.TaskID, err: err}
	}
}

func (m Model) readAllInbox() tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx := m.svc, m.ctx
	return func() tea.Msg {
		_, err := svc.ReadAllInbox(ctx)
		return inboxMarkMsg{all: true, read: true, err: err}
	}
}

// onInboxMark applies a mark to the listing on screen, and opens the task when
// the mark was the first half of opening it.
func (m Model) onInboxMark(msg inboxMarkMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.err = "marking the inbox: " + msg.err.Error()
		return m, nil
	}
	m.err = ""
	for i := range m.inbox {
		if msg.all || m.inbox[i].ID == msg.id {
			m.inbox[i].ReadAt = nil
			if msg.read {
				m.inbox[i].ReadAt = new(time.Time)
			}
		}
	}
	switch {
	case msg.all:
		m.status = "inbox marked read"
	case msg.open:
		return m, m.detailFor(core.TaskRef{ID: msg.taskID})
	case msg.read:
		m.status = "marked read"
	default:
		m.status = "marked unread"
	}
	return m, nil
}

// handleInboxKey moves through the listing, opens the selected task, toggles
// its read state and marks everything read.
func (m Model) handleInboxKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		return m.leave(nil)
	case key.Matches(msg, m.keys.Up):
		m.inboxSel = clamp(m.inboxSel-1, 0, len(m.inbox)-1)
	case key.Matches(msg, m.keys.Down):
		m.inboxSel = clamp(m.inboxSel+1, 0, len(m.inbox)-1)
	case key.Matches(msg, m.keys.Top):
		m.inboxSel = 0
	case key.Matches(msg, m.keys.Bottom):
		m.inboxSel = max(0, len(m.inbox)-1)
	case key.Matches(msg, m.keys.ReadAll):
		return m, m.readAllInbox()
	case len(m.inbox) == 0:
		return m, nil
	case key.Matches(msg, m.keys.Enter):
		it := m.inbox[m.inboxSel]
		if it.TaskGone {
			m.err = "that task has been deleted"
			return m, nil
		}
		if it.Read() {
			return m, m.detailFor(core.TaskRef{ID: it.TaskID})
		}
		return m, m.markInbox(it, true, true)
	case key.Matches(msg, m.keys.ToggleRead):
		it := m.inbox[m.inboxSel]
		return m, m.markInbox(it, !it.Read(), false)
	}
	m.inboxOff = ScrollWindow(m.inboxOff, m.inboxSel, m.inboxRows(), len(m.inbox))
	return m, nil
}

func (m Model) inboxRows() int {
	body := LayoutFor(m.width, m.height, len(m.columns)).BodyHeight - projectHeaderLines
	if body < 1 {
		body = 1
	}
	return VisibleRows(body, len(m.inbox))
}

// inboxLines renders the listing under its heading.
func (m Model) inboxLines(layout Layout) []string {
	head := []string{m.theme.Header.Render("  inbox"), ""}
	if empty := InboxEmptyState(len(m.inbox)); !empty.Zero() {
		return append(head, m.emptyLines(empty)...)
	}
	rows := VisibleRows(layout.BodyHeight-projectHeaderLines, len(m.inbox))
	offset := ScrollWindow(m.inboxOff, m.inboxSel, rows, len(m.inbox))
	lines := make([]string, 0, rows+1)
	for i := offset; i < len(m.inbox) && len(lines) < rows; i++ {
		lines = append(lines, m.fit(m.inboxLine(m.inbox[i], i == m.inboxSel)))
	}
	if hint := ScrollHint(offset, rows, len(m.inbox)); hint != "" {
		lines = append(lines, m.theme.Dim.Render("  "+hint))
	}
	return append(head, lines...)
}

func (m Model) inboxLine(it core.InboxItem, selected bool) string {
	row := "  " + SelectionMarker(selected) + InboxRow(it, m.timeStyle)
	switch {
	case selected:
		return m.selection().Render(Truncate(row, m.width))
	case it.Read():
		return m.theme.Dim.Render(row)
	default:
		return row
	}
}
