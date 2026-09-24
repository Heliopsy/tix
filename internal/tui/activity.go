// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
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
// The selection indexes the events the filter keeps, not the ring, so an
// event the filter hides never moves it.
func (m Model) recordActivity(e core.Event) Model {
	following := m.activitySel >= len(m.shownActivity())-1
	m.activity = append(m.activity, e)
	if len(m.activity) > activityCap {
		m.activity = append([]core.Event(nil), m.activity[len(m.activity)-activityCap:]...)
	}
	return m.reselectActivity(following)
}

// shownActivity is the tail the activity view draws: the events the ring
// holds that the activity filter accepts.
func (m Model) shownActivity() []core.Event { return VisibleEvents(m.activity, m.activityFilter) }

// reselectActivity keeps the selection inside the filtered tail, pinning it
// to the newest line when the view was following it.
func (m Model) reselectActivity(following bool) Model {
	shown := len(m.shownActivity())
	if following {
		m.activitySel = shown - 1
	}
	m.activitySel = clamp(m.activitySel, 0, shown-1)
	m.activityOff = ScrollWindow(m.activityOff, m.activitySel, m.activityRows(), shown)
	return m
}

// activityRows is how many event lines the activity view has room to draw.
func (m Model) activityRows() int {
	return VisibleRows(LayoutFor(m.width, m.height, len(m.columns)).BodyHeight, len(m.shownActivity()))
}

// applyActivityFilterText parses an activity expression, keeping the filter
// already in force when the new one cannot be read.
func (m Model) applyActivityFilterText(text string) Model {
	parsed, err := ParseActivityFilter(text)
	if err != nil {
		m.activityFilterErr = err.Error()
		return m
	}
	m.activityFilterErr, m.activityFilterText, m.activityFilter = "", text, parsed
	return m.reselectActivity(false)
}

// handleActivityKey scrolls the event tail and returns to where it was opened
// from, the same back-navigation every other view follows.
func (m Model) handleActivityKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	shown := len(m.shownActivity())
	switch {
	case key.Matches(msg, m.keys.Back):
		return m.leave(nil)
	case key.Matches(msg, m.keys.Filter):
		return m.startPrompt(promptActivityFilter, m.activityFilterText), textinput.Blink
	case key.Matches(msg, m.keys.ClearFltr):
		return m.applyActivityFilterText(""), nil
	case key.Matches(msg, m.keys.Up):
		m.activitySel = clamp(m.activitySel-1, 0, shown-1)
	case key.Matches(msg, m.keys.Down):
		m.activitySel = clamp(m.activitySel+1, 0, shown-1)
	case key.Matches(msg, m.keys.Top):
		m.activitySel = 0
	case key.Matches(msg, m.keys.Bottom):
		m.activitySel = max(0, shown-1)
	}
	m.activityOff = ScrollWindow(m.activityOff, m.activitySel, m.activityRows(), shown)
	return m, nil
}

// activityLines renders the live event tail, scrolled the same way the board
// and the project list are, and says plainly when there is nothing yet or
// when the subscription has actually dropped.
func (m Model) activityLines(layout Layout) []string {
	shown := m.shownActivity()
	if empty := ActivityEmptyState(len(shown), len(m.activity), m.connected); !empty.Zero() {
		return m.emptyLines(empty)
	}
	rows := VisibleRows(layout.BodyHeight, len(shown))
	offset := ScrollWindow(m.activityOff, m.activitySel, rows, len(shown))
	lines := make([]string, 0, rows+1)
	for i := offset; i < len(shown) && len(lines) < rows; i++ {
		lines = append(lines, m.fit(m.eventLine(shown[i], i == m.activitySel)))
	}
	if hint := ScrollHint(offset, rows, len(shown)); hint != "" {
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

// activityCount is what the status bar says the activity view is holding: the
// size of the tail, and how much of it the filter is keeping when one is in
// force, so a short list is never mistaken for a quiet tenant.
func (m Model) activityCount() string {
	kept := len(m.activity)
	if !m.activityFilter.Active() {
		return fmt.Sprintf("%d events kept, cap %d", kept, activityCap)
	}
	return fmt.Sprintf("%d of %d events match, cap %d", len(m.shownActivity()), kept, activityCap)
}
