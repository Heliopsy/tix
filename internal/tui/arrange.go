// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"
	"fmt"
	"sort"

	tea "charm.land/bubbletea/v2"
	"github.com/heliopsy/tix/internal/core"
)

// boardView is the view a project's board is arranged under, the same one
// the web board of that project reads, so a column hidden in either place is
// hidden in both.
func boardView(projectKey string) string { return core.ProjectView(projectKey) }

// ArrangeColumns draws the reader's arrangement over the workflow's columns:
// hidden ones are left out and named, starred ones lead, then the ones the
// reader positioned in their order, then the rest in workflow order, and a
// collapsed one keeps its count but not its cards.
func ArrangeColumns(cols []Column, prefs []core.GroupPref) ([]Column, []string) {
	byKey := make(map[string]core.GroupPref, len(prefs))
	for _, p := range prefs {
		byKey[p.Group] = p
	}
	type ranked struct {
		col   Column
		pos   int
		index int
	}
	drawn := make([]ranked, 0, len(cols))
	var hidden []string
	for i, c := range cols {
		p, ok := byKey[c.Key]
		if !ok || !c.Declared {
			drawn = append(drawn, ranked{col: c, index: i})
			continue
		}
		if p.Hidden {
			hidden = append(hidden, c.Key)
			continue
		}
		c.Starred = p.Starred
		if p.Collapsed {
			c.Collapsed, c.Folded, c.Tasks = true, len(c.Tasks), nil
		}
		drawn = append(drawn, ranked{col: c, pos: p.Position, index: i})
	}
	sort.SliceStable(drawn, func(i, j int) bool {
		a, b := drawn[i], drawn[j]
		if a.col.Starred != b.col.Starred {
			return a.col.Starred
		}
		if (a.pos > 0) != (b.pos > 0) {
			return a.pos > 0
		}
		if a.pos != b.pos {
			return a.pos < b.pos
		}
		return a.index < b.index
	})
	out := make([]Column, len(drawn))
	for i, r := range drawn {
		out[i] = r.col
	}
	return out, hidden
}

// ColumnNeighbour is the index the column at i swaps with when moved by step,
// or -1 when there is none: past either end, across the line between starred
// and unstarred columns, or onto a column the arrangement cannot name.
func ColumnNeighbour(cols []Column, i, step int) int {
	j := i + step
	if i < 0 || i >= len(cols) || j < 0 || j >= len(cols) {
		return -1
	}
	if !cols[i].Declared || !cols[j].Declared || cols[i].Starred != cols[j].Starred {
		return -1
	}
	return j
}

// SwappedOrder is the drawn order of the declared columns after the column at
// i trades places with the one at j.
func SwappedOrder(cols []Column, i, j int) []string {
	keys := make([]string, 0, len(cols))
	for k := range cols {
		switch k {
		case i:
			keys = append(keys, cols[j].Key)
		case j:
			keys = append(keys, cols[i].Key)
		default:
			if cols[k].Declared {
				keys = append(keys, cols[k].Key)
			}
		}
	}
	return keys
}

// arranges reports whether this reader may read their arrangement at all. A
// reader who may not gets the board in workflow order, as before.
func (m Model) arranges() bool {
	may := m.permits()
	return may != nil && may("ListGroupPrefs")
}

// readArrangement reads the reader's arrangement of a project's board. A
// failure leaves the board in workflow order rather than refusing to draw
// it: the arrangement is a preference about the board, not the board.
func readArrangement(ctx context.Context, svc core.Service, projectKey string, arranges bool) ([]core.GroupPref, bool) {
	if !arranges {
		return nil, false
	}
	prefs, err := svc.ListGroupPrefs(ctx, boardView(projectKey))
	if err != nil {
		return nil, false
	}
	return prefs, true
}

// drawColumns builds the board's columns from the tasks the filter keeps and
// lays the reader's arrangement over them.
func (m Model) drawColumns() Model {
	m.columns, m.hiddenCols = ArrangeColumns(BuildColumns(m.workflow, m.visibleTasks(m.tasks)), m.arrangement)
	return m
}

