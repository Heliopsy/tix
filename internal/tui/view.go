// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/query"
)

// View renders the current frame onto the alternate screen, which is where the
// interface has always drawn: bubbletea takes the screen from the view rather
// than from a program option.
func (m Model) View() tea.View {
	view := tea.NewView(m.Frame())
	view.AltScreen = true
	return view
}

// Frame renders the current frame.
func (m Model) Frame() string {
	layout := LayoutFor(m.width, m.height, len(m.columns))
	if layout.Mode == LayoutTooSmall {
		return fmt.Sprintf("terminal is %dx%d; tix tui needs at least %dx%d\n", m.width, m.height, MinWidth, MinHeight)
	}
	return strings.Join([]string{
		m.titleBar(),
		strings.Join(m.bodyLines(layout), "\n"),
		m.footerLines(),
	}, "\n")
}

// titleBar names the target and the view, carrying the project's own colour
// and icon so one board is never mistaken for another.
func (m Model) titleBar() string {
	parts := []string{m.theme.Title.Render("tix")}
	if m.actor != nil && m.actor.Handle != "" {
		parts = append(parts, m.theme.Dim.Render("as "+m.actor.Handle))
	}
	if m.tenantKey != "" {
		parts = append(parts, m.theme.Dim.Render("@"+m.tenantKey))
	}
	if m.project.Key != "" {
		parts = append(parts, m.projectLabel(m.project))
	}
	// Which view is open, before the connection state. Nothing on screen said
	// this: projects, the board, activity, the tenant screen and statistics
	// all rendered into the same frame under the same title, so the only way
	// to know where you were was to recognise the body, and the only way to
	// know a key did anything was that the body changed.
	parts = append(parts, m.theme.Header.Render(m.viewName()))
	parts = append(parts, m.connectionLabel())
	if m.view == viewActivity && m.activityFilterText != "" {
		parts = append(parts, m.theme.Ref.Render("activity: "+m.activityFilterText))
	} else if m.filterText != "" {
		parts = append(parts, m.theme.Ref.Render("filter: "+m.filterText))
	}
	return m.fit(strings.Join(parts, m.theme.Bar.Render(" │ ")))
}

// viewName is the open view, for the title bar.
func (m Model) viewName() string {
	switch m.view {
	case viewProjects:
		return "projects"
	case viewBoard:
		return "board"
	case viewDetail:
		return "task"
	case viewActivity:
		return "activity"
	case viewTenant:
		return "tenant"
	case viewStats:
		return "statistics"
	case viewSettings:
		return "settings"
	case viewProject:
		return "project"
	case viewHistory:
		return "history"
	case viewHelp:
		return "help"
	}
	return ""
}

// projectLabel renders a project with its icon and its palette colour.
func (m Model) projectLabel(p core.Project) string {
	label := m.theme.Project(p.Color).Render(ProjectGlyph(p) + " " + p.Key)
	if p.Name != "" {
		label += m.theme.Dim.Render(" " + p.Name)
	}
	return label
}

// connectionLabel states the subscription's health in words as well as colour.
func (m Model) connectionLabel() string {
	if m.connected {
		return m.theme.Status.Render("● live")
	}
	return m.theme.Error.Render("○ disconnected, retrying")
}

// fit truncates a styled line to the terminal's width.
func (m Model) fit(line string) string {
	if m.width <= 0 || lipgloss.Width(line) <= m.width {
		return line
	}
	return m.theme.Style().MaxWidth(m.width).Render(line)
}

// bodyLines renders whichever view is open.
func (m Model) bodyLines(layout Layout) []string {
	switch m.view {
	case viewHelp:
		return m.helpLines(layout)
	case viewProjects:
		return m.projectLines()
	case viewDetail:
		return m.detailLines(layout)
	case viewSettings:
		return m.settingsLines(layout)
	case viewActivity:
		return m.activityLines(layout)
	case viewTenant:
		return m.tenantLines(layout)
	case viewStats:
		return m.statsLines(layout)
	case viewProject:
		return m.setupLines(layout)
	case viewHistory:
		return m.historyLines(layout)
	default:
		return m.boardLines(layout)
	}
}

