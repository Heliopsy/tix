// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/query"
)

// Model is the whole state of the terminal interface.
type Model struct {
	svc   core.Service
	ctx   context.Context
	actor *core.Actor
	keys  KeyMap
	theme Theme
	now   func() time.Time
	// timeStyle renders every timestamp the interface draws. The zero value
	// is a working default, so a Model built without one still renders.
	timeStyle output.TimeStyle

	view viewKind
	// stats is the last statistics read, statsWindow indexes
	// statsWindowDays, and statsOff scrolls the view.
	stats       *core.Stats
	statsErr    error
	statsWindow int
	statsOff    int
	stack       []viewKind

	width  int
	height int

	projects   []core.Project
	projectSel int
	projectOff int

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
	prompt     promptKind

	detail    *detailMsg
	detailOff int
	helpOff   int

	scheme    Scheme
	overrides map[string]string

	// prefs is what the settings screen reads and writes, prefSources names
	// the configuration layer each value arrived from, and savePrefs writes
	// a change down. A nil savePrefs is a session with nowhere to write.
	prefs       Preferences
	prefSources Preferences
	savePrefs   PreferenceWriter
	session     SessionInfo
	settingSel  int
	settingsOff int
	// autoColor is what the colour probe decided for this run, which the auto
	// mode resolves to. renderer and brand rebuild the theme when the colour
	// preference changes.
	autoColor bool
	renderer  *lipgloss.Renderer
	brand     core.Theme

	choice  choiceKind
	choices []Choice

	leases    map[string]string
	lastSeq   int64
	connected bool
	events    <-chan core.Event
	// gen names the connection the open subscription belongs to. Switching
	// tenants bumps it, so an event the previous tenant's stream was already
	// holding is dropped rather than drawn under the new tenant's name.
	gen int

	// tenantKey is the tenant this session is pinned to, and dialTenant opens
	// a connection pinned to another. dialTenant is nil when the caller gave
	// no way to dial, and the tenant view then says so rather than offering a
	// switch that cannot happen.
	tenantKey  string
	dialTenant TenantDialer
	// ownConn is the connection this session opened for itself by switching.
	// The connection it started with belongs to the caller, so it is never
	// closed here.
	ownConn TenantConn

	activity    []core.Event
	activitySel int
	activityOff int

	// activityFilter narrows the live tail. It is the audit filter the CLI
	// takes, parsed by the same grammar, minus the terms an event cannot
	// answer.
	activityFilterText string
	activityFilter     query.ActivityFilter
	activityFilterErr  string

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
	Out     io.Writer
	// Color overrides the environment probe. A caller whose destination is not
	// a file, an SSH session for instance, cannot be judged by asking whether
	// the writer is a character device, but it still knows whether the client
	// wants colour. Nil leaves the decision to ColorEnabled.
	Color *bool
	// Renderer draws this run's styles. lipgloss resolves a colour depth once
	// per renderer, so a caller serving several terminals at once gives each
	// one its own and no client can set another client's depth. Nil takes the
	// process-wide default, which is what a single local terminal wants.
	Renderer *lipgloss.Renderer
	// Brand is the tenant's resolved accent, from core's theme registry, so
	// a tenant presents one colour here and in a browser. The zero value
	// keeps the built-in accent, which is what an unthemed tenant gets.
	Brand core.Theme
	// Scheme names the keybinding preset, and Overrides rebinds single
	// actions on top of it.
	Scheme    string
	Overrides map[string]string
	// Prefs are the display settings the settings screen offers, Sources
	// names the configuration layer each one arrived from, and SavePrefs
	// writes a change back. A nil SavePrefs leaves the screen usable and
	// says the choices last only for the session.
	Prefs     Preferences
	Sources   Preferences
	SavePrefs PreferenceWriter
	// Session is what the settings screen answers "what am I connected to"
	// with. It is read-only: the target belongs to configuration.
	Session SessionInfo
	// TimeStyle renders every timestamp the interface draws. The zero value
	// still works, so a caller that has not wired configuration through yet
	// is not broken.
	TimeStyle output.TimeStyle
	Project   string
	Filter    string
	// Tenant names the tenant the supplied service is pinned to, and Dial
	// opens a connection pinned to another one. A nil Dial leaves the tenant
	// view read-only.
	Tenant string
	Dial   TenantDialer
	Now    func() time.Time
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
	auto := colorChoice(cfg)
	prefs := cfg.Prefs
	if strings.TrimSpace(prefs.Keymap) == "" {
		prefs.Keymap = cfg.Scheme
	}
	m := Model{
		svc: cfg.Service, ctx: ctx, actor: cfg.Actor,
		keys: DefaultKeyMap(), theme: NewTheme(cfg.Renderer, auto, cfg.Brand), now: now,
		timeStyle: cfg.TimeStyle,
		view:      viewProjects, input: in, leases: map[string]string{},
		openProject: cfg.Project, width: 80, height: 24,
		scheme: SchemeDefault, overrides: cfg.Overrides,
		tenantKey: cfg.Tenant, dialTenant: cfg.Dial,
		prefs: prefs, prefSources: cfg.Sources, savePrefs: cfg.SavePrefs,
		session:   cfg.Session,
		autoColor: auto, renderer: cfg.Renderer, brand: cfg.Brand,
	}
	// The probe decides only when no colour mode was configured, so a reader
	// who asked for colour over a pipe still gets it.
	m.theme = NewTheme(cfg.Renderer, ColorFor(prefs.Color, auto), cfg.Brand)
	m = m.installScheme(prefs.Keymap)
	if cfg.Filter != "" {
		m = m.applyFilterText(cfg.Filter)
	}
	return m
}

