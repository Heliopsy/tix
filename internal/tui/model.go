package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/heliopsy/tix/internal/core"
)

// Model is the whole state of the terminal interface.
type Model struct {
	svc   core.Service
	ctx   context.Context
	actor *core.Actor
	keys  KeyMap
	theme Theme
	now   func() time.Time

	view viewKind
	prev viewKind

	width  int
	height int

	projects   []core.Project
	projectSel int

	project  core.Project
	workflow *core.WorkflowDefinition
	tasks    []core.Task
	columns  []Column
	sel      Selection
	rowOff   int

	filterText string
	filter     core.TaskFilter
	filterErr  string
	input      textinput.Model
	editing    bool

	detail    *detailMsg
	detailOff int

	choosing bool
	choices  []core.State

	leases    map[string]string
	lastSeq   int64
	connected bool
	events    <-chan core.Event

	err         string
	status      string
	fatal       error
	interrupted bool
	openProject string
}

// Config builds a model without opening anything.
type Config struct {
	Service core.Service
	Context context.Context
	Actor   *core.Actor
	Environ []string
	Project string
	Filter  string
	Now     func() time.Time
}

// New builds the initial model.
func New(cfg Config) Model {
	in := textinput.New()
	in.Prompt = "filter: "
	in.Placeholder = "status:todo is:unclaimed text"
	in.CharLimit = 512
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	ctx := cfg.Context
	if ctx == nil {
		ctx = context.Background()
	}
	m := Model{
		svc: cfg.Service, ctx: ctx, actor: cfg.Actor,
		keys: DefaultKeyMap(), theme: NewTheme(ColorEnabled(cfg.Environ)), now: now,
		view: viewProjects, input: in, leases: map[string]string{},
		openProject: cfg.Project, width: 80, height: 24,
	}
	if cfg.Filter != "" {
		m = m.applyFilterText(cfg.Filter)
	}
	return m
}

// Init starts the project listing and the event subscription.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadProjects(), m.subscribe(m.lastSeq))
}

// Update folds one message into the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m.reduce(msg) }

// reduce is the whole state machine, kept free of rendering.
func (m Model) reduce(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.onResize(msg), nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case projectsMsg:
		return m.onProjects(msg)
	case boardMsg:
		return m.onBoard(msg)
	case tasksMsg:
		return m.onTasks(msg)
	case detailMsg:
		m.detail, m.detailOff, m.view = &msg, 0, viewDetail
		return m, nil
	case eventMsg:
		return m.onEvent(msg)
	case streamMsg:
		return m.onStream(msg)
	case reconnectMsg:
		return m, m.subscribe(m.lastSeq)
	case actionMsg:
		return m.onAction(msg)
	case errMsg:
		return m.onError(msg)
	}
	return m, nil
}

// onResize adopts a new terminal size, ignoring a size the terminal cannot report.
func (m Model) onResize(msg tea.WindowSizeMsg) Model {
	if msg.Width <= 0 || msg.Height <= 0 {
		return m
	}
	m.width, m.height = msg.Width, msg.Height
	return m
}

// onProjects installs a project listing and opens a board when one was asked for.
func (m Model) onProjects(msg projectsMsg) (Model, tea.Cmd) {
	m.projects = msg.projects
	if m.projectSel >= len(m.projects) {
		m.projectSel = max(0, len(m.projects)-1)
	}
	if m.openProject == "" {
		return m, nil
	}
	for i, p := range m.projects {
		if p.Key == m.openProject {
			m.projectSel, m.openProject = i, ""
			return m, m.loadBoard(p)
		}
	}
	m.err = "project " + m.openProject + " is not accessible"
	m.openProject = ""
	return m, nil
}

// onBoard installs a project's workflow and first page of tasks.
func (m Model) onBoard(msg boardMsg) (Model, tea.Cmd) {
	m.project, m.workflow, m.view = msg.project, msg.workflow, viewBoard
	return m.installTasks(msg.tasks)
}

// onTasks refreshes the board's tasks without changing the open project.
func (m Model) onTasks(msg tasksMsg) (Model, tea.Cmd) {
	return m.installTasks(msg.tasks)
}

// installTasks rebuilds the columns and keeps the selection on its task.
func (m Model) installTasks(tasks []core.Task) (Model, tea.Cmd) {
	selected, _ := TaskAt(m.columns, m.sel)
	m.tasks = tasks
	m.columns = BuildColumns(m.workflow, m.visibleTasks(tasks))
	m.sel = PreserveSelection(m.columns, m.sel, selected.ID)
	m.rowOff = ScrollOffset(m.rowOff, m.sel.Row, LayoutFor(m.width, m.height, len(m.columns)).BodyHeight)
	return m, nil
}

// visibleTasks drops the tasks the active filter excludes.
func (m Model) visibleTasks(tasks []core.Task) []core.Task {
	out := make([]core.Task, 0, len(tasks))
	for _, t := range tasks {
		if MatchesFilter(m.filter, t, m.project.Key, m.now()) {
			out = append(out, t)
		}
	}
	return out
}