// projectLines renders the project picker, scrolled so the selection is always
// on screen and so a list that runs past the terminal says that it does.
func (m Model) projectLines() []string {
	if empty := ProjectsEmptyState(len(m.projects)); !empty.Zero() {
		return m.emptyLines(empty)
	}
	// A count, because a scrolling list with no total cannot tell you whether
	// you have seen all of it.
	head := []string{m.theme.Header.Render(fmt.Sprintf("  %d projects", len(m.projects))), ""}
	rows := m.projectRows()
	offset := ScrollWindow(m.projectOff, m.projectSel, rows, len(m.projects))
	lines := make([]string, 0, rows+1)
	for i := offset; i < len(m.projects) && len(lines) < rows; i++ {
		lines = append(lines, m.projectRow(m.projects[i], i == m.projectSel))
	}
	if hint := ScrollHint(offset, rows, len(m.projects)); hint != "" {
		lines = append(lines, m.theme.Dim.Render("  "+hint))
	}
	return append(head, lines...)
}

// settingsLines renders the display preferences and the session facts,
// scrolled so the selected setting is always on screen.
func (m Model) settingsLines(layout Layout) []string {
	raw := SettingsView(m.settingsState())
	rows := VisibleRows(layout.BodyHeight, len(raw))
	window, offset := SettingsWindow(raw, m.settingsOff, rows)
	out := make([]string, 0, len(window)+1)
	for _, l := range window {
		out = append(out, m.fit(m.settingsStyle(l)))
	}
	if hint := ScrollHint(offset, rows, len(raw)); hint != "" {
		out = append(out, m.theme.Dim.Render("  "+hint))
	}
	return out
}

// setupLines renders the project screen, scrolled so a workflow with more states
// than the terminal has rows is still reachable and says that it has more.
func (m Model) setupLines(layout Layout) []string {
	if m.setup == nil {
		return []string{"  no project open"}
	}
	raw := ProjectView(m.setupState())
	rows := VisibleRows(layout.BodyHeight, len(raw))
	offset := ScrollWindow(m.setupOff, m.setupOff, rows, len(raw))
	out := make([]string, 0, rows+1)
	for i := offset; i < len(raw) && i < offset+rows; i++ {
		out = append(out, m.fit(m.setupStyle(raw[i])))
	}
	if hint := ScrollHint(offset, rows, len(raw)); hint != "" {
		out = append(out, m.theme.Dim.Render("  "+hint))
	}
	return out
}

// setupStyle draws one project line by what it is.
func (m Model) setupStyle(l ProjectLine) string {
	switch {
	case l.Heading:
		return m.theme.Header.Render(l.Text)
	case l.Dim:
		return m.theme.Dim.Render(l.Text)
	default:
		return l.Text
	}
}

// settingsStyle draws one settings line by what it is.
func (m Model) settingsStyle(l SettingsLine) string {
	switch {
	case l.Heading:
		return m.theme.Header.Render(l.Text)
	case l.Row == m.settingSel && !l.Dim:
		return m.selection().Render(l.Text)
	case l.Dim:
		return m.theme.Dim.Render(l.Text)
	default:
		return l.Text
	}
}

// projectRow renders one project of the picker.
func (m Model) projectRow(p core.Project, selected bool) string {
	label := SelectionMarker(selected) + ProjectGlyph(p) + " " + p.Key + "  " + p.Name
	if p.Archived() {
		label += "  (archived)"
	}
	label = Truncate(label, m.width)
	if selected {
		return m.selection().Render(label)
	}
	return m.theme.Project(p.Color).Render(label)
}

// emptyLines fills a body with an empty state, centred enough to read as a
// message rather than as a failure.
func (m Model) emptyLines(state EmptyState) []string {
	lines := make([]string, 0, 4)
	for i, l := range state.Lines() {
		if i == 1 {
			lines = append(lines, m.theme.Header.Render(l))
			continue
		}
		lines = append(lines, m.theme.Empty.Render(l))
	}
	return lines
}

// boardLines renders the columns that fit, keeping the selection in view.
func (m Model) boardLines(layout Layout) []string {
	empty := BoardEmptyState(m.project.Key, m.columns, m.filterText != "")
	if !empty.Zero() {
		return m.emptyLines(empty)
	}
	start, end := VisibleRange(len(m.columns), layout.VisibleColumns, m.sel.Col)
	widths := m.columnWidths(layout, start, end)
	blocks := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		sized := layout
		sized.ColumnWidth = widths[i-start]
		blocks = append(blocks, m.columnBlock(i, sized, i < end-1))
	}
	return strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, blocks...), "\n")
}