// installScheme adopts the configured keys, reporting a scheme or an override
// it cannot use rather than falling back to the defaults in silence.
func (m Model) installScheme(name string) Model {
	scheme, err := ParseScheme(name)
	if err != nil {
		m.err = err.Error()
		return m
	}
	keys, err := KeyMapFrom(scheme, m.overrides)
	if err != nil {
		m.err = "keybindings: " + err.Error()
		return m
	}
	m.scheme, m.keys = scheme, keys
	m.prefs.Keymap = string(scheme)
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
	case tenantMsg:
		return m.onTenant(msg)
	case statsMsg:
		return m.applyStats(msg), nil
	case projectsMsg:
		return m.onProjects(msg)
	case boardMsg:
		return m.onBoard(msg)
	case tasksMsg:
		return m.onTasks(msg)
	case detailMsg:
		m.detail, m.detailOff = &msg, 0
		return m.enterView(viewDetail), nil
	case eventMsg:
		return m.onEvent(msg)
	case streamMsg:
		return m.onStream(msg)
	case reconnectMsg:
		if msg.gen != m.gen {
			return m, nil
		}
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
	m.project, m.workflow = msg.project, msg.workflow
	return m.enterView(viewBoard).installTasks(msg.tasks)
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
	if msg.gen != m.gen {
		return m, nil
	}
	if msg.event.Seq > m.lastSeq {
		m.lastSeq = msg.event.Seq
	}
	m.connected = true
	m = m.recordActivity(msg.event)
	if m.events == nil {
		return m, m.reloadFor(msg.event)
	}
	return m, tea.Batch(nextEvent(m.events, m.gen), m.reloadFor(msg.event))
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
	if msg.gen != m.gen {
		return m, nil
	}
	if msg.connected && msg.events != nil {
		m.connected, m.events, m.status = true, msg.events, "connected"
		return m, nextEvent(msg.events, m.gen)
	}
	m.connected, m.events = false, nil
	if msg.err != nil {
		m.err = "event stream: " + msg.err.Error()
	}
	return m, reconnect(m.gen)
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
	case actionRelease:
		delete(m.leases, msg.ref.ID)
	}
	m.status, m.err = msg.statusText(), ""
	if msg.kind == actionNewProject {
		return m, m.loadProjects()
	}
	if m.project.ID == "" {
		return m, nil
	}
	return m, m.reloadAfter(msg)
}

