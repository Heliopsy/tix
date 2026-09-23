// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// activityCap bounds how many events the activity view keeps, so a long
// session watching a busy tenant cannot grow this without limit.
const activityCap = 200

// openActivity shows the live event tail. The subscription already runs in
// the background for every view, so opening this one only changes what is
// drawn, never what is fetched.
func (m Model) openActivity() Model { return m.enterView(viewActivity) }

// recordActivity appends an event to the capped ring the activity view draws
// from, dropping the oldest once the cap is reached, and follows the newest
// event unless the selection has already been pulled back to read history.
func (m Model) recordActivity(e core.Event) Model {
	following := m.activitySel >= len(m.activity)-1
	m.activity = append(m.activity, e)
	if len(m.activity) > activityCap {
		m.activity = append([]core.Event(nil), m.activity[len(m.activity)-activityCap:]...)
	}
	if following {
		m.activitySel = len(m.activity) - 1
	}
	m.activityOff = ScrollWindow(m.activityOff, m.activitySel, m.activityRows(), len(m.activity))
	return m
}

// activityRows is how many event lines the activity view has room to draw.
func (m Model) activityRows() int {
	return VisibleRows(LayoutFor(m.width, m.height, len(m.columns)).BodyHeight, len(m.activity))
}

// handleActivityKey scrolls the event tail and returns to where it was opened
// from, the same back-navigation every other view follows.
func (m Model) handleActivityKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		return m.leave(nil)
	case key.Matches(msg, m.keys.Up):
		m.activitySel = clamp(m.activitySel-1, 0, len(m.activity)-1)
	case key.Matches(msg, m.keys.Down):
		m.activitySel = clamp(m.activitySel+1, 0, len(m.activity)-1)
	case key.Matches(msg, m.keys.Top):
		m.activitySel = 0
	case key.Matches(msg, m.keys.Bottom):
		m.activitySel = max(0, len(m.activity)-1)
	}
	m.activityOff = ScrollWindow(m.activityOff, m.activitySel, m.activityRows(), len(m.activity))
	return m, nil
}

// activityLines renders the live event tail, scrolled the same way the board
// and the project list are, and says plainly when there is nothing yet or
// when the subscription has actually dropped.
func (m Model) activityLines(layout Layout) []string {
	if empty := ActivityEmptyState(len(m.activity), m.connected); !empty.Zero() {
		return m.emptyLines(empty)
	}
	rows := VisibleRows(layout.BodyHeight, len(m.activity))
	offset := ScrollWindow(m.activityOff, m.activitySel, rows, len(m.activity))
	lines := make([]string, 0, rows+1)
	for i := offset; i < len(m.activity) && len(lines) < rows; i++ {
		lines = append(lines, m.fit(m.eventLine(m.activity[i], i == m.activitySel)))
	}
	if hint := ScrollHint(offset, rows, len(m.activity)); hint != "" {
		lines = append(lines, m.theme.Dim.Render("  "+hint))
	} else if len(m.activity) >= activityCap {
		lines = append(lines, m.theme.Dim.Render(fmt.Sprintf("  keeping the latest %d events; older ones are dropped", activityCap)))
	}
	return lines
}

// eventLine renders one event the same way tix watch draws a line for it:
// a clock time, the actor, what happened, and the task or subject it happened
// to. Sharing internal/output's field extraction with the CLI is what keeps
// the two surfaces from describing the same event two different ways.
func (m Model) eventLine(e core.Event, selected bool) string {
	marker := SelectionMarker(selected)
	when := output.FormatClock(e.OccurredAt)
	actor := output.EventActor(e)
	verb := output.EventVerb(e.Type)
	ref := output.EventRef(e)
	head := marker + when + "  " + actor + "  " + verb + "  " + ref + "  "
	detail := Truncate(output.EventDetail(e), max(0, m.width-len([]rune(head))))
	if selected {
		return m.theme.Selected.Render(Truncate(head+detail, m.width))
	}
	return m.theme.Dim.Render(marker+when) + "  " + actor + "  " + verb + "  " +
		m.theme.Ref.Render(ref) + "  " + detail
}