// onEvent records the stream position and refreshes what the event touched.
func (m Model) onEvent(msg eventMsg) (Model, tea.Cmd) {
	if msg.event.Seq > m.lastSeq {
		m.lastSeq = msg.event.Seq
	}
	m.connected = true
	if m.events == nil {
		return m, m.reloadFor(msg.event)
	}
	return m, tea.Batch(nextEvent(m.events), m.reloadFor(msg.event))
}

// reloadFor reloads only what an event can have changed.
func (m Model) reloadFor(e core.Event) tea.Cmd {
	if m.project.ID != "" && e.ProjectID != "" && e.ProjectID != m.project.ID {
		return nil
	}
	switch e.Type {
	case core.EventProjectCreated, core.EventProjectUpdated:
		return m.loadProjects()
	case core.EventWorkflowUpdated:
		if m.project.ID == "" {
			return nil
		}
		return m.loadBoard(m.project)
	}
	if m.project.ID == "" {
		return nil
	}
	return m.loadTasks()
}

// onStream records the subscription's health and reconnects without a gap.
func (m Model) onStream(msg streamMsg) (Model, tea.Cmd) {
	if msg.connected && msg.events != nil {
		m.connected, m.events, m.status = true, msg.events, "connected"
		return m, nextEvent(msg.events)
	}
	m.connected, m.events = false, nil
	if msg.err != nil {
		m.err = "event stream: " + msg.err.Error()
	}
	return m, reconnect()
}

// onAction reports a board action and reloads so the board shows the truth.
func (m Model) onAction(msg actionMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.err = actionFailure(msg)
		if m.project.ID == "" {
			return m, nil
		}
		return m, m.loadTasks()
	}
	switch msg.kind {
	case actionClaim:
		m.leases[msg.ref.ID] = msg.token
		m.status = "claimed " + msg.ref.String()
	case actionRelease:
		delete(m.leases, msg.ref.ID)
		m.status = "released " + msg.ref.String()
	default:
		m.status = "transitioned " + msg.ref.String()
	}
	m.err = ""
	if m.project.ID == "" {
		return m, nil
	}
	return m, m.loadTasks()
}

// actionFailure explains a refused action in the words the service used.
func actionFailure(msg actionMsg) string {
	switch core.KindOf(msg.err) {
	case core.KindConflict:
		return "cannot " + msg.kind.Label() + ": the task is already claimed by another worker: " + msg.err.Error()
	case core.KindPrecondition:
		return "cannot " + msg.kind.Label() + ": " + msg.err.Error()
	case core.KindLeaseExpired:
		return "cannot " + msg.kind.Label() + ": the lease has expired: " + msg.err.Error()
	default:
		return msg.kind.Label() + " failed: " + msg.err.Error()
	}
}

// onError shows a failure, or gives up when it cannot be recovered from.
func (m Model) onError(msg errMsg) (Model, tea.Cmd) {
	if msg.fatal {
		m.fatal = msg.err
		return m, tea.Quit
	}
	m.err = msg.err.Error()
	return m, nil
}

// handleKey routes a key to the mode that owns it.
func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Interrupt):
		m.interrupted = true
		return m, tea.Quit
	case m.editing:
		return m.handleFilterKey(msg)
	case m.choosing:
		return m.handleChoiceKey(msg)
	case m.view == viewHelp:
		return m.handleHelpKey(msg)
	case key.Matches(msg, m.keys.Help):
		m.prev, m.view = m.view, viewHelp
		return m, nil
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Refresh):
		return m, m.refresh()
	}
	switch m.view {
	case viewProjects:
		return m.handleProjectsKey(msg)
	case viewBoard:
		return m.handleBoardKey(msg)
	case viewDetail:
		return m.handleDetailKey(msg)
	}
	return m, nil
}

// handleHelpKey dismisses the help view back to where it was opened from.
func (m Model) handleHelpKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Back, m.keys.Help, m.keys.Quit) {
		m.view = m.prev
	}
	return m, nil
}

// handleProjectsKey moves through the project listing and opens a board.
func (m Model) handleProjectsKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		m.projectSel = clamp(m.projectSel-1, 0, len(m.projects)-1)
	case key.Matches(msg, m.keys.Down):
		m.projectSel = clamp(m.projectSel+1, 0, len(m.projects)-1)
	case key.Matches(msg, m.keys.Top):
		m.projectSel = 0
	case key.Matches(msg, m.keys.Bottom):
		m.projectSel = max(0, len(m.projects)-1)
	case key.Matches(msg, m.keys.Enter):
		if m.projectSel < len(m.projects) {
			m.err = ""
			return m, m.loadBoard(m.projects[m.projectSel])
		}
	}
	return m, nil
}