// reloadAfter fetches whatever the action can have changed, including the open
// task, so the detail view never shows a value the action has just replaced.
func (m Model) reloadAfter(msg actionMsg) tea.Cmd {
	if m.view == viewDetail && m.detail != nil && msg.kind.Mutates() {
		return tea.Batch(m.loadTasks(), m.reloadDetail(m.detail.task))
	}
	return m.loadTasks()
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
	case m.prompt != promptNone:
		return m.handlePromptKey(msg)
	case m.choice != choiceNone:
		return m.handleChoiceKey(msg)
	case m.view == viewHelp:
		return m.handleHelpKey(msg)
	case m.view == viewSettings:
		return m.handleSettingsKey(msg)
	case key.Matches(msg, m.keys.Help):
		m.helpOff = 0
		return m.enterView(viewHelp), nil
	case key.Matches(msg, m.keys.Quit):
		return m.leave(tea.Quit)
	case key.Matches(msg, m.keys.Refresh):
		return m, m.refresh()
	case key.Matches(msg, m.keys.Projects):
		return m.rootView(), nil
	case key.Matches(msg, m.keys.Settings):
		return m.openSettings(), nil
	case key.Matches(msg, m.keys.Activity):
		return m.openActivity(), nil
	case key.Matches(msg, m.keys.Stats):
		return m.openStats()
	case key.Matches(msg, m.keys.Tenant):
		return m.openTenant(), nil
	}
	switch m.view {
	case viewProjects:
		return m.handleProjectsKey(msg)
	case viewBoard:
		return m.handleBoardKey(msg)
	case viewDetail:
		return m.handleDetailKey(msg)
	case viewActivity:
		return m.handleActivityKey(msg)
	case viewTenant:
		return m.handleTenantKey(msg)
	case viewStats:
		return m.handleStatsKey(msg)
	}
	return m, nil
}

// handleHelpKey scrolls the help view and dismisses it back to where it was
// opened from.
func (m Model) handleHelpKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back, m.keys.Help, m.keys.Quit):
		m.helpOff = 0
		m, _ = m.popView()
	case key.Matches(msg, m.keys.Up):
		m.helpOff = max(0, m.helpOff-1)
	case key.Matches(msg, m.keys.Down):
		m.helpOff++
	case key.Matches(msg, m.keys.Top):
		m.helpOff = 0
	}
	return m, nil
}

// enterView moves one level deeper, remembering where it came from. Entering
// the view already open is a refresh rather than a step, so a reload never
// makes the way back one press longer.
func (m Model) enterView(v viewKind) Model {
	if m.view == v {
		return m
	}
	m.stack = append(append([]viewKind{}, m.stack...), m.view)
	m.view = v
	return m
}

// popView returns to the view one level up, reporting whether there was one.
func (m Model) popView() (Model, bool) {
	if len(m.stack) == 0 {
		return m, false
	}
	m.view = m.stack[len(m.stack)-1]
	m.stack = m.stack[:len(m.stack)-1]
	if m.view != viewDetail {
		m.detail = nil
	}
	return m, true
}

// underView names the view the open one was reached from, which is what help
// describes when it is asked about "this view".
func (m Model) underView() viewKind {
	if len(m.stack) == 0 {
		return m.view
	}
	return m.stack[len(m.stack)-1]
}

// rootView returns to the project list, discarding the way back.
func (m Model) rootView() Model {
	m.view, m.stack, m.detail = viewProjects, nil, nil
	return m
}

// leave goes back one level, and does what the caller asked only when there is
// no level to go back to. Quitting from a nested view would otherwise lose a
// place a reflex press was only meant to step out of.
func (m Model) leave(fallback tea.Cmd) (Model, tea.Cmd) {
	if next, ok := m.popView(); ok {
		return next, nil
	}
	return m, fallback
}

// openSettings shows the display preferences and what this session is
// connected to, starting on the first setting.
func (m Model) openSettings() Model {
	m.settingSel, m.settingsOff = 0, 0
	return m.enterView(viewSettings)
}

// settingsState is what the settings screen renders from.
func (m Model) settingsState() SettingsState {
	return SettingsState{
		Prefs: m.prefs, Sources: m.prefSources, Session: m.sessionInfo(),
		Selected: m.settingSel, Persistent: m.savePrefs != nil,
		ColorAuto: m.autoColor, Now: m.now(),
	}
}

// sessionInfo fills in the facts the interface knows better than its caller:
// the tenant and the actor both change when a session switches tenant.
func (m Model) sessionInfo() SessionInfo {
	info := m.session
	if m.tenantKey != "" {
		info.Tenant = m.tenantKey
	}
	if m.actor != nil && m.actor.Handle != "" {
		info.Actor = "@" + m.actor.Handle
	}
	return info
}

// handleSettingsKey moves between the settings and cycles the selected one.
// A change applies to the frame it is read in and is written down at once,
// because a preference that lasts until the next restart is worse than none.
func (m Model) handleSettingsKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back, m.keys.Quit):
		m, _ = m.popView()
	case key.Matches(msg, m.keys.Up):
		m.settingSel, m.settingsOff = m.moveSettings(-1)
	case key.Matches(msg, m.keys.Down):
		m.settingSel, m.settingsOff = m.moveSettings(1)
	case key.Matches(msg, m.keys.Top):
		m.settingSel, m.settingsOff = 0, 0
	case key.Matches(msg, m.keys.Bottom):
		lines, rows := m.settingsBody()
		m.settingSel, m.settingsOff = SettingCount-1, max(0, len(lines)-rows)
	case key.Matches(msg, m.keys.Left):
		return m.cycleSetting(-1), nil
	case key.Matches(msg, m.keys.Right, m.keys.Enter):
		return m.cycleSetting(1), nil
	}
	return m, nil
}