// columnWidths shares the board's width among the columns on screen by what
// each has to show. An equal split spent as many cells on a column holding the
// word "empty" as on the column holding the work.
func (m Model) columnWidths(layout Layout, start, end int) []int {
	n := end - start
	if n <= 0 {
		return nil
	}
	demands, floors := make([]int, n), make([]int, n)
	for i := start; i < end; i++ {
		head := ColumnHeadingText(m.columns[i])
		demands[i-start] = ColumnDemand(head, m.columns[i].Tasks)
		floors[i-start] = ColumnFloor(head, m.columns[i].Tasks)
	}
	return DistributeWidth(layout.Width-(n-1)*CardGap, demands, floors)
}

// columnBlock renders one column inside its own border. It is drawn to the
// height of what it holds rather than to the height of the terminal, so an
// empty column is a short box instead of a screenful of blank frame.
func (m Model) columnBlock(index int, layout Layout, gap bool) string {
	col := m.columns[index]
	focused := index == m.sel.Col
	inner := max(1, layout.InnerWidth()-ColumnGutter)
	body := max(1, layout.BodyHeight-BorderHeight)
	lines := append([]string{m.columnHeading(col, inner)}, m.cardBlock(col, focused, inner, max(1, body-1))...)
	// A cell of air between the column's border and the card's own bar, so the
	// two vertical strokes are not drawn against each other.
	for i := range lines {
		lines[i] = strings.Repeat(" ", ColumnGutter) + lines[i]
	}

	style := m.theme.Column
	if focused {
		style = m.theme.Focused
	}
	if gap {
		style = style.MarginRight(CardGap)
	}
	// Width and Height are the outside of the box, border included, so the
	// column is given its whole share rather than its share less the frame.
	return style.Width(layout.ColumnWidth).
		Height(clamp(len(lines)+BorderHeight, 1+BorderHeight, layout.BodyHeight)).
		Render(strings.Join(lines, "\n"))
}

// cardBlock renders the cards of one column into a line budget, separated by a
// blank line, with the scroll hint a window that hides cards owes the reader.
func (m Model) cardBlock(col Column, focused bool, width, rows int) []string {
	if len(col.Tasks) == 0 {
		return []string{m.quiet(m.theme.Empty).Render(Truncate("empty", width))}
	}
	heights := CardHeights(col.Tasks, width, m.now(), m.actorID())
	offset, selected := 0, 0
	if focused {
		offset, selected = m.rowOff, m.sel.Row
	}
	start, count := CardWindow(heights, offset, selected, rows)
	if start > 0 || start+count < len(col.Tasks) {
		start, count = CardWindow(heights, offset, selected, max(1, rows-1))
	}
	out := make([]string, 0, rows)
	for i := start; i < start+count && i < len(col.Tasks); i++ {
		if i > start {
			out = append(out, "")
		}
		out = append(out, m.cardLines(col.Tasks[i], col.Category, width, focused && i == m.sel.Row)...)
	}
	if hint := ScrollHint(start, count, len(col.Tasks)); hint != "" {
		out = append(out, m.quiet(m.theme.Dim).Render(Truncate(hint, width)))
	}
	return out
}

// ColumnHeadingText names a column and counts it.
func ColumnHeadingText(col Column) string {
	return col.Label + " (" + fmt.Sprint(len(col.Tasks)) + ")"
}

// columnHeading names a column and counts it, coloured by the category its
// state belongs to rather than by its name, because workflows are user defined.
// It does not number the column: which slice of a wide board is on screen is
// said once in the status bar rather than repeated in every heading.
func (m Model) columnHeading(col Column, width int) string {
	return m.quiet(m.theme.Category(col.Category).Bold(m.theme.Color)).
		Render(Truncate(ColumnHeadingText(col), width))
}

// cardLines renders one task as the board draws it: a bar down the left edge
// carrying the state, the title in full width beside it, and the identifiers
// dim beneath. The title leads because the title is what a person scans for,
// and it wraps onto a second line indented to the same column rather than into
// the unindented fragment it used to leave.
//
// Selection is a heavier bar as well as a brighter colour, so a reader whose
// terminal is getting no escapes at all can still see which card is selected.
func (m Model) cardLines(t core.Task, category core.StateCategory, width int, selected bool) []string {
	card := CardOf(t, width, m.now(), m.actorID())
	bar := CardBar(selected)
	barStyle := m.quiet(m.theme.Category(category))
	titleStyle := m.quiet(m.theme.Style())
	if selected {
		barStyle, titleStyle = m.quiet(m.selection()), m.quiet(m.selection())
	}
	out := make([]string, 0, len(card.Title)+1)
	for _, line := range card.Title {
		out = append(out, barStyle.Render(bar)+" "+titleStyle.Render(line))
	}
	return append(out, barStyle.Render(bar)+" "+m.quiet(m.theme.Dim).Render(card.Meta))
}

