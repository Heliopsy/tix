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
	// An input panel is taller than the one line the chrome budget assumes, and
	// a body drawn to the budget pushed the panel's own legend off the bottom
	// of the terminal, which is where the keys that answer it live.
	footer := m.footerLines()
	layout.BodyHeight = max(1, layout.BodyHeight-max(0, strings.Count(footer, "\n")+1-FooterLines))
	return strings.Join([]string{
		m.titleBar(),
		strings.Join(m.bodyLines(layout), "\n"),
		footer,
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
	keyCol := ProjectKeyColumn(m.projects)
	for i := offset; i < len(m.projects) && len(lines) < rows; i++ {
		lines = append(lines, m.projectRow(m.projects[i], keyCol, i == m.projectSel))
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

// ProjectIcon renders a project's glyph in a fixed two cells, followed by one
// space.
//
// The glyph cannot be assumed to be one cell. Half the seeded icons are Wide
// by Unicode's east-asian width and half are Neutral: U+1F916 and U+1F4DA
// take two cells, U+1F5A5 and U+1F6E0 take one, and a terminal may render a
// text-default emoji either way. Appending a single space to whatever it is
// left the list ragged, with "web" touching its icon while "agents" had a gap.
// Measuring and padding is the only thing that lines up in every font.
func ProjectIcon(p core.Project) string {
	glyph := ProjectGlyph(p)
	w := lipgloss.Width(glyph)
	if w >= ProjectIconCells {
		return glyph + " "
	}
	return glyph + strings.Repeat(" ", ProjectIconCells-w) + " "
}

// ProjectIconCells is the width every project glyph is drawn in, so the keys
// after them start in one column.
const ProjectIconCells = 2

// ProjectKeyColumn is the width the key column needs for these projects, so
// every name starts in the same place. Two literal spaces used to separate a
// key from its name, which put the names wherever each key happened to end.
func ProjectKeyColumn(projects []core.Project) int {
	widest := 0
	for _, p := range projects {
		if w := lipgloss.Width(p.Key); w > widest {
			widest = w
		}
	}
	return widest
}

// projectRow renders one project of the picker.
func (m Model) projectRow(p core.Project, keyCol int, selected bool) string {
	label := SelectionMarker(selected) + ProjectIcon(p) + pad(p.Key, keyCol) + "  " + p.Name
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
	return append(out, barStyle.Render(bar)+" "+m.metaLine(card.Meta))
}

// metaLine renders a card's identifiers, each run in the style its own meaning
// asks for: dim throughout, except the deadline marker, which the theme
// colours by how late it is. The marker is a word either way, so a terminal
// getting no escapes loses the colour and keeps the meaning.
func (m Model) metaLine(segs []MetaSegment) string {
	var b strings.Builder
	for _, s := range segs {
		b.WriteString(m.quiet(m.theme.Due(s.Due).Inherit(m.theme.Dim)).Render(s.Text))
	}
	return b.String()
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
	return m.paletteOpen || m.prompt != promptNone || m.choice != choiceNone ||
		m.confirm.Open() || m.form.Open()
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
		m.theme.Dim.Render(timeText(t, m.timeStyle)) + m.dueText(t, now),
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

// timeText renders the dates a task simply has, through the configured style,
// dropping "updated" when it renders the same as "created" so a task edited
// the day it was made does not repeat itself.
//
// The due date is not among them. It is the one date that means something
// different depending on what day it is read, so it is drawn by dueText in the
// style that says so rather than dim beside two dates that are only history.
func timeText(t core.Task, style output.TimeStyle) string {
	created := style.Format(t.CreatedAt)
	parts := []string{"time: created " + created}
	if updated := style.Format(t.UpdatedAt); updated != "" && updated != created {
		parts = append(parts, "updated "+updated)
	}
	return strings.Join(parts, "   ")
}

// dueText renders the deadline beside the other dates, named by how it stands
// against today. A bare date left the reader comparing it with a calendar; the
// state is what they were working out, so the detail view says it.
func (m Model) dueText(t core.Task, now time.Time) string {
	due := m.timeStyle.FormatPtr(t.DueAt)
	if due == "" {
		return ""
	}
	state := t.DueState(now)
	text := "due " + due
	if state.Notable() {
		text += " (" + state.String() + ")"
	}
	return "   " + m.theme.Due(state).Inherit(m.theme.Dim).Render(text)
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
	// Wrapped, not cut. The legend used to be one line on the promise that its
	// entries would stay short enough to fit, which a narrow terminal has never
	// kept: a legend truncated mid-entry explains a marker by half its words.
	lines = append(lines, "")
	lines = append(lines, Wrap("card markers: "+strings.Join(CardLegend, ", "), max(1, layout.Width))...)
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

// CardLegend explains the markers a card carries. The help wraps it, so an
// entry is short for readability rather than to fit one line.
// The due markers are drawn only where a deadline is pressing. A marker for
// any due date at all was removed once for appearing on nearly every card,
// which is why "due" here means this week and not "has a date".
var CardLegend = []string{
	"@ claimed", "@me claimed by you", "! blocked", "+ dependencies",
	DeletedMarker + " deleted",
	DueSoonMarker + " due within a week", DueOverdueMarker + " overdue",
}

// footerLines renders the status bar and then either the open input mode or
// the bindings of the view.
//
// While an input mode is open the view's bindings are not drawn at all. A
// legend advertising "n new task" under a text field is a lie the interface
// tells constantly: pressing n there types the letter n. The panel carries its
// own legend instead, so whatever is on the last line is what will work.
func (m Model) footerLines() string {
	var lines []string
	if bar := m.statusBar(); bar != "" {
		lines = append(lines, bar)
	}
	if panel := m.inputPanel(); len(panel) > 0 {
		return strings.Join(append(lines, panel...), "\n")
	}
	keys := make([]string, 0, 8)
	// The palette first. fit truncates from the right, so a terminal too narrow
	// for the whole line drops the view's own bindings before it drops the one
	// key that reaches all of them, which is the complaint this hint answers.
	if hint := FooterHint(m.keys, m.width); hint != "" {
		name, desc, _ := strings.Cut(hint, " ")
		keys = append(keys, m.theme.Selected.Render(name)+m.theme.Dim.Render(" "+desc))
	}
	for _, e := range m.keys.ShortHelp(m.view, m.actionContext()) {
		keys = append(keys, m.theme.Selected.Render(e.Keys)+m.theme.Dim.Render(" "+e.Desc))
	}
	lines = append(lines, m.fit(strings.Join(keys, m.theme.Bar.Render("  "))))
	return strings.Join(lines, "\n")
}

// InputRule is the line an input panel is separated from the body by. It runs
// the width of the terminal, so a reader on a terminal with no colour at all
// can see that the screen below it has taken the keyboard.
const InputRule = "─"

// FooterLines is the footer the chrome budget allows for: the status bar and
// the line of bindings. Anything taller comes out of the body.
const FooterLines = 2

// inputPanel draws whichever input mode is open, or nothing. Every mode is the
// same shape: a rule, a title, the rows it gathers, and the keys that answer
// it, so a reader who has used one has used all of them.
func (m Model) inputPanel() []string {
	switch {
	case m.paletteOpen:
		return m.panel(m.palettePanel())
	case m.prompt != promptNone:
		return m.panel(m.promptPanel())
	case m.choice != choiceNone:
		return m.panel(m.choicePanel())
	case m.confirm.Open():
		return m.panel(m.confirmPanel())
	case m.form.Open():
		return m.panel(m.formPanel())
	}
	return nil
}

// Panel is one input mode's contents: what it is called, what a reader needs
// to know to answer it, its rows, and the keys that answer it.
type Panel struct {
	Title string
	Note  string
	Rows  []string
	Keys  string
}

// panel frames an input mode, rule first and legend last.
func (m Model) panel(p Panel) []string {
	width := max(1, m.width)
	out := []string{m.theme.Bar.Render(strings.Repeat(InputRule, width)),
		m.fit(m.theme.Header.Render(p.Title + ":"))}
	if p.Note != "" {
		out = append(out, m.fit(m.theme.Dim.Render("  "+p.Note)))
	}
	out = append(out, p.Rows...)
	return append(out, m.fit(m.theme.Dim.Render("  "+p.Keys)))
}

// promptPanel renders a single-value input as a form with one field, which is
// what it always was. The board behind it is dimmed by quiet, and the legend
// names the two keys that end it.
func (m Model) promptPanel() Panel {
	spec, _ := m.prompt.Spec()
	if spec.Multiline {
		return Panel{
			Title: spec.Title(),
			Note:  spec.Placeholder,
			Rows:  m.proseRows(spec.Field),
			Keys:  m.proseHelp(),
		}
	}
	row := "  " + pad(spec.Field, FormLabelWidth) + m.input.View()
	return Panel{
		Title: spec.Title(),
		Note:  spec.Placeholder,
		Rows:  []string{m.fit(row)},
		Keys:  m.acceptHelp("apply"),
	}
}

// proseRows draws the multi-line field into the same two columns every other
// input mode uses: the label once, and the text beside it, every line of it
// lined up under the first.
func (m Model) proseRows(label string) []string {
	lines := strings.Split(strings.TrimRight(m.area.View(), "\n"), "\n")
	out := make([]string, 0, len(lines))
	for i, l := range lines {
		head := pad("", FormLabelWidth)
		if i == 0 {
			head = pad(label, FormLabelWidth)
		}
		out = append(out, m.fit(strings.Repeat(" ", DetailIndent)+head+l))
	}
	return out
}

// proseHelp names the keys a multi-line field answers to. Enter is listed as
// the newline it now inserts, because a reader who cannot discover how to save
// a field is worse off than with the one line it replaced.
func (m Model) proseHelp() string {
	return m.keys.Accept.Help().Key + " newline   " +
		m.keys.Commit.Help().Key + " apply   " + m.keys.Cancel.Help().Key + " cancel"
}

// choicePanel renders the numbered picker. The options keep their digits,
// because a single keystroke is why the picker exists and nothing here is
// worth making a reader press twice.
func (m Model) choicePanel() Panel {
	row := "  " + pad(m.choice.Field(), FormLabelWidth) + strings.Join(ChoiceLabels(m.choices), "   ")
	return Panel{
		Title: strings.TrimSuffix(strings.TrimSpace(m.choice.Prompt()), ":"),
		Rows:  []string{m.fit(m.theme.Style().Render(row))},
		Keys:  ChoiceHelp(len(m.choices)) + "   " + m.keys.Cancel.Help().Key + " cancel",
	}
}

// confirmPanel renders the pending question. It is the one panel that has to be
// read rather than glanced at, so the question is in the error style the status
// bar uses for a refusal.
func (m Model) confirmPanel() Panel {
	return Panel{
		Title: "confirm",
		Rows:  []string{m.fit("  " + m.theme.Error.Render(m.confirm.Question()))},
		Keys:  ConfirmHelp(m.keys.Agree.Help().Key, m.keys.Cancel.Help().Key),
	}
}

// formPanel renders the open form: one row per visible field with the values
// that field could hold instead.
func (m Model) formPanel() Panel {
	rows := make([]string, 0, len(m.form.Fields))
	for _, l := range m.form.Lines() {
		if l.Selected && l.Kind == FieldProse {
			rows = append(rows, m.proseRows(l.Label)...)
			continue
		}
		value := l.Value
		if l.Selected && l.Kind == FieldText {
			value = m.input.View()
		}
		row := "  " + pad(l.Label, FormLabelWidth) + value
		if l.Selected {
			rows = append(rows, m.fit(m.selection().Render(row)+m.theme.Dim.Render("   "+l.Hint)))
			continue
		}
		rows = append(rows, m.fit(m.theme.Dim.Render(row)))
	}
	return Panel{Title: m.form.Title, Note: m.form.Note, Rows: rows, Keys: m.formHelp()}
}

// FormLabelWidth is the column the values of every input mode line up at.
const FormLabelWidth = 14

// ChoiceHelp names the digits a picker takes.
func ChoiceHelp(n int) string {
	if n <= 1 {
		return "1 choose"
	}
	return "1-" + fmt.Sprint(n) + " choose"
}

// acceptHelp names the two keys that end a single-value input.
func (m Model) acceptHelp(verb string) string {
	return m.keys.Accept.Help().Key + " " + verb + "   " + m.keys.Cancel.Help().Key + " cancel"
}

// statusBar states what the board holds and what just happened, so the line is
// never blank and never carries only one of the two.
func (m Model) statusBar() string {
	var segments []string
	// Only on the board. The count of columns and what slice of them is on
	// screen says nothing about one task, and it sat under the task detail as
	// though it did.
	if m.view == viewBoard {
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

// pad right-pads s to width, measuring what a terminal draws rather than
// counting runes.
//
// A rune is not a cell. An emoji is commonly two, so a column padded by rune
// count comes out ragged the moment anything in it carries one, which is what
// the project list did.
func pad(s string, width int) string {
	if n := width - lipgloss.Width(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

// formHelp names the keys that drive a form, under whichever scheme is loaded
// and for whichever field the cursor is on. A field the reader types into owns
// the arrows, so the legend stops offering them as the way between fields the
// moment they stop being it.
func (m Model) formHelp() string {
	fields := m.keys.NextField.Help().Key + " " + m.keys.PrevField.Help().Key + " field   "
	switch {
	case m.form.Prose():
		return fields + m.proseHelp()
	case m.form.Typing():
		return fields + m.acceptHelp("apply")
	default:
		return fields + m.keys.Left.Help().Key + " " + m.keys.Right.Help().Key + " value   " +
			m.acceptHelp("apply")
	}
}
