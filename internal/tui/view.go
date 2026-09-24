// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/query"
)

// View renders the current frame.
func (m Model) View() string {
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
		return m.settingsLines()
	case viewActivity:
		return m.activityLines(layout)
	case viewTenant:
		return m.tenantLines(layout)
	case viewStats:
		return m.statsLines(layout)
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

// settingsLines renders the keybinding schemes on offer, marking the active
// one and showing the keys each would give the actions people reach for most.
func (m Model) settingsLines() []string {
	lines := []string{m.theme.Header.Render("keybindings"), ""}
	for i, scheme := range Schemes() {
		mark := "  "
		if scheme == m.scheme {
			mark = "✓ "
		}
		label := SelectionMarker(i == m.schemeSel) + mark + pad(string(scheme), 10) + SchemeDescription(scheme)
		lines = append(lines, m.emphasize(m.fit(label), i == m.schemeSel))
	}
	lines = append(lines, "", m.theme.Dim.Render("  the highlighted scheme binds:"))
	preview := KeyMapFor(Schemes()[clamp(m.schemeSel, 0, len(Schemes())-1)])
	for _, action := range []string{"Up", "Down", "New", "EditTitle", "Comment", "Filter", "Back"} {
		b, ok := preview.Binding(action)
		if !ok {
			continue
		}
		lines = append(lines, "    "+pad(strings.Join(b.Keys(), ", "), 22)+b.Help().Desc)
	}
	return lines
}

// projectRow renders one project of the picker.
func (m Model) projectRow(p core.Project, selected bool) string {
	label := SelectionMarker(selected) + ProjectGlyph(p) + " " + p.Key + "  " + p.Name
	if p.Archived() {
		label += "  (archived)"
	}
	label = Truncate(label, m.width)
	if selected {
		return m.theme.Selected.Render(label)
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
	blocks := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		blocks = append(blocks, m.columnBlock(i, layout, i < end-1))
	}
	return strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, blocks...), "\n")
}

// columnBlock renders one column inside its own border.
func (m Model) columnBlock(index int, layout Layout, gap bool) string {
	col := m.columns[index]
	focused := index == m.sel.Col
	inner := layout.InnerWidth()
	rows := VisibleRows(layout.CardRows(), len(col.Tasks))

	offset := 0
	if focused {
		offset = ScrollWindow(m.rowOff, m.sel.Row, rows, len(col.Tasks))
	}
	lines := []string{m.columnHeading(col, inner)}
	for i := offset; i < len(col.Tasks) && len(lines) <= rows; i++ {
		lines = append(lines, m.cardLine(col.Tasks[i], inner, focused && i == m.sel.Row))
	}
	if hint := ScrollHint(offset, rows, len(col.Tasks)); hint != "" {
		lines = append(lines, m.theme.Dim.Render(Truncate(hint, inner)))
	}
	if len(col.Tasks) == 0 {
		lines = append(lines, m.theme.Empty.Render(Truncate("  empty", inner)))
	}

	style := m.theme.Column
	if focused {
		style = m.theme.Focused
	}
	if gap {
		style = style.MarginRight(1)
	}
	return style.Width(inner).Height(max(1, layout.BodyHeight-BorderHeight)).Render(strings.Join(lines, "\n"))
}

// columnHeading names a column and counts it, coloured by the category its
// state belongs to rather than by its name, because workflows are user defined.
// It does not number the column: which slice of a wide board is on screen is
// said once in the status bar rather than repeated in every heading.
func (m Model) columnHeading(col Column, width int) string {
	head := col.Label + " (" + fmt.Sprint(len(col.Tasks)) + ")"
	return m.theme.Category(col.Category).Bold(true).Render(Truncate(head, width))
}

// cardLine renders one task as a single line of a column. A selected card is
// drawn in one colour throughout so it cannot be missed, and an unselected one
// carries the CLI's meanings: the ref, then the priority, then its markers.
func (m Model) cardLine(t core.Task, width int, selected bool) string {
	marker := SelectionMarker(selected)
	ref := t.Ref
	badge := "P" + priorityDigit(t.Priority)
	flags := CardFlags(t, m.now())
	head := marker + ref + " " + badge + flags + " "
	title := Truncate(t.Title, max(1, width-len([]rune(head))))
	if selected {
		return m.theme.Selected.Render(Truncate(head+title, width))
	}
	return m.theme.Ref.Render(marker+ref) + " " +
		m.theme.Priority(t.Priority).Render(badge) +
		m.theme.Blocked.Render(flags) + " " + title
}

// CardFlags renders a card's state markers compactly, so a narrow column still
// says what is true of a task. The help view carries the legend.
func CardFlags(t core.Task, now time.Time) string {
	var b strings.Builder
	if t.ClaimedAtTime(now) {
		b.WriteString("@")
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

	body := bodyLines(t.Body)
	fields := customFieldLines(t, d.fieldDefs)
	comments := commentLines(d.comments, d.actors, m.timeStyle)
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
	return theme.Dim.Render("status: ") + theme.Category(category).Bold(true).Render(status) +
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

// bodyLines splits a task body into indented display lines, matching the
// indentation every other section's content carries.
func bodyLines(body string) []string {
	if strings.TrimSpace(body) == "" {
		return nil
	}
	raw := strings.Split(body, "\n")
	out := make([]string, len(raw))
	for i, l := range raw {
		out[i] = "  " + l
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
func commentLines(comments []core.Comment, actors map[string]string, style output.TimeStyle) []string {
	out := make([]string, 0, len(comments)*2)
	for _, c := range comments {
		out = append(out, "  "+style.Format(c.CreatedAt)+"  "+actorLabel(c.AuthorActorID, actors)+":")
		for _, l := range strings.Split(c.Body, "\n") {
			out = append(out, "    "+l)
		}
	}
	return out
}

// helpLines renders the bindings of the view help was opened from.
func (m Model) helpLines(layout Layout) []string {
	lines := []string{m.theme.Header.Render("help"), ""}
	lines = append(lines, "this view:")
	for _, e := range m.keys.ViewHelp(m.underView()) {
		lines = append(lines, "  "+pad(e.Keys, 12)+e.Desc)
	}
	lines = append(lines, "", "everywhere:")
	for _, e := range m.keys.GlobalHelp() {
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

// CardLegend explains the markers a card carries.
var CardLegend = []string{"@ claimed", "! blocked", "+ has dependencies", "* has a due date"}

// footerLines renders the prompt, the status bar and the advertised bindings.
func (m Model) footerLines() string {
	var lines []string
	switch {
	case m.prompt != promptNone:
		lines = append(lines, m.input.View())
	case m.choice != choiceNone:
		lines = append(lines, m.fit(m.theme.Header.Render(m.choice.Prompt())+
			strings.Join(ChoiceLabels(m.choices), "  ")+m.theme.Dim.Render("   (esc cancels)")))
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

// emphasize marks a line as selected without relying on colour.
func (m Model) emphasize(line string, selected bool) string {
	if !selected {
		return line
	}
	return m.theme.Selected.Render(line)
}
