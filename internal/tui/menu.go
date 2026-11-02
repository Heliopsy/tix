// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// openMenu shows the start menu, refreshing the projects its Watch row picks
// from on the way.
func (m Model) openMenu() Model { return m.enterView(viewMenu) }

// menuRows are the start menu's rows: one per registered interface, in the
// registry's order.
func (m Model) menuRows() []Interface { return Interfaces() }

// menuProjectLabel names what the Watch row is set to watch.
func (m Model) menuProjectLabel() string {
	if m.menuProject <= 0 || m.menuProject > len(m.projects) {
		return "all projects"
	}
	return output.ShowKey(m.projects[m.menuProject-1].Key)
}

// handleMenuKey moves between the rows, picks a project for Watch, and opens
// the selected interface one level down, so back returns here.
func (m Model) handleMenuKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	rows := m.menuRows()
	watching := m.menuSel < len(rows) && rows[m.menuSel].opens == viewActivity
	switch {
	case key.Matches(msg, m.keys.Up):
		m.menuSel = clamp(m.menuSel-1, 0, len(rows)-1)
	case key.Matches(msg, m.keys.Down):
		m.menuSel = clamp(m.menuSel+1, 0, len(rows)-1)
	case key.Matches(msg, m.keys.Top):
		m.menuSel = 0
	case key.Matches(msg, m.keys.Bottom):
		m.menuSel = len(rows) - 1
	case watching && key.Matches(msg, m.keys.Right):
		m.menuProject = (m.menuProject + 1) % (len(m.projects) + 1)
	case watching && key.Matches(msg, m.keys.Left):
		m.menuProject = (m.menuProject + len(m.projects)) % (len(m.projects) + 1)
	case key.Matches(msg, m.keys.Enter):
		return m.openInterface(rows[m.menuSel])
	}
	return m, nil
}

// openInterface enters the view a menu row stands for. Watch carries the
// project its row picked; the activity filter typed with / stays as it was.
func (m Model) openInterface(i Interface) (Model, tea.Cmd) {
	if i.opens == viewActivity {
		m.activityProject = m.menuPicked()
		m = m.reselectActivity(true)
	}
	m.err = ""
	return m.enterView(i.opens), nil
}

// menuPicked is the project the Watch row is set to, or none for every one.
func (m Model) menuPicked() core.Project {
	if m.menuProject > 0 && m.menuProject <= len(m.projects) {
		return m.projects[m.menuProject-1]
	}
	return core.Project{}
}

// menuLines renders one row per interface, the Watch row carrying its picker.
func (m Model) menuLines() []string {
	lines := []string{m.theme.Header.Render("Open an interface"), ""}
	for i, row := range m.menuRows() {
		selected := i == m.menuSel
		line := SelectionMarker(selected) + pad(row.Title, 8) + "  " + row.Summary
		if row.opens == viewActivity {
			line += "  [" + m.menuProjectLabel() + "]"
		}
		if !m.canReach(row.opens) {
			line += "  (not offered to you)"
		}
		if selected {
			lines = append(lines, m.fit(m.selection().Render(line)))
			continue
		}
		lines = append(lines, m.fit(line))
	}
	lines = append(lines, "", m.theme.Dim.Render("  ~ returns here from anywhere"))
	return lines
}