// quiet drops a style to the dim one while an input mode owns the screen, so
// the board behind a prompt reads as the background it has become. It is never
// the only signal that input is being captured: the panel and the key legend
// below it say so on a terminal with no colour at all.
func (m Model) quiet(style lipgloss.Style) lipgloss.Style {
	if m.inputOpen() {
		return m.theme.Dim
	}
	return style
}

// inputOpen reports whether a prompt, a picker, a confirmation or a form is
// gathering an answer.
func (m Model) inputOpen() bool {
	return m.prompt != promptNone || m.choice != choiceNone || m.confirm.Open() || m.form.Open()
}

// DeletedMarker is what a card carries once the task behind it is deleted.
const DeletedMarker = "†"

// CardFlags renders a card's state markers compactly, so a narrow column still
// says what is true of a task. The help view carries the legend.
//
// mine is the actor reading the board. A claimed card said only that somebody
// held it, which on a shared board leaves the first question anyone asks, "is
// that me", answerable only by opening the task. A card held by the reader is
// marked differently, and an empty mine degrades to the old behaviour rather
// than guessing.
func CardFlags(t core.Task, now time.Time, mine string) string {
	var b strings.Builder
	if t.ClaimedAtTime(now) {
		if mine != "" && t.ClaimedByActorID == mine {
			b.WriteString("@me")
		} else {
			b.WriteString("@")
		}
	}
	if t.Blocked {
		b.WriteString("!")
	}
	if len(t.DependsOn) > 0 {
		b.WriteString("+")
	}
	if t.DueAt != nil {
		b.WriteString("*")
	}
	// A deleted task is drawn only where a filter asked for one, and an
	// unmarked card there is indistinguishable from live work.
	if t.DeletedAt != nil {
		b.WriteString(DeletedMarker)
	}
	return b.String()
}

// detailLines renders the open task in full: the identity of the task, then
// the ideas its fields group into, then whatever sections have content. A
// task with nothing in a section spends no line saying so, so an empty task
// reads as short rather than as twelve lines of "none".
func (m Model) detailLines(layout Layout) []string {
	if m.detail == nil {
		return []string{"  no task open"}
	}
	d := m.detail
	t := d.task
	category := output.StateCategoryOf(t.Status)
	if m.workflow != nil {
		if s, ok := m.workflow.State(t.Status); ok {
			category = s.Category
		}
	}
	now := m.now()
	claimed := t.ClaimedAtTime(now)
	claim := claimSummary(t, now, leaseTimeStyle(), d.actors)

	lines := []string{
		m.theme.Ref.Render(t.Ref) + "  " + m.theme.Title.Render(t.Title),
		stateLine(m.theme, t.Status, category, t.Priority, claim, claimed),
		m.theme.Dim.Render(peopleText(t, d.actors)),
		m.theme.Dim.Render(timeText(t, m.timeStyle)),
	}
	if tags := tagsText(t.Tags); tags != "" {
		lines = append(lines, m.theme.Dim.Render(tags))
	}
	lines = append(lines, "")

	body := bodyLines(t.Body, max(1, m.width-DetailIndent))
	fields := customFieldLines(t, d.fieldDefs)
	comments := commentLines(d.comments, d.actors, m.timeStyle, m.commentSel, max(1, m.width-CommentIndent))
	if len(body) == 0 && len(fields) == 0 && len(d.subtasks) == 0 && len(d.deps) == 0 && len(d.artifacts) == 0 && len(d.comments) == 0 {
		lines = append(lines, m.theme.Empty.Render("  nothing else recorded on this task."))
	} else {
		lines = append(lines, m.section("body", body)...)
		lines = append(lines, m.section(fmt.Sprintf("custom fields (%d)", len(fields)), fields)...)
		lines = append(lines, m.section(fmt.Sprintf("subtasks (%d)", len(d.subtasks)), refLines(d.subtasks))...)
		lines = append(lines, m.section(fmt.Sprintf("dependencies (%d)", len(d.deps)), dependencyLines(d.deps))...)
		lines = append(lines, m.section(fmt.Sprintf("artifacts (%d)", len(d.artifacts)), artifactLines(d.artifacts))...)
		lines = append(lines, m.section(fmt.Sprintf("comments (%d)", len(d.comments)), comments)...)
	}

	wrapped := make([]string, 0, len(lines))
	for _, l := range lines {
		wrapped = append(wrapped, m.fit(l))
	}
	return WindowLines(wrapped, m.detailOff, layout.BodyHeight)
}