// followColumn selects the column with the given key, keeping the selection
// where it was when no drawn column has it.
func followColumn(cols []Column, sel Selection, key string) Selection {
	for i, c := range cols {
		if c.Key == key {
			if i == sel.Col {
				return sel
			}
			return ClampSelection(cols, Selection{Col: i})
		}
	}
	return sel
}

// selectedColumn is the column under the board's selection that the
// arrangement can name.
func (m Model) selectedColumn() (Column, bool) {
	if m.view != viewBoard || m.sel.Col < 0 || m.sel.Col >= len(m.columns) {
		return Column{}, false
	}
	c := m.columns[m.sel.Col]
	return c, c.Declared
}

// arrangeColumn changes the reader's preference on one column of this board.
func (m Model) arrangeColumn(col Column, in core.GroupPrefInput, sentence string) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx, view := m.svc, m.ctx, boardView(m.project.Key)
	return func() tea.Msg {
		_, err := svc.SetGroupPref(ctx, view, col.Key, in)
		return actionMsg{kind: actionArrange, label: col.Label, sentence: sentence, err: err}
	}
}

// moveColumn trades the selected column's place with its neighbour.
func (m Model) moveColumn(step int) (Model, tea.Cmd) {
	j := ColumnNeighbour(m.columns, m.sel.Col, step)
	if j < 0 || m.svc == nil {
		return m, nil
	}
	col := m.columns[m.sel.Col]
	order := SwappedOrder(m.columns, m.sel.Col, j)
	svc, ctx, view := m.svc, m.ctx, boardView(m.project.Key)
	side := "right"
	if step < 0 {
		side = "left"
	}
	m.followCol = col.Key
	return m, func() tea.Msg {
		_, err := svc.OrderGroups(ctx, view, core.GroupOrderInput{Groups: order})
		return actionMsg{kind: actionArrange, label: col.Label, sentence: "moved " + col.Label + " " + side, err: err}
	}
}

// showColumns brings every hidden column back, one preference at a time, so
// a column that was also starred or moved keeps that.
func (m Model) showColumns() tea.Cmd {
	if m.svc == nil || len(m.hiddenCols) == 0 {
		return nil
	}
	svc, ctx, view, keys := m.svc, m.ctx, boardView(m.project.Key), append([]string(nil), m.hiddenCols...)
	shown := false
	sentence := fmt.Sprintf("showed %d hidden column", len(keys))
	if len(keys) != 1 {
		sentence += "s"
	}
	return func() tea.Msg {
		for _, k := range keys {
			if _, err := svc.SetGroupPref(ctx, view, k, core.GroupPrefInput{Hidden: &shown}); err != nil {
				return actionMsg{kind: actionArrange, label: k, err: err}
			}
		}
		return actionMsg{kind: actionArrange, sentence: sentence}
	}
}

// columnAction performs one of the board's column actions.
func (m Model) columnAction(id actionID) (Model, tea.Cmd, bool) {
	if id == doShowColumns {
		return m, m.showColumns(), true
	}
	col, ok := m.selectedColumn()
	if !ok {
		return m, nil, true
	}
	switch id {
	case doHideColumn:
		hidden := true
		return m, m.arrangeColumn(col, core.GroupPrefInput{Hidden: &hidden}, "hid "+col.Label), true
	case doStarColumn:
		star := !col.Starred
		verb := "starred "
		if !star {
			verb = "unstarred "
		}
		return m, m.arrangeColumn(col, core.GroupPrefInput{Starred: &star}, verb+col.Label), true
	case doFoldColumn:
		fold := !col.Collapsed
		verb := "collapsed "
		if !fold {
			verb = "expanded "
		}
		return m, m.arrangeColumn(col, core.GroupPrefInput{Collapsed: &fold}, verb+col.Label), true
	case doColumnLeft:
		next, cmd := m.moveColumn(-1)
		return next, cmd, true
	case doColumnRight:
		next, cmd := m.moveColumn(1)
		return next, cmd, true
	}
	return m, nil, false
}