// handleBoardKey moves the board selection and runs the board actions.
func (m Model) handleBoardKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Left):
		m.sel = MoveSelection(m.columns, m.sel, -1, 0)
	case key.Matches(msg, m.keys.Right):
		m.sel = MoveSelection(m.columns, m.sel, 1, 0)
	case key.Matches(msg, m.keys.Up):
		m.sel = MoveSelection(m.columns, m.sel, 0, -1)
	case key.Matches(msg, m.keys.Down):
		m.sel = MoveSelection(m.columns, m.sel, 0, 1)
	case key.Matches(msg, m.keys.Top):
		m.sel = ClampSelection(m.columns, Selection{Col: m.sel.Col})
	case key.Matches(msg, m.keys.Bottom):
		m.sel = ClampSelection(m.columns, Selection{Col: m.sel.Col, Row: 1 << 30})
	case key.Matches(msg, m.keys.Filter):
		return m.startEditing(), textinput.Blink
	case key.Matches(msg, m.keys.ClearFltr):
		return m.applyFilterText(""), m.loadTasks()
	case key.Matches(msg, m.keys.Projects), key.Matches(msg, m.keys.Back):
		m.view = viewProjects
		return m, nil
	case key.Matches(msg, m.keys.Enter):
		return m, m.loadDetail()
	case key.Matches(msg, m.keys.Claim):
		return m, m.claim()
	case key.Matches(msg, m.keys.Release):
		return m.release()
	case key.Matches(msg, m.keys.Transition):
		return m.startChoosing(), nil
	default:
		return m, nil
	}
	m.rowOff = ScrollOffset(m.rowOff, m.sel.Row, LayoutFor(m.width, m.height, len(m.columns)).BodyHeight)
	return m, nil
}

// handleDetailKey scrolls the detail view and returns to the board.
func (m Model) handleDetailKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		m.view, m.detail = viewBoard, nil
	case key.Matches(msg, m.keys.Up):
		m.detailOff = max(0, m.detailOff-1)
	case key.Matches(msg, m.keys.Down):
		m.detailOff++
	case key.Matches(msg, m.keys.Claim):
		return m, m.claim()
	case key.Matches(msg, m.keys.Release):
		return m.release()
	case key.Matches(msg, m.keys.Transition):
		return m.startChoosing(), nil
	}
	return m, nil
}

// handleFilterKey edits the filter bar and applies it on acceptance.
func (m Model) handleFilterKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.editing, m.filterErr = false, ""
		m.input.Blur()
		return m, nil
	case key.Matches(msg, m.keys.Accept):
		next := m.applyFilterText(m.input.Value())
		if next.filterErr != "" {
			return next, nil
		}
		next.editing = false
		next.input.Blur()
		return next, next.loadTasks()
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// handleChoiceKey picks a transition target by number.
func (m Model) handleChoiceKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Cancel) {
		m.choosing, m.choices = false, nil
		return m, nil
	}
	n := digit(msg.String())
	if n < 1 || n > len(m.choices) {
		return m, nil
	}
	target := m.choices[n-1].Key
	m.choosing, m.choices = false, nil
	return m, m.transition(target)
}

// startEditing focuses the filter bar on the current expression.
func (m Model) startEditing() Model {
	m.editing = true
	m.input.SetValue(m.filterText)
	m.input.CursorEnd()
	m.input.Focus()
	return m
}

// applyFilterText parses an expression, keeping the old filter when it is bad.
func (m Model) applyFilterText(text string) Model {
	parsed, err := ParseFilter(text)
	if err != nil {
		m.filterErr = err.Error()
		return m
	}
	m.filterErr, m.filterText, m.filter = "", text, parsed
	selected, _ := TaskAt(m.columns, m.sel)
	m.columns = BuildColumns(m.workflow, m.visibleTasks(m.tasks))
	m.sel = PreserveSelection(m.columns, m.sel, selected.ID)
	return m
}

// startChoosing offers the transitions the workflow permits from here.
func (m Model) startChoosing() Model {
	task, ok := m.selectedTask()
	if !ok {
		return m
	}
	choices := NextStates(m.workflow, task.Status)
	if len(choices) == 0 {
		m.err = "no transition is permitted from " + task.Status
		return m
	}
	m.choosing, m.choices, m.err = true, choices, ""
	return m
}

// release refuses when this session does not hold the task's lease.
func (m Model) release() (Model, tea.Cmd) {
	task, ok := m.selectedTask()
	if !ok {
		return m, nil
	}
	if m.leases[task.ID] == "" {
		m.err = "cannot release: this session does not hold a lease on " + task.Ref
		return m, nil
	}
	return m, m.releaseCmd(task)
}

// selectedTask returns the task the current view acts on.
func (m Model) selectedTask() (core.Task, bool) {
	if m.view == viewDetail && m.detail != nil {
		return m.detail.task, true
	}
	return TaskAt(m.columns, m.sel)
}

// refresh reloads whatever the current view shows.
func (m Model) refresh() tea.Cmd {
	if m.project.ID == "" {
		return m.loadProjects()
	}
	return m.loadTasks()
}

// digit parses a single decimal key press.
func digit(s string) int {
	if len(s) != 1 || s[0] < '1' || s[0] > '9' {
		return 0
	}
	return int(s[0] - '0')
}

// clamp bounds v within lo and hi.
func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