// settingsBody is the screen's lines and how many of them fit.
func (m Model) settingsBody() ([]SettingsLine, int) {
	lines := SettingsView(m.settingsState())
	height := LayoutFor(m.width, m.height, len(m.columns)).BodyHeight
	return lines, VisibleRows(height, len(lines))
}

// moveSettings steps the cursor or the window by one line.
func (m Model) moveSettings(delta int) (int, int) {
	lines, rows := m.settingsBody()
	return MoveSettings(lines, m.settingSel, m.settingsOff, delta, rows)
}

// cycleSetting steps the selected setting and adopts the result, leaving the
// value alone when the build cannot render what it would become.
func (m Model) cycleSetting(delta int) Model {
	settings := SettingsFor(m.prefs)
	if m.settingSel < 0 || m.settingSel >= len(settings) {
		return m
	}
	set := settings[m.settingSel]
	value := CycleValue(set.Options, m.prefs.Value(m.settingSel), delta)
	return m.usePreferences(m.prefs.With(m.settingSel, value), set, value)
}

// usePreferences adopts a changed preference set and persists it, reporting a
// value the interface cannot render rather than taking it on.
func (m Model) usePreferences(next Preferences, set Setting, value string) Model {
	keys, style, err := ApplyPreference(next, m.overrides)
	if err != nil {
		m.err = set.Key + " cannot be " + value + ": " + err.Error()
		return m
	}
	scheme, _ := ParseScheme(next.Keymap)
	m.prefs, m.keys, m.timeStyle, m.scheme = next, keys, style, scheme
	m.theme = NewTheme(m.renderer, ColorFor(next.Color, m.autoColor), m.brand)
	m.err = ""
	var saveErr error
	if m.savePrefs != nil {
		saveErr = m.savePrefs(next)
	}
	m.status = SaveNote(set, value, m.session.ConfigFile, saveErr, m.savePrefs != nil)
	return m
}

// useScheme adopts a keybinding scheme, which is the settings screen's keymap
// row reached by name rather than by cycling.
func (m Model) useScheme(scheme Scheme) Model {
	settings := SettingsFor(m.prefs)
	return m.usePreferences(m.prefs.With(SettingKeymap, string(scheme)),
		settings[SettingKeymap], string(scheme))
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
	case key.Matches(msg, m.keys.New):
		return m.openPrompt(promptNewProject)
	case key.Matches(msg, m.keys.Enter):
		if m.projectSel < len(m.projects) {
			m.err = ""
			return m, m.loadBoard(m.projects[m.projectSel])
		}
	}
	m.projectOff = ScrollWindow(m.projectOff, m.projectSel, m.projectRows(), len(m.projects))
	return m, nil
}

// projectRows is how many project rows the current terminal has room for.
func (m Model) projectRows() int {
	// The header and its blank line are body lines too. Budgeting the whole
	// body for rows drew more of them than the frame had, which the scroll
	// guard caught: the count and the rows have to come out of one total.
	body := LayoutFor(m.width, m.height, len(m.columns)).BodyHeight - projectHeaderLines
	if body < 1 {
		body = 1
	}
	return VisibleRows(body, len(m.projects))
}

// projectHeaderLines is how many body lines the projects header occupies.
const projectHeaderLines = 2

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
	case key.Matches(msg, m.keys.Back):
		return m.leave(nil)
	case key.Matches(msg, m.keys.Enter):
		return m, m.loadDetail()
	default:
		return m.handleTaskKey(msg)
	}
	m.rowOff = ScrollWindow(m.rowOff, m.sel.Row, m.cardRows(), m.columnLength(m.sel.Col))
	return m, nil
}

// cardRows is how many cards the selected column has room to draw.
func (m Model) cardRows() int {
	return VisibleRows(LayoutFor(m.width, m.height, len(m.columns)).CardRows(), m.columnLength(m.sel.Col))
}

// columnLength is how many cards the selected column holds.
func (m Model) columnLength(col int) int {
	if col < 0 || col >= len(m.columns) {
		return 0
	}
	return len(m.columns[col].Tasks)
}