// section titles a non-empty block. An empty block contributes nothing.
func (m Model) section(title string, body []string) []string {
	if len(body) == 0 {
		return nil
	}
	return append(append([]string{m.theme.Header.Render(title + ":")}, body...), "")
}

// stateLine draws status, priority and the claim as one idea, coloured the
// way the board colours a card: the category the status belongs to, the ends
// of the priority range, and the claim only when it is live.
func stateLine(theme Theme, status string, category core.StateCategory, priority core.Priority, claim string, claimed bool) string {
	claimStyle := theme.Dim
	if claimed {
		claimStyle = theme.Claimed
	}
	return theme.Dim.Render("status: ") + theme.Category(category).Bold(theme.Color).Render(status) +
		theme.Dim.Render("   priority: ") + theme.Priority(priority).Render("P"+priorityDigit(priority)) +
		theme.Dim.Render("   claim: ") + claimStyle.Render(claim)
}

// claimSummary states who holds a task and how fresh the lease is. It always
// renders relative, independent of the configured output style, because a
// lease is judged by how much is left rather than the clock face it started
// at — the same reasoning the activity view already applies to its live tail.
func claimSummary(t core.Task, now time.Time, style output.TimeStyle, actors map[string]string) string {
	if !t.ClaimedAtTime(now) {
		return "unclaimed"
	}
	who := actorLabel(t.ClaimedByActorID, actors)
	if expiry := style.FormatPtr(t.LeaseExpiresAt); expiry != "" {
		return "claimed by " + who + ", lease expires " + expiry
	}
	return "claimed by " + who
}

// leaseTimeStyle is the always-relative style claimSummary renders with.
func leaseTimeStyle() output.TimeStyle {
	style, _ := output.NewTimeStyle(output.TimeRelative, "local")
	return style
}

// peopleText names who a task belongs to: who created it, and who, if
// anyone, it is assigned to.
func peopleText(t core.Task, actors map[string]string) string {
	who := "people: created by " + actorLabel(t.CreatorActorID, actors)
	if t.AssigneeActorID == "" {
		return who + "   assigned to unassigned"
	}
	return who + "   assigned to " + actorLabel(t.AssigneeActorID, actors)
}

// timeText renders a task's dates through the configured style, dropping
// "updated" when it renders the same as "created" so a task edited the day
// it was made does not repeat itself.
func timeText(t core.Task, style output.TimeStyle) string {
	created := style.Format(t.CreatedAt)
	parts := []string{"time: created " + created}
	if updated := style.Format(t.UpdatedAt); updated != "" && updated != created {
		parts = append(parts, "updated "+updated)
	}
	if due := style.FormatPtr(t.DueAt); due != "" {
		parts = append(parts, "due "+due)
	}
	return strings.Join(parts, "   ")
}

// tagsText renders a task's tags, or the empty string when it has none, so
// the caller can leave the line out entirely rather than saying "tags: none".
func tagsText(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	return "tags: " + strings.Join(tags, ", ")
}

// actorLabel names an actor by the handle the detail view resolved, falling
// back to a short identifier when the lookup found nothing — a stale actor,
// a service that could not be reached, or an identifier not looked up at all.
func actorLabel(id string, actors map[string]string) string {
	if id == "" {
		return "unknown"
	}
	if handle, ok := actors[id]; ok && handle != "" {
		return "@" + handle
	}
	return shortID(id)
}

// The columns a task's prose is indented to. A section's content sits at
// DetailIndent, and a comment's body one step further in under its author.
const (
	DetailIndent  = 2
	CommentIndent = 4
)

