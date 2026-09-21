package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// View renders the current frame.
func (m Model) View() string {
	layout := LayoutFor(m.width, m.height, len(m.columns))
	if layout.Mode == LayoutTooSmall {
		return fmt.Sprintf("terminal is %dx%d; tix tui needs at least %dx%d\n", m.width, m.height, MinWidth, MinHeight)
	}
	return strings.Join([]string{
		m.theme.Title.Render(m.headerLine()),
		strings.Join(m.bodyLines(layout), "\n"),
		m.footerLines(),
	}, "\n")
}

// headerLine names the target, the view and the subscription's health.
func (m Model) headerLine() string {
	parts := []string{"tix"}
	if m.actor != nil && m.actor.Handle != "" {
		parts = append(parts, "as "+m.actor.Handle)
	}
	if m.project.Key != "" {
		parts = append(parts, m.project.Key+" ("+m.project.Name+")")
	}
	parts = append(parts, m.connectionLabel())
	if m.filterText != "" {
		parts = append(parts, "filter: "+m.filterText)
	}
	return Truncate(strings.Join(parts, "  |  "), m.width)
}

// connectionLabel states the subscription's health in words.
func (m Model) connectionLabel() string {
	if m.connected {
		return "[live]"
	}
	return "[disconnected, retrying]"
}

// bodyLines renders whichever view is open.
func (m Model) bodyLines(layout Layout) []string {
	switch m.view {
	case viewHelp:
		return m.helpLines()
	case viewProjects:
		return m.projectLines(layout)
	case viewDetail:
		return m.detailLines(layout)
	default:
		return m.boardLines(layout)
	}
}

// projectLines renders the project picker.
func (m Model) projectLines(layout Layout) []string {
	if len(m.projects) == 0 {
		return []string{"", "  No projects yet.", "", "  Create one with:  tix project add KEY \"Name\"", ""}
	}
	lines := make([]string, 0, len(m.projects))
	for i, p := range m.projects {
		label := SelectionMarker(i == m.projectSel) + p.Key + "  " + p.Name
		if p.Archived() {
			label += "  (archived)"
		}
		lines = append(lines, m.emphasize(Truncate(label, m.width), i == m.projectSel))
	}
	return WindowLines(lines, ScrollOffset(0, m.projectSel, layout.BodyHeight), layout.BodyHeight)
}

// boardLines renders the columns that fit, keeping the selection in view.
func (m Model) boardLines(layout Layout) []string {
	if len(m.columns) == 0 {
		return []string{"", "  No tasks match.", ""}
	}
	start, end := VisibleRange(len(m.columns), layout.VisibleColumns, m.sel.Col)
	rendered := make([][]string, 0, end-start)
	for i := start; i < end; i++ {
		rendered = append(rendered, m.columnLines(i, layout))
	}
	return joinColumns(rendered, layout)
}

// columnLines renders one column as a fixed-width block of lines.
func (m Model) columnLines(index int, layout Layout) []string {
	col := m.columns[index]
	head := fmt.Sprintf("%s [%d/%d] (%d)", col.Label, index+1, len(m.columns), len(col.Tasks))
	lines := []string{
		m.theme.Header.Render(pad(Truncate(head, layout.ColumnWidth), layout.ColumnWidth)),
		pad(strings.Repeat("-", min(layout.ColumnWidth, 40)), layout.ColumnWidth),
	}
	height := layout.BodyHeight - 2
	offset := 0
	if index == m.sel.Col {
		offset = ScrollOffset(m.rowOff, m.sel.Row, height)
	}
	cards := cardLines(col, m.now())
	for row, card := range WindowLines(cards, offset, height) {
		selected := index == m.sel.Col && offset+row == m.sel.Row
		lines = append(lines, m.emphasize(pad(Truncate(SelectionMarker(selected)+card, layout.ColumnWidth), layout.ColumnWidth), selected))
	}
	return lines
}

// cardLines renders every task of a column as one plain line each.
func cardLines(col Column, now time.Time) []string {
	out := make([]string, 0, len(col.Tasks))
	for _, t := range col.Tasks {
		out = append(out, t.Ref+" "+strings.Join(TaskBadges(t, now), " ")+" "+t.Title)
	}
	return out
}

// emphasize marks a line as selected without relying on colour.
func (m Model) emphasize(line string, selected bool) string {
	if !selected {
		return line
	}
	return m.theme.Selected.Render(line)
}

// joinColumns lays rendered columns side by side.
func joinColumns(cols [][]string, layout Layout) []string {
	height := 0
	for _, c := range cols {
		if len(c) > height {
			height = len(c)
		}
	}
	out := make([]string, 0, height)
	for row := 0; row < height; row++ {
		parts := make([]string, 0, len(cols))
		for _, c := range cols {
			if row < len(c) {
				parts = append(parts, c[row])
				continue
			}
			parts = append(parts, pad("", layout.ColumnWidth))
		}
		out = append(out, strings.TrimRight(strings.Join(parts, " "), " "))
	}
	return out
}