// handleDetailKey scrolls the detail view and returns to the board.
func (m Model) handleDetailKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		return m.leave(nil)
	case key.Matches(msg, m.keys.Up):
		m.detailOff = max(0, m.detailOff-1)
	case key.Matches(msg, m.keys.Down):
		m.detailOff++
	default:
		return m.handleTaskKey(msg)
	}
	return m, nil
}

// handleTaskKey runs the actions that act on the selected task, so the board
// and the detail view offer exactly the same set.
func (m Model) handleTaskKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Claim):
		return m, m.claim()
	case key.Matches(msg, m.keys.Release):
		return m.release()
	case key.Matches(msg, m.keys.Transition):
		return m.startChoosing(choiceTransition), nil
	case key.Matches(msg, m.keys.Priority):
		return m.startChoosing(choicePriority), nil
	case key.Matches(msg, m.keys.New):
		return m.openPrompt(promptNewTask)
	case key.Matches(msg, m.keys.EditTitle):
		return m.openPrompt(promptTitle)
	case key.Matches(msg, m.keys.EditBody):
		return m.openPrompt(promptBody)
	case key.Matches(msg, m.keys.Assign):
		return m.openPrompt(promptAssignee)
	case key.Matches(msg, m.keys.Comment):
		return m.openPrompt(promptComment)
	case key.Matches(msg, m.keys.Tag):
		return m.openPrompt(promptTag)
	case key.Matches(msg, m.keys.Untag):
		return m.openPrompt(promptUntag)
	case key.Matches(msg, m.keys.Depend):
		return m.openPrompt(promptDependency)
	case key.Matches(msg, m.keys.ClaimNext):
		return m, m.claimNext()
	case key.Matches(msg, m.keys.Renew):
		return m.renewLease()
	}
	return m, nil
}

// handlePromptKey edits the open input and acts on it when it is accepted.
func (m Model) handlePromptKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		return m.closePrompt(), nil
	case key.Matches(msg, m.keys.Accept):
		return m.submitPrompt(strings.TrimSpace(m.input.Value()))
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// submitPrompt performs the action the open input was gathering text for. An
// empty input cancels, so a stray keystroke never sends a blank edit.
func (m Model) submitPrompt(text string) (Model, tea.Cmd) {
	if m.prompt == promptFilter {
		next := m.applyFilterText(m.input.Value())
		if next.filterErr != "" {
			return next, nil
		}
		return next.closePrompt(), next.loadTasks()
	}
	if m.prompt == promptActivityFilter {
		next := m.applyActivityFilterText(m.input.Value())
		if next.activityFilterErr != "" {
			return next, nil
		}
		return next.closePrompt(), nil
	}
	if text == "" {
		return m.closePrompt(), nil
	}
	if m.prompt == promptTenant {
		return m.submitTenant(text)
	}
	kind := m.prompt
	next := m.closePrompt()
	return next, next.runPrompt(kind, text)
}

// runPrompt turns accepted text into the one service call it stands for.
func (m Model) runPrompt(kind promptKind, text string) tea.Cmd {
	switch kind {
	case promptNewTask:
		return m.createTask(text)
	case promptNewProject:
		key, name := SplitProjectEntry(text)
		return m.createProject(key, name)
	}
	task, ok := m.selectedTask()
	if !ok {
		return nil
	}
	switch kind {
	case promptTitle:
		return m.updateTask(task, core.UpdateTaskInput{Title: &text})
	case promptBody:
		return m.updateTask(task, core.UpdateTaskInput{Body: &text})
	case promptAssignee:
		return m.updateTask(task, core.UpdateTaskInput{AssigneeActorID: &text})
	case promptComment:
		return m.comment(task, text)
	case promptTag:
		return m.tag(task, text)
	case promptUntag:
		return m.untag(task, text)
	case promptDependency:
		return m.depend(task, text)
	default:
		return nil
	}
}

// openPrompt focuses an input, refusing one that needs a task when none is
// selected so the interface never gathers text it cannot use.
func (m Model) openPrompt(kind promptKind) (Model, tea.Cmd) {
	if kind.NeedsTask() {
		if _, ok := m.selectedTask(); !ok {
			m.err = "no task is selected"
			return m, nil
		}
	}
	if kind == promptNewTask && m.project.Key == "" {
		m.err = "open a project before adding a task"
		return m, nil
	}
	return m.startPrompt(kind, m.promptSeed(kind)), textinput.Blink
}