// bodyLines wraps a task body into indented display lines, matching the
// indentation every other section's content carries.
//
// It wraps rather than clipping. A body cut at the right edge ends mid-word
// with no mark saying so, which reads as a task somebody failed to finish
// writing rather than as a pane too narrow to hold it.
func bodyLines(body string, width int) []string {
	if strings.TrimSpace(body) == "" {
		return nil
	}
	var out []string
	for _, l := range Wrap(body, width) {
		out = append(out, strings.Repeat(" ", DetailIndent)+l)
	}
	return out
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

// artifactLines renders the structured output recorded on a task.
func artifactLines(artifacts []core.Artifact) []string {
	out := make([]string, 0, len(artifacts))
	for _, a := range artifacts {
		name := a.Name
		if name == "" {
			name = shortID(a.ID)
		}
		out = append(out, "  ["+string(a.Kind)+"] "+name)
	}
	return out
}

// commentLines renders the comment thread, its timestamps through the
// configured style and its authors resolved to handles where the detail view
// could resolve them.
//
// selected marks the comment the comment actions act on, with the same marker a
// selected card carries, because a thread whose entries cannot be told apart is
// a thread whose entries cannot be edited or removed.
func commentLines(comments []core.Comment, actors map[string]string, style output.TimeStyle, selected, width int) []string {
	out := make([]string, 0, len(comments)*2)
	for i, c := range comments {
		out = append(out, SelectionMarker(i == selected)+
			style.Format(c.CreatedAt)+"  "+actorLabel(c.AuthorActorID, actors)+":")
		for _, l := range Wrap(c.Body, width) {
			out = append(out, strings.Repeat(" ", CommentIndent)+l)
		}
	}
	return out
}

// CommentTarget names one comment the way the thread shows it, which is what a
// confirmation has to say before it removes one: a thread of four comments from
// the same author on the same day is told apart by its timestamp, so the name
// carries both.
func CommentTarget(c core.Comment, actors map[string]string, style output.TimeStyle) string {
	when := style.Format(c.CreatedAt)
	who := actorLabel(c.AuthorActorID, actors)
	if when == "" {
		return "the comment by " + who
	}
	return "the comment by " + who + " from " + when
}

// helpLines renders the bindings of the view help was opened from.
func (m Model) helpLines(layout Layout) []string {
	lines := []string{m.theme.Header.Render("help"), ""}
	lines = append(lines, "this view:")
	for _, e := range m.keys.ViewHelp(m.underView(), m.permits()) {
		lines = append(lines, "  "+pad(e.Keys, 12)+e.Desc)
	}
	lines = append(lines, "", "everywhere:")
	for _, e := range m.keys.GlobalHelp(m.offersView()) {
		lines = append(lines, "  "+pad(e.Keys, 12)+e.Desc)
	}
	lines = append(lines, "", "card markers: "+strings.Join(CardLegend, ", "))
	lines = append(lines, "", "filter bar accepts: "+strings.Join(FilterKeys, ", "))
	lines = append(lines, "  "+query.SyntaxHint())
	lines = append(lines, "", "activity filter accepts: "+strings.Join(ActivityFilterKeys, ", "))
	lines = append(lines, "  "+ActivitySyntaxHint())
	rows := VisibleRows(layout.BodyHeight, len(lines))
	offset := ScrollWindow(m.helpOff, m.helpOff, rows, len(lines))
	windowed := WindowLines(lines, offset, rows)
	if hint := ScrollHint(offset, rows, len(lines)); hint != "" {
		windowed = append(windowed, m.theme.Dim.Render(hint))
	}
	out := make([]string, 0, len(windowed))
	for _, l := range windowed {
		out = append(out, m.fit(l))
	}
	return out
}

// CardLegend explains the markers a card carries. It renders as one line, so
// the entries stay short enough that the whole legend fits a narrow terminal
// rather than being truncated into uselessness.
var CardLegend = []string{
	"@ claimed", "@me claimed by you", "! blocked", "+ dependencies", "* due date",
	DeletedMarker + " deleted",
}

// footerLines renders the prompt, the status bar and the advertised bindings.
func (m Model) footerLines() string {
	var lines []string
	switch {
	case m.prompt != promptNone:
		lines = append(lines, m.input.View())
	case m.choice != choiceNone:
		lines = append(lines, m.fit(m.theme.Header.Render(m.choice.Prompt())+
			strings.Join(ChoiceLabels(m.choices), "  ")+m.theme.Dim.Render("   (esc cancels)")))
	case m.confirm.Open():
		lines = append(lines, m.confirmLines()...)
	case m.form.Open():
		lines = append(lines, m.formLines()...)
	}
	if bar := m.statusBar(); bar != "" {
		lines = append(lines, bar)
	}
	keys := make([]string, 0, 8)
	for _, e := range m.keys.ShortHelp(m.view, m.actionContext()) {
		keys = append(keys, m.theme.Selected.Render(e.Keys)+m.theme.Dim.Render(" "+e.Desc))
	}
	lines = append(lines, m.fit(strings.Join(keys, m.theme.Bar.Render("  "))))
	return strings.Join(lines, "\n")
}

// statusBar states what the board holds and what just happened, so the line is
// never blank and never carries only one of the two.
func (m Model) statusBar() string {
	var segments []string
	if m.view == viewBoard || m.view == viewDetail {
		count := fmt.Sprintf("%d tasks in %d columns", countTasks(m.columns), len(m.columns))
		// Only when the board is wider than the terminal does which slice is
		// on screen matter, so the quiet case stays quiet.
		visible := LayoutFor(m.width, m.height, len(m.columns)).VisibleColumns
		if start, end := VisibleRange(len(m.columns), visible, m.sel.Col); end-start < len(m.columns) {
			count += fmt.Sprintf(", showing %d-%d", start+1, end)
		}
		segments = append(segments, m.theme.Dim.Render(count))
	}
	if m.view == viewProjects {
		segments = append(segments, m.theme.Dim.Render(fmt.Sprintf("%d projects", len(m.projects))))
	}
	if m.view == viewActivity {
		segments = append(segments, m.theme.Dim.Render(m.activityCount()))
	}
	if m.view == viewHistory {
		segments = append(segments, m.theme.Dim.Render(HistoryCount(len(m.history), m.historyMore)))
	}
	switch {
	case m.activityFilterErr != "":
		segments = append(segments, m.theme.Error.Render("activity filter error: "+m.activityFilterErr))
	case m.filterErr != "":
		segments = append(segments, m.theme.Error.Render("filter error: "+m.filterErr))
	case m.err != "":
		segments = append(segments, m.theme.Error.Render("✗ "+m.err))
	case m.status != "":
		segments = append(segments, m.theme.Status.Render("✓ "+m.status))
	}
	if note := m.holderNote(); note != "" {
		segments = append(segments, m.theme.Claimed.Render(note))
	}
	if len(segments) == 0 {
		return ""
	}
	return m.fit(strings.Join(segments, m.theme.Bar.Render(" │ ")))
}

// pad right-pads s to width.
func pad(s string, width int) string {
	if n := width - len([]rune(s)); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

// confirmLines draws the pending question. It is the one line of the frame that
// has to be read rather than glanced at, so the question is in the error style
// the status bar uses for a refusal and the keys that answer it are named.
func (m Model) confirmLines() []string {
	return []string{m.fit(m.theme.Error.Render(m.confirm.Question()) + " " +
		m.theme.Dim.Render(ConfirmHelp(m.keys.Agree.Help().Key, m.keys.Cancel.Help().Key)))}
}

// formLines draws the open form: its title, one row per visible field with the
// values that field could hold instead, and the keys that move and answer it.
func (m Model) formLines() []string {
	out := []string{m.fit(m.theme.Header.Render(m.form.Title + ":"))}
	if m.form.Note != "" {
		out = append(out, m.fit(m.theme.Dim.Render("  "+m.form.Note)))
	}
	for _, l := range m.form.Lines() {
		row := "  " + pad(l.Label, 14) + l.Value
		if l.Selected {
			out = append(out, m.fit(m.selection().Render(row)+
				m.theme.Dim.Render("   "+l.Hint)))
			continue
		}
		out = append(out, m.fit(m.theme.Dim.Render(row)))
	}
	return append(out, m.fit(m.theme.Dim.Render("  "+m.formHelp())))
}

// formHelp names the keys that drive a form, under whichever scheme is loaded.
func (m Model) formHelp() string {
	return m.keys.Up.Help().Key + " " + m.keys.Down.Help().Key + " field   " +
		m.keys.Left.Help().Key + " " + m.keys.Right.Help().Key + " value   " +
		m.keys.Accept.Help().Key + " apply   " + m.keys.Cancel.Help().Key + " cancel"
}
