// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// historyLimit is how much of the durable log one screen reads. A history view
// answers "what happened to this", which the recent end answers; the rest is a
// question for `tix audit ls`, and the view says so when there is more.
const historyLimit = 100

// HistorySubject is what a history listing is scoped to: one task, one project,
// or the tenant when the reader asked from somewhere that names neither.
type HistorySubject struct {
	Type  string
	ID    string
	Label string
}

// Title names the listing, which is the whole of what tells a task's history
// apart from the tenant's.
func (s HistorySubject) Title() string {
	if s.Label == "" {
		return "history of this tenant"
	}
	return "history of " + s.Label
}

// Filter selects the entries this subject's history is made of. A subject that
// names nothing selects the tenant's whole log, which is what the audit listing
// returns with no subject named.
func (s HistorySubject) Filter() core.AuditFilter {
	return core.AuditFilter{
		SubjectType: s.Type, SubjectID: s.ID,
		Page: core.Page{Limit: historyLimit},
	}
}

// historySubject decides whose history the key opens. It is the same rule the
// project screen's subject follows: the thing the reader is looking at, so the
// key means one thing wherever it is pressed.
func (m Model) historySubject() HistorySubject {
	if m.view == viewProject && m.setup != nil {
		return HistorySubject{Type: "project", ID: m.setup.project.ID,
			Label: "project " + m.setup.project.Key}
	}
	if task, ok := m.selectedTask(); ok && m.view != viewProjects {
		return HistorySubject{Type: "task", ID: task.ID, Label: task.Ref}
	}
	if m.view == viewProjects && m.projectSel < len(m.projects) {
		p := m.projects[m.projectSel]
		return HistorySubject{Type: "project", ID: p.ID, Label: "project " + p.Key}
	}
	return HistorySubject{}
}

// HistoryRow renders one audit entry in the words the entry itself records: the
// sequence it was written at, when it happened, who did it, what was done and
// to what. It is the same reduction `tix audit ls` prints, so the two surfaces
// cannot describe one change two different ways.
func HistoryRow(e core.AuditEntry, style output.TimeStyle, actors map[string]string) string {
	subject := e.SubjectType
	if e.SubjectID != "" {
		subject += "/" + shortID(e.SubjectID)
	}
	parts := []string{
		"#" + strconv.FormatInt(e.Seq, 10),
		style.Format(e.OccurredAt),
		actorLabel(e.ActorID, actors),
		e.Action,
		subject,
	}
	if e.Source != "" {
		parts = append(parts, "via "+string(e.Source))
	}
	return strings.Join(parts, "  ")
}

// HistoryEmptyState decides what the history view says when it draws no line.
// A subject whose log is genuinely empty has to say so in words: a screen with
// a heading over blank space reads as a read that failed, and a read that did
// fail never opens this view at all.
func HistoryEmptyState(entries int, subject HistorySubject) EmptyState {
	if entries > 0 {
		return EmptyState{}
	}
	return EmptyState{
		Title: "Nothing is recorded against " + subject.Named() + " yet.",
		Hint:  "The durable log records changes; `v` watches them as they happen.",
	}
}

// Named is the subject in the words an empty state uses about it.
func (s HistorySubject) Named() string {
	if s.Label == "" {
		return "this tenant"
	}
	return s.Label
}

// HistoryCount is what the status bar says the view is holding, naming the
// bound rather than leaving a truncated log looking like the whole of one.
func HistoryCount(entries int, more bool) string {
	count := strconv.Itoa(entries) + " entries, newest first"
	if more {
		return count + "; older ones with tix audit ls"
	}
	return count
}

// openHistory reads the durable record of what happened to whatever the reader
// is looking at.
//
// It is a view of its own rather than a section of the activity screen, and the
// two are not the same question. Activity is the live tail of a subscription:
// it starts empty, holds the latest two hundred events of the session, and is
// offered to a reader holding event:subscribe. History is the stored log: it
// answers what happened before this session, it is bounded by a page rather
// than by a session, and reading it needs audit:read. Folding one into the
// other would offer a reader with audit:read no history at all, and would
// promise a reader with only event:subscribe a record the service will refuse.
func (m Model) openHistory() (Model, tea.Cmd) {
	if !m.canReach(viewHistory) {
		return m, nil
	}
	return m, m.loadHistory(m.historySubject())
}

// onHistory installs a history listing and opens the view on it. A read that
// failed opens nothing: there is no screen worth entering that says only that
// the read was refused, and the status bar already carries the reason.
func (m Model) onHistory(msg historyMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.err = "reading history: " + msg.err.Error()
		return m, nil
	}
	m.history, m.historySubj, m.historyMore = msg.entries, msg.subject, msg.more
	m.historyActors, m.historySel, m.historyOff = msg.actors, 0, 0
	m.err = ""
	return m.enterView(viewHistory), nil
}

// handleHistoryKey scrolls the listing and returns to where it was opened from,
// the same back-navigation every other view follows.
func (m Model) handleHistoryKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		return m.leave(nil)
	case key.Matches(msg, m.keys.Up):
		m.historySel = clamp(m.historySel-1, 0, len(m.history)-1)
	case key.Matches(msg, m.keys.Down):
		m.historySel = clamp(m.historySel+1, 0, len(m.history)-1)
	case key.Matches(msg, m.keys.Top):
		m.historySel = 0
	case key.Matches(msg, m.keys.Bottom):
		m.historySel = max(0, len(m.history)-1)
	}
	m.historyOff = ScrollWindow(m.historyOff, m.historySel, m.historyRows(), len(m.history))
	return m, nil
}

// historyRows is how many entries the view has room to draw, the heading and
// its blank line taken out of the same total the rows come from.
func (m Model) historyRows() int {
	body := LayoutFor(m.width, m.height, len(m.columns)).BodyHeight - projectHeaderLines
	if body < 1 {
		body = 1
	}
	return VisibleRows(body, len(m.history))
}

// historyLines renders the stored log, headed by what it is the history of,
// because a listing of changes with no subject named is a listing of changes to
// whatever happened to be selected.
func (m Model) historyLines(layout Layout) []string {
	if empty := HistoryEmptyState(len(m.history), m.historySubj); !empty.Zero() {
		return append([]string{m.theme.Header.Render("  " + m.historySubj.Title()), ""},
			m.emptyLines(empty)...)
	}
	head := []string{m.theme.Header.Render("  " + m.historySubj.Title()), ""}
	rows := VisibleRows(layout.BodyHeight-projectHeaderLines, len(m.history))
	offset := ScrollWindow(m.historyOff, m.historySel, rows, len(m.history))
	lines := make([]string, 0, rows+1)
	for i := offset; i < len(m.history) && len(lines) < rows; i++ {
		lines = append(lines, m.fit(m.historyLine(m.history[i], i == m.historySel)))
	}
	if hint := ScrollHint(offset, rows, len(m.history)); hint != "" {
		lines = append(lines, m.theme.Dim.Render("  "+hint))
	}
	return append(head, lines...)
}

// historyLine draws one entry, marked the way a selected row is marked
// everywhere else.
func (m Model) historyLine(e core.AuditEntry, selected bool) string {
	row := "  " + SelectionMarker(selected) + HistoryRow(e, m.timeStyle, m.historyActors)
	if selected {
		return m.selection().Render(Truncate(row, m.width))
	}
	return m.theme.Dim.Render(row)
}