// promptSeed prefills an input with the value the action would replace.
func (m Model) promptSeed(kind promptKind) string {
	switch kind {
	case promptFilter:
		return m.filterText
	case promptActivityFilter:
		return m.activityFilterText
	}
	task, ok := m.selectedTask()
	if !ok {
		return ""
	}
	switch kind {
	case promptTitle:
		return task.Title
	case promptBody:
		return strings.ReplaceAll(task.Body, "\n", " ")
	case promptAssignee:
		return task.AssigneeActorID
	default:
		return ""
	}
}

// startPrompt focuses an input introduced by its own spec.
func (m Model) startPrompt(kind promptKind, initial string) Model {
	spec, ok := kind.Spec()
	if !ok {
		return m
	}
	m.prompt, m.err = kind, ""
	m.input.Prompt, m.input.Placeholder, m.input.CharLimit = spec.Prompt, spec.Placeholder, spec.Limit
	m.input.SetValue(initial)
	m.input.CursorEnd()
	m.input.Focus()
	return m
}

// closePrompt dismisses the open input.
func (m Model) closePrompt() Model {
	m.prompt, m.filterErr, m.activityFilterErr = promptNone, "", ""
	m.input.SetValue("")
	m.input.Blur()
	return m
}

// handleChoiceKey picks a numbered option.
func (m Model) handleChoiceKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Cancel) {
		m.choice, m.choices = choiceNone, nil
		return m, nil
	}
	picked, ok := ChoiceAt(m.choices, msg.String())
	if !ok {
		return m, nil
	}
	kind := m.choice
	m.choice, m.choices = choiceNone, nil
	return m, m.runChoice(kind, picked)
}

// runChoice turns a picked option into the one service call it stands for.
func (m Model) runChoice(kind choiceKind, picked Choice) tea.Cmd {
	if kind == choiceTransition {
		return m.transition(picked.Value)
	}
	task, ok := m.selectedTask()
	if !ok {
		return nil
	}
	value, err := strconv.Atoi(picked.Value)
	if err != nil {
		return nil
	}
	priority := core.Priority(value)
	return m.updateTask(task, core.UpdateTaskInput{Priority: &priority})
}

// startEditing focuses the filter bar on the current expression.
func (m Model) startEditing() Model {
	return m.startPrompt(promptFilter, m.filterText)
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

// startChoosing offers the options a picker has, refusing one that has none.
func (m Model) startChoosing(kind choiceKind) Model {
	task, ok := m.selectedTask()
	if !ok {
		m.err = "no task is selected"
		return m
	}
	choices := PriorityChoices()
	if kind == choiceTransition {
		choices = TransitionChoices(m.workflow, task.Status)
		if len(choices) == 0 {
			m.err = "no transition is permitted from " + task.Status
			return m
		}
	}
	m.choice, m.choices, m.err = kind, choices, ""
	return m
}

// renewLease refuses when this session does not hold the task's lease.
func (m Model) renewLease() (Model, tea.Cmd) {
	task, ok := m.selectedTask()
	if !ok {
		return m, nil
	}
	if m.leases[task.ID] == "" {
		m.err = "cannot renew: this session does not hold a lease on " + task.Ref
		return m, nil
	}
	return m, m.renew(task)
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

// actionContext describes the selected task so the footer only offers keys
// that will work on it.
func (m Model) actionContext() ActionContext {
	ctx := ActionContext{HasProject: m.project.Key != ""}
	task, ok := m.selectedTask()
	if !ok {
		return ctx
	}
	ctx.HasTask = true
	ctx.CanTransition = len(TransitionChoices(m.workflow, task.Status)) > 0
	if !task.ClaimedAtTime(m.now()) {
		return ctx
	}
	if m.leases[task.ID] != "" {
		ctx.HeldHere = true
		return ctx
	}
	ctx.HeldElsewhere = true
	return ctx
}

// holderNote names the worker holding the selected task, which the footer says
// instead of offering a lease key that would be refused.
func (m Model) holderNote() string {
	task, ok := m.selectedTask()
	if !ok || !task.ClaimedAtTime(m.now()) || m.leases[task.ID] != "" {
		return ""
	}
	return "held by " + shortID(task.ClaimedByActorID)
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

// actorID is the reader's own actor identifier, or empty when the session has
// none. Used to tell a card this reader holds from one somebody else does.
func (m Model) actorID() string {
	if m.actor == nil {
		return ""
	}
	return m.actor.ID
}