// detailLines renders the open task in full.
func (m Model) detailLines(layout Layout) []string {
	if m.detail == nil {
		return []string{"  no task open"}
	}
	d := m.detail
	lines := []string{
		m.theme.Header.Render(d.task.Ref + "  " + d.task.Title),
		"status: " + d.task.Status + "   priority: P" + priorityDigit(d.task.Priority),
		"assignee: " + orNone(d.task.AssigneeActorID) + "   creator: " + orNone(d.task.CreatorActorID),
		"claimed: " + claimLabel(d.task, m.now()),
		"due: " + orNone(output.FormatCompactPtr(d.task.DueAt)),
		"created: " + output.FormatCompact(d.task.CreatedAt) + "   updated: " + output.FormatCompact(d.task.UpdatedAt),
		"tags: " + orNone(strings.Join(d.task.Tags, ", ")),
		"",
	}
	lines = append(lines, sectionLines("body", bodyLines(d.task.Body))...)
	lines = append(lines, sectionLines("custom fields", customFieldLines(d.task, d.fieldDefs))...)
	lines = append(lines, sectionLines("subtasks", refLines(d.subtasks))...)
	lines = append(lines, sectionLines("dependencies", dependencyLines(d.deps))...)
	lines = append(lines, sectionLines("comments", commentLines(d.comments))...)
	wrapped := make([]string, 0, len(lines))
	for _, l := range lines {
		wrapped = append(wrapped, Truncate(l, m.width))
	}
	return WindowLines(wrapped, m.detailOff, layout.BodyHeight)
}

// sectionLines titles a block, stating when it is empty.
func sectionLines(title string, body []string) []string {
	if len(body) == 0 {
		return []string{title + ": none", ""}
	}
	return append(append([]string{title + ":"}, body...), "")
}

// bodyLines splits a task body into display lines.
func bodyLines(body string) []string {
	if strings.TrimSpace(body) == "" {
		return nil
	}
	return strings.Split(body, "\n")
}

// customFieldLines renders custom field values in a stable order.
func customFieldLines(t core.Task, defs []core.FieldDef) []string {
	labels := map[string]string{}
	for _, d := range defs {
		labels[d.Key] = d.Label
	}
	keys := make([]string, 0, len(t.CustomFields))
	for k := range t.CustomFields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		label := labels[k]
		if label == "" {
			label = k
		}
		out = append(out, "  "+label+": "+fmt.Sprint(t.CustomFields[k]))
	}
	return out
}

// refLines renders tasks as one reference per line.
func refLines(tasks []core.Task) []string {
	out := make([]string, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, "  "+t.Ref+" ["+t.Status+"] "+t.Title)
	}
	return out
}

// dependencyLines renders what a task waits on.
func dependencyLines(deps []core.Dependency) []string {
	out := make([]string, 0, len(deps))
	for _, d := range deps {
		out = append(out, "  depends on "+d.DependsOn)
	}
	return out
}

// commentLines renders the comment thread.
func commentLines(comments []core.Comment) []string {
	out := make([]string, 0, len(comments)*2)
	for _, c := range comments {
		out = append(out, "  "+output.FormatCompact(c.CreatedAt)+" "+shortID(c.AuthorActorID)+":")
		for _, l := range strings.Split(c.Body, "\n") {
			out = append(out, "    "+l)
		}
	}
	return out
}

// helpLines renders the bindings of the view help was opened from.
func (m Model) helpLines() []string {
	lines := []string{m.theme.Header.Render("help"), ""}
	lines = append(lines, "this view:")
	for _, e := range m.keys.ViewHelp(m.prev) {
		lines = append(lines, "  "+pad(e.Keys, 12)+e.Desc)
	}
	lines = append(lines, "", "everywhere:")
	for _, e := range m.keys.GlobalHelp() {
		lines = append(lines, "  "+pad(e.Keys, 12)+e.Desc)
	}
	lines = append(lines, "", "filter bar accepts: "+strings.Join(FilterKeys, ", "))
	return lines
}

// footerLines renders the prompt, the status and the advertised bindings.
func (m Model) footerLines() string {
	var lines []string
	switch {
	case m.editing:
		lines = append(lines, m.input.View())
	case m.choosing:
		lines = append(lines, "transition to: "+strings.Join(choiceLabels(m.choices), "  ")+"   (esc cancels)")
	}
	if m.filterErr != "" {
		lines = append(lines, m.theme.Error.Render("filter error: "+Truncate(m.filterErr, m.width)))
	}
	if m.err != "" {
		lines = append(lines, m.theme.Error.Render("error: "+Truncate(m.err, m.width)))
	} else if m.status != "" {
		lines = append(lines, m.theme.Status.Render(Truncate(m.status, m.width)))
	}
	keys := make([]string, 0, 8)
	for _, e := range m.keys.ShortHelp(m.view) {
		keys = append(keys, e.Keys+" "+e.Desc)
	}
	lines = append(lines, m.theme.Dim.Render(Truncate(strings.Join(keys, "  "), m.width)))
	return strings.Join(lines, "\n")
}

// choiceLabels numbers the transition targets on offer.
func choiceLabels(states []core.State) []string {
	out := make([]string, 0, len(states))
	for i, s := range states {
		label := s.Label
		if label == "" {
			label = s.Key
		}
		out = append(out, strconv.Itoa(i+1)+") "+label)
	}
	return out
}

// claimLabel states who holds a task, in words rather than colour.
func claimLabel(t core.Task, now time.Time) string {
	if !t.ClaimedAtTime(now) {
		return "no"
	}
	return "yes, by " + t.ClaimedByActorID + " until " + output.FormatCompactPtr(t.LeaseExpiresAt)
}

// orNone renders an empty value as a word.
func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "none"
	}
	return s
}

// pad right-pads s to width.
func pad(s string, width int) string {
	if n := width - len([]rune(s)); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}
