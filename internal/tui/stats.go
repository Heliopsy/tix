// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heliopsy/tix/internal/core"
)

// openStats shows the numbers for the project in focus, or for the whole
// tenant when the board is not open on one.
//
// The figures are fetched rather than derived from the tasks already loaded.
// The board holds one project's non-terminal work; throughput is a count of
// what left, over a window, which is a question only the store can answer.
func (m Model) openStats() (Model, tea.Cmd) {
	m = m.enterView(viewStats)
	m.stats = nil
	m.statsErr = nil
	return m, m.loadStats()
}

// loadStats reads the window the view is set to.
func (m Model) loadStats() tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx := m.svc, m.ctx
	in := core.StatsInput{Window: core.Duration(m.statsDays()) * core.Duration(dayNanos)}
	// The board's project narrows the read, so opening statistics from a
	// project answers "how is this project doing" rather than burying it in
	// the tenant's total.
	if m.project.ID != "" {
		in.ProjectRef = m.project.Key
	}
	return func() tea.Msg {
		s, err := svc.Stats(ctx, in)
		if err != nil {
			return statsMsg{err: err}
		}
		return statsMsg{stats: s}
	}
}

// dayNanos is one day, as the nanosecond count core.Duration carries.
const dayNanos = 24 * 60 * 60 * 1e9

// statsWindowDays are the windows the view cycles through.
var statsWindowDays = []int{7, 14, 30, 90}

// statsDays is the window presently selected.
func (m Model) statsDays() int {
	if m.statsWindow < 0 || m.statsWindow >= len(statsWindowDays) {
		return statsWindowDays[1]
	}
	return statsWindowDays[m.statsWindow]
}

// cycleStatsWindow moves to the next window and refetches.
func (m Model) cycleStatsWindow() (Model, tea.Cmd) {
	m.statsWindow = (m.statsWindow + 1) % len(statsWindowDays)
	m.stats = nil
	return m, m.loadStats()
}

// statsLines renders the statistics view.
func (m Model) statsLines(layout Layout) []string {
	head := fmt.Sprintf("statistics, last %d days", m.statsDays())
	if m.project.ID != "" {
		head += "  " + m.project.Key
	}
	lines := []string{m.theme.Header.Render(head), ""}

	switch {
	case m.statsErr != nil:
		return append(lines, m.theme.Error.Render("  "+m.statsErr.Error()))
	case m.stats == nil:
		return append(lines, m.theme.Dim.Render("  reading..."))
	}
	s := m.stats

	lines = append(lines,
		fmt.Sprintf("  %-18s %d", "completed", s.Completed),
		fmt.Sprintf("  %-18s %d", "created", s.Created),
		fmt.Sprintf("  %-18s %s", "median lead time", s.MedianLeadTime.Human()),
		fmt.Sprintf("  %-18s %s", "slowest", s.SlowestLeadTime.Human()),
		"",
	)

	lines = append(lines, m.theme.Header.Render("  completed per day"))
	if max := maxCompleted(s.PerDay); max > 0 {
		for _, d := range s.PerDay {
			lines = append(lines, fmt.Sprintf("  %-11s %s %3d",
				d.Date, m.theme.Bar.Render(sparkBar(d.Completed, max, 24)), d.Completed))
		}
	} else {
		lines = append(lines, m.theme.Empty.Render("  nothing completed in this window"))
	}
	lines = append(lines, "")

	lines = append(lines, m.theme.Header.Render("  where the work is"))
	if len(s.ByCategory) == 0 {
		lines = append(lines, m.theme.Empty.Render("  no tasks yet"))
	}
	for _, c := range s.ByCategory {
		lines = append(lines, fmt.Sprintf("  %-18s %d", c.Category, c.Count))
	}
	lines = append(lines, "")

	// The measure is printed every time the leaderboard is, and it comes from
	// core so this wording cannot drift from the web screen's or the CLI's.
	lines = append(lines, m.theme.Header.Render("  most active"),
		m.theme.Dim.Render("  counting "+s.LeaderboardMeasure))
	if len(s.TopActors) == 0 {
		lines = append(lines, m.theme.Empty.Render("  nobody moved a task to a terminal state"))
	}
	for _, a := range s.TopActors {
		who := a.Handle
		if who == "" {
			who = a.ActorID
		}
		lines = append(lines, fmt.Sprintf("  %-24s %d", Truncate(who, 24), a.Moved))
	}
	lines = append(lines, "")

	lines = append(lines, m.theme.Header.Render("  waiting longest"))
	if len(s.Oldest) == 0 {
		lines = append(lines, m.theme.Empty.Render("  nothing is waiting"))
	}
	for _, o := range s.Oldest {
		lines = append(lines, fmt.Sprintf("  %s  %-*s %s",
			m.theme.Ref.Render(o.Ref), 34, Truncate(o.Title, 34), o.Age.Human()))
	}

	rows := VisibleRows(layout.BodyHeight, len(lines))
	off := ScrollWindow(m.statsOff, m.statsOff, rows, len(lines))
	windowed := WindowLines(lines, off, rows)
	if hint := ScrollHint(off, rows, len(lines)); hint != "" {
		windowed = append(windowed, m.theme.Dim.Render(hint))
	}
	out := make([]string, 0, len(windowed))
	for _, l := range windowed {
		out = append(out, m.fit(l))
	}
	return out
}

// sparkBar draws one bar as block characters.
//
// Blocks rather than a plotting library: a terminal has one glyph width to
// work with, and six numbers do not justify a dependency. A non-zero count
// always gets at least one block, so "one completed" is visibly different
// from none rather than rounding to an empty row.
func sparkBar(v, max, width int) string {
	if v <= 0 || max <= 0 || width <= 0 {
		return ""
	}
	n := v * width / max
	if n < 1 {
		n = 1
	}
	return strings.Repeat("█", n)
}

// maxCompleted is the tallest day in the series.
func maxCompleted(days []core.StatsDay) int {
	max := 0
	for _, d := range days {
		if d.Completed > max {
			max = d.Completed
		}
	}
	return max
}

// handleStatsKey scrolls the view and cycles the window.
func (m Model) handleStatsKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Down):
		m.statsOff++
	case key.Matches(msg, m.keys.Up):
		m.statsOff = clamp(m.statsOff-1, 0, m.statsOff)
	case key.Matches(msg, m.keys.Filter):
		// The filter key cycles the window here. A statistics view has one
		// thing worth narrowing and it is the period, so rather than open a
		// text prompt for a single number, the key that means "narrow this"
		// on every other view steps through the windows.
		return m.cycleStatsWindow()
	case key.Matches(msg, m.keys.Refresh):
		m.stats = nil
		return m, m.loadStats()
	}
	return m, nil
}

// applyStats installs a statistics read, or the error that came back instead.
func (m Model) applyStats(msg statsMsg) Model {
	m.stats, m.statsErr = msg.stats, msg.err
	if msg.err != nil {
		m.stats = nil
	}
	m.statsOff = 0
	return m
}
