// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"
	"io"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/query"
)

// Model is the whole state of the terminal interface.
type Model struct {
	svc   core.Service
	ctx   context.Context
	actor *core.Actor
	// access is which views this reader is offered, resolved once from the
	// actor's authority. Nothing recomputes it per keystroke.
	access ViewAccess
	keys   KeyMap
	theme  Theme
	now    func() time.Time
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
	// area is the multi-line field. A body and a comment are prose, and prose
	// crammed into one line is clipped at the terminal edge with no way to see
	// the end of it and no way to type a newline.
	area   textarea.Model
	prompt promptKind

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
	// mode resolves to. profile and brand rebuild the theme when the colour
	// preference changes.
	autoColor bool
	profile   colorprofile.Profile
	brand     core.Theme

	// pulseQuiet is the low phase of the selected row's pulse, pulsing reports
	// whether a phase is in flight, and lastInput is when a key last arrived,
	// which is what the idle pause measures. Nothing else schedules a repaint
	// on a timer, so a session that stops pulsing sends nothing at all.
	pulseQuiet bool
	pulsing    bool
	lastInput  time.Time

	choice  choiceKind
	choices []Choice

	// paletteOpen is the command palette, which takes the keyboard the way a
	// prompt does. paletteSel indexes the entries the query leaves and
	// paletteOff scrolls them; the query itself is the model's own input field.
	paletteOpen bool
	paletteSel  int
	paletteOff  int

	// form is the open multi-field input and confirm the destructive action
	// waiting for agreement. Only one of them is ever open: a form that ends in
	// a destructive call hands over to the confirmation and closes.
	form    Form
	confirm Confirm
	// commentSel indexes the open task's comment thread, which is what the
	// comment actions act on.
	commentSel int
	// setup is the project screen's subject, which is not always the project the
	// board has open: the listing offers the screen for the row under the
	// cursor. setupOff scrolls it, and pending carries a custom field between
	// the prompt that names it and the form that defines it.
	setup    *projectMsg
	setupOff int
	pending  pendingField
	// allowed is which operations this reader may perform, resolved once from
	// the actor. Nothing recomputes it per keystroke.
	allowed ActionAccess

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
	// admin is what the tenant screen states, adminSel indexes the domains and
	// the members as one list, and adminOff scrolls the screen.
	admin    *tenantInfoMsg
	adminSel int
	adminOff int
	// ownConn is the connection this session opened for itself by switching.
	// The connection it started with belongs to the caller, so it is never
	// closed here.
	ownConn TenantConn

	activity    []core.Event
	activitySel int
	activityOff int

	// history is the page of the durable log the history view draws, with the
	// subject it was read for, the handles its actors resolved to, and whether
	// the log runs past the page. It is separate from activity because the two
	// answer different questions from different sources under different scopes.
	history       []core.AuditEntry
	historySubj   HistorySubject
	historyActors map[string]string
	historyMore   bool
	historySel    int
	historyOff    int

	// actorsFor names the form a directory read was started for, because one
	// read serves the assignee picker and the whole-task edit.
	actorsFor formKind
	// directory is the tenant's actors, as the assignee picker last read them.
	// A handle is what a reader picks and an identifier is what the service
	// takes, so the listing is kept rather than reduced to labels.
	directory []core.Actor

	// pendingArtifact is the name gathered by the prompt, held between it and
	// the form that classifies the artifact.
	pendingArtifact string

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
	// Access is which views this reader may enter, from capability.TUIAccess.
	// Leaving it out offers only the views that need no authority.
	Access  ViewAccess
	Environ []string
	Out     io.Writer
	// Color overrides the environment probe. A caller whose destination is not
	// a file, an SSH session for instance, cannot be judged by asking whether
	// the writer is a character device, but it still knows whether the client
	// wants colour. Nil leaves the decision to ColorEnabled.
	Color *bool
	// Profile is the colour depth this run's styles are flattened to. A caller
	// serving several terminals at once gives each one its own, so no client
	// can set another client's depth. The zero value keeps every colour at
	// full fidelity and leaves the flattening to the output layer, which is
	// what a single local terminal wants.
	Profile colorprofile.Profile
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
	area := textarea.New()
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
		svc: cfg.Service, ctx: ctx, actor: cfg.Actor, access: cfg.Access,
		keys: DefaultKeyMap(), theme: NewTheme(cfg.Profile, auto, cfg.Brand), now: now,
		timeStyle: cfg.TimeStyle,
		view:      viewProjects, input: in, area: area, leases: map[string]string{},
		openProject: cfg.Project, width: 80, height: 24,
		scheme: SchemeDefault, overrides: cfg.Overrides,
		tenantKey: cfg.Tenant, dialTenant: cfg.Dial,
		prefs: prefs, prefSources: cfg.Sources, savePrefs: cfg.SavePrefs,
		session:   cfg.Session,
		autoColor: auto, profile: cfg.Profile, brand: cfg.Brand,
		allowed: resolveActions(cfg.Actor),
	}
	// The probe decides only when no colour mode was configured, so a reader
	// who asked for colour over a pipe still gets it.
	m.theme = NewTheme(cfg.Profile, ColorFor(prefs.Color, auto), cfg.Brand)
	m = m.styleInput().installScheme(prefs.Keymap)
	m.lastInput = now()
	m.pulsing = MotionEnabled(prefs.Motion, m.theme.Color)
	if cfg.Filter != "" {
		m = m.applyFilterText(cfg.Filter)
	}
	return m
}

// styleInput draws the two text fields the way the rest of the frame is drawn.
// Both bubbles ship their own colours and their own reverse-video cursor,
// which a colourless run must not inherit: "NO_COLOR writes no escape
// anywhere" was true of everything this interface renders except the widgets
// it does not render itself.
//
// The multi-line field also arrives with a border character for a prompt, line
// numbers down its left edge and a blinking cursor, none of which belong in a
// panel that already says what is being asked.
func (m Model) styleInput() Model {
	m.area.Prompt = ""
	m.area.ShowLineNumbers = false
	m.area.EndOfBufferCharacter = ' '
	if m.theme.Color {
		return m
	}
	plain := lipgloss.NewStyle()
	state := textinput.StyleState{Text: plain, Placeholder: plain, Suggestion: plain, Prompt: plain}
	m.input.SetStyles(textinput.Styles{Focused: state, Blurred: state})
	// The bubble's own caret is drawn in reverse video, which is an escape
	// like any other. Where the frame carries none, the panel's rule and its
	// legend are what say the keyboard has been taken.
	m.input.SetVirtualCursor(false)
	area := textarea.StyleState{Base: plain, Text: plain, LineNumber: plain,
		CursorLineNumber: plain, CursorLine: plain, EndOfBuffer: plain,
		Placeholder: plain, Prompt: plain, Selection: plain}
	m.area.SetStyles(textarea.Styles{Focused: area, Blurred: area})
	m.area.SetVirtualCursor(false)
	return m
}

// The range a multi-line field is allowed to take. A short body reserving half
// the terminal is as wrong as a long one drawn into two lines.
const (
	MinProseLines = 3
	MaxProseLines = 10
)

// ProseHeight sizes a multi-line field: tall enough for what is in it, never
// taller than a third of the terminal, so the panel's legend stays the last
// line on screen.
func ProseHeight(lines, terminal int) int {
	return clamp(lines, MinProseLines, clamp(terminal/3, MinProseLines, MaxProseLines))
}

// fitArea sizes the multi-line field to what it holds and what the terminal
// has. It runs wherever the field's content or the terminal changes, because
// the widget draws its own height and cannot ask the frame for one.
func (m Model) fitArea() Model {
	m.area.SetWidth(max(MinProseWidth, m.width-DetailIndent-FormLabelWidth))
	m.area.SetHeight(ProseHeight(m.area.LineCount(), m.height))
	return m
}

// MinProseWidth is the narrowest a multi-line field is drawn, below which the
// terminal is already too small for the board.
const MinProseWidth = 16

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

// Init starts the project listing, the event subscription and, when this
// session animates at all, the first phase of the selection's pulse.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.loadProjects(), m.subscribe(m.lastSeq)}
	if m.pulsing {
		cmds = append(cmds, pulse())
	}
	return tea.Batch(cmds...)
}

// Update folds one message into the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m.reduce(msg) }

// reduce is the whole state machine, kept free of rendering.
func (m Model) reduce(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.onResize(msg), nil
	case tea.KeyPressMsg:
		next, cmd := m.handleKey(msg)
		return next.afterInput(cmd)
	case pulseMsg:
		return m.onPulse()
	case tenantMsg:
		return m.onTenant(msg)
	case statsMsg:
		return m.applyStats(msg), nil
	case tagsMsg:
		return m.onTags(msg)
	case actorsMsg:
		// One read serves two pickers. The tenant screen asks for the same
		// directory the assignee picker does, and which of them is waiting is
		// decided by the view that asked rather than by a second message type.
		if m.view == viewTenant {
			return m.onMemberDirectory(msg)
		}
		return m.onActors(msg)
	case tenantInfoMsg:
		return m.onTenantInfo(msg)
	case historyMsg:
		return m.onHistory(msg)
	case projectMsg:
		m.setup, m.setupOff = &msg, 0
		return m.enterView(viewProject), nil
	case projectsMsg:
		return m.onProjects(msg)
	case boardMsg:
		return m.onBoard(msg)
	case tasksMsg:
		return m.onTasks(msg)
	case detailMsg:
		m.detail, m.detailOff = &msg, 0
		m.commentSel = clamp(m.commentSel, 0, len(msg.comments)-1)
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

// afterInput records the keystroke the idle pause is measured from, and arms
// the pulse again when the pause had stopped it or when this keystroke is the
// one that turned the motion on.
func (m Model) afterInput(cmd tea.Cmd) (Model, tea.Cmd) {
	m.lastInput = m.now()
	if m.pulsing || !MotionEnabled(m.prefs.Motion, m.theme.Color) {
		return m, cmd
	}
	m.pulsing = true
	return m, tea.Batch(cmd, pulse())
}

// onPulse changes the selected row's emphasis and asks for the next phase,
// unless the reader has gone idle or turned the motion off. Returning no
// command ends the chain, and with it every frame this session would have
// sent while nobody was there.
func (m Model) onPulse() (Model, tea.Cmd) {
	if !PulseRunning(m.prefs.Motion, m.theme.Color, m.now().Sub(m.lastInput)) {
		m.pulsing, m.pulseQuiet = false, false
		return m, nil
	}
	m.pulseQuiet = !m.pulseQuiet
	return m, pulse()
}

// selection is the style this frame draws the selected row in. The marker is
// not part of it: the pulse varies emphasis and never removes the cue.
func (m Model) selection() lipgloss.Style {
	return m.theme.Selection(!m.pulseQuiet)
}

// onResize adopts a new terminal size, ignoring a size the terminal cannot report.
func (m Model) onResize(msg tea.WindowSizeMsg) Model {
	if msg.Width <= 0 || msg.Height <= 0 {
		return m
	}
	m.width, m.height = msg.Width, msg.Height
	return m.fitArea()
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
	// The project screen's subject is not always the board's project, so it is
	// asked about before the board's own filter narrows the event away.
	if m.view == viewProject && m.setup != nil &&
		(e.ProjectID == "" || e.ProjectID == m.setup.project.ID) {
		return m.loadProject(m.setup.project.Key)
	}
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
	// A deleted task has no detail view to go back to, so the interface leaves
	// it rather than fetching a task the service has just removed and reporting
	// the "not found" as a failure of its own.
	if msg.kind == actionDelete && m.view == viewDetail {
		m, _ = m.popView()
		m.detail, m.commentSel = nil, 0
	}
	// A deleted project has no setup screen to go back to, so the interface
	// leaves it rather than fetching a project the service has just removed.
	if msg.kind == actionDeleteProject {
		next := m.rootView()
		next.setup = nil
		return next, next.loadProjects()
	}
	if msg.kind.ChangesTenant() {
		return m, m.loadTenantInfo()
	}
	if msg.kind.ChangesSetup() && m.setupRef() != "" {
		return m, tea.Batch(m.loadProject(m.setupRef()), m.loadProjects())
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
		// No guess at which conflict it was. KindConflict covers a held
		// claim, a version clash and a task already in a terminal state, and
		// naming one of them produced a sentence that argued with itself:
		// "cannot claim: the task is already claimed by another worker:
		// conflict: task ops-1 is already finished in status done". The
		// service already says which conflict it is.
		return "cannot " + msg.kind.Label() + ": " + msg.err.Error()
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
func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Interrupt):
		m.interrupted = true
		return m, tea.Quit
	case m.confirm.Open():
		return m.handleConfirmKey(msg)
	case m.form.Open():
		return m.handleFormKey(msg)
	case m.prompt != promptNone:
		return m.handlePromptKey(msg)
	case m.choice != choiceNone:
		return m.handleChoiceKey(msg)
	case m.paletteOpen:
		return m.handlePaletteKey(msg)
	case m.view == viewHelp:
		return m.handleHelpKey(msg)
	case m.view == viewSettings:
		return m.handleSettingsKey(msg)
	}
	// The cross-view keys resolve through the one action list rather than
	// through a switch of their own. A key with a case here and no entry there
	// would be a key the palette could not offer, and an entry with no case
	// would be a palette row that did nothing.
	if a, ok := ResolveAction(msg, m.keys.globalKeys()); ok {
		if next, cmd, handled := m.performAction(a.id); handled {
			return next, cmd
		}
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
	case viewProject:
		return m.handleProjectKey(msg)
	case viewHistory:
		return m.handleHistoryKey(msg)
	}
	return m, nil
}

// handleHelpKey scrolls the help view and dismisses it back to where it was
// opened from.
func (m Model) handleHelpKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
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
//
// A view this reader is not offered is not entered. This is the only door into
// a view, so the refusal cannot be walked around by a new call site, and a
// sandbox visitor and an enrolled administrator run the same code here.
func (m Model) enterView(v viewKind) Model {
	if m.view == v || !m.canReach(v) {
		return m
	}
	m.stack = append(append([]viewKind{}, m.stack...), m.view)
	m.view = v
	// A refusal belongs to the screen that earned it. It used to survive every
	// move, so "cannot claim: ..." from the board was still on the statistics
	// screen and the settings screen, describing an action taken somewhere
	// else entirely.
	m.err = ""
	return m
}

// popView returns to the view one level up, reporting whether there was one.
func (m Model) popView() (Model, bool) {
	if len(m.stack) == 0 {
		return m, false
	}
	m.view = m.stack[len(m.stack)-1]
	m.stack = m.stack[:len(m.stack)-1]
	m.err = ""
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
func (m Model) handleSettingsKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
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
	m.theme = NewTheme(m.profile, ColorFor(next.Color, m.autoColor), m.brand)
	m = m.styleInput()
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
func (m Model) handleProjectsKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
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
func (m Model) handleBoardKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
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
func (m Model) handleDetailKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		return m.leave(nil)
	case key.Matches(msg, m.keys.Left):
		m.commentSel = m.moveComment(-1)
	case key.Matches(msg, m.keys.Right):
		m.commentSel = m.moveComment(1)
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
func (m Model) handleTaskKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	a, ok := ResolveAction(msg, m.keys.taskBindings())
	if !ok {
		return m, nil
	}
	// The gate is asked here as well as by the footer, which is what keeps the
	// two from disagreeing: a reader who finds the key by reading the source,
	// or by pressing it out of habit from another session, is refused here
	// rather than by the service.
	if !a.gate.permitted(m.permits()) {
		return m, nil
	}
	next, cmd, _ := m.performAction(a.id)
	return next, cmd
}

// performAction is the one implementation of every action the interface has. The
// key press reaches it by resolving the press to an action, and the palette
// reaches it with the entry the reader chose, so an action cannot be performed
// two slightly different ways.
//
// The returned bool reports that the id was wired to something, which is what
// TestEveryActionIsPerformed asks of every action in the list.
func (m Model) performAction(id actionID) (Model, tea.Cmd, bool) {
	switch id {
	case doPalette:
		return m.openPalette(), textinput.Blink, true
	case doHelp:
		m.helpOff = 0
		return m.enterView(viewHelp), nil, true
	case doRefresh:
		return m, m.refresh(), true
	case doProjects:
		return m.rootView(), nil, true
	case doProject:
		next, cmd := m.openProjectSetup()
		return next, cmd, true
	case doSettings:
		return m.openSettings(), nil, true
	case doActivity:
		return m.openActivity(), nil, true
	case doHistory:
		next, cmd := m.openHistory()
		return next, cmd, true
	case doStats:
		if !m.canReach(viewStats) {
			return m, nil, true
		}
		next, cmd := m.openStats()
		return next, cmd, true
	case doTenant:
		next, cmd := m.openTenant()
		return next, cmd, true
	case doQuit:
		next, cmd := m.leave(tea.Quit)
		return next, cmd, true
	case doInterrupt:
		m.interrupted = true
		return m, tea.Quit, true
	}
	return m.performTaskAction(id)
}

// performTaskAction is the half of performAction that acts on the selection,
// split out because one switch over thirty cases is past what a reader holds.
func (m Model) performTaskAction(id actionID) (Model, tea.Cmd, bool) {
	switch id {
	case doNewTask:
		next, cmd := m.openPrompt(promptNewTask)
		return next, cmd, true
	case doClaimNext:
		return m, m.claimNext(), true
	case doClaim:
		return m, m.claim(), true
	case doRelease:
		next, cmd := m.release()
		return next, cmd, true
	case doTransition:
		return m.startChoosing(choiceTransition), nil, true
	case doEdit:
		next, cmd := m.openTaskEditForm()
		return next, cmd, true
	case doPriority:
		return m.startChoosing(choicePriority), nil, true
	case doCyclePriority:
		next, cmd := m.cyclePriority()
		return next, cmd, true
	case doAssign:
		next, cmd := m.openAssigneeForm()
		return next, cmd, true
	case doComment:
		next, cmd := m.openPrompt(promptComment)
		return next, cmd, true
	case doCommentEdit:
		next, cmd := m.openPrompt(promptCommentEdit)
		return next, cmd, true
	case doTag:
		next, cmd := m.openPrompt(promptTag)
		return next, cmd, true
	case doUntag:
		next, cmd := m.openPrompt(promptUntag)
		return next, cmd, true
	case doTags:
		next, cmd := m.openTagForm()
		return next, cmd, true
	case doDepend:
		next, cmd := m.openPrompt(promptDependency)
		return next, cmd, true
	case doUndepend:
		next, cmd := m.openDependencyForm()
		return next, cmd, true
	case doDelete:
		next, cmd := m.openDeleteForm()
		return next, cmd, true
	case doRestore:
		next, cmd := m.restoreTask()
		return next, cmd, true
	case doArtifact:
		next, cmd := m.openPrompt(promptArtifact)
		return next, cmd, true
	case doRenew:
		next, cmd := m.renewLease()
		return next, cmd, true
	}
	return m, nil, false
}

// handlePromptKey edits the open input and acts on it when it is accepted.
//
// Enter does not accept a multi-line field. Inserting a newline is what the
// field is for, so the commit key is the one that ends it and the panel's
// legend names that key rather than leaving the reader to find it.
func (m Model) handlePromptKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	prose := m.promptMultiline()
	switch {
	case key.Matches(msg, m.keys.Cancel):
		return m.closePrompt(), nil
	case key.Matches(msg, m.keys.Commit):
		return m.submitPrompt(strings.TrimSpace(m.promptValue()))
	case m.accepts(msg, prose):
		return m.submitPrompt(strings.TrimSpace(m.promptValue()))
	}
	var cmd tea.Cmd
	if prose {
		m.area, cmd = m.area.Update(msg)
		return m.fitArea(), cmd
	}
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// accepts reports whether a press ends a free-text input. Enter inside a
// multi-line field inserts a newline, which is the whole point of the field, so
// it is the one press that does not accept; a scheme that puts accept on a
// chord as well keeps that chord working there.
func (m Model) accepts(msg tea.KeyPressMsg, prose bool) bool {
	if prose && msg.String() == "enter" {
		return false
	}
	return key.Matches(msg, m.keys.Accept)
}

// promptMultiline reports whether the open input gathers prose.
func (m Model) promptMultiline() bool {
	spec, ok := m.prompt.Spec()
	return ok && spec.Multiline
}

// promptValue is what the open input holds, from whichever of the two fields
// is gathering it.
func (m Model) promptValue() string {
	if m.promptMultiline() {
		return m.area.Value()
	}
	return m.input.Value()
}

// submitPrompt performs the action the open input was gathering text for. An
// empty input cancels, so a stray keystroke never sends a blank edit.
func (m Model) submitPrompt(text string) (Model, tea.Cmd) {
	if m.prompt == promptFilter {
		next := m.applyFilterText(m.promptValue())
		if next.filterErr != "" {
			return next, nil
		}
		return next.closePrompt(), next.loadTasks()
	}
	if m.prompt == promptActivityFilter {
		next := m.applyActivityFilterText(m.promptValue())
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
	if kind == promptNewField {
		return next.openFieldDefForm(text)
	}
	if kind == promptArtifact {
		return next.openArtifactForm(text)
	}
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
	case promptProjectName:
		return m.updateProject(core.UpdateProjectInput{Name: &text}, attrName)
	case promptProjectDesc:
		return m.updateProject(core.UpdateProjectInput{Description: &text}, attrDescription)
	case promptProjectIcon:
		return m.updateProject(core.UpdateProjectInput{Icon: &text}, attrIcon)
	case promptTenantName:
		return m.updateTenant(core.UpdateTenantInput{Name: &text}, attrName)
	case promptDomain:
		return m.addDomain(text)
	}
	task, ok := m.selectedTask()
	if !ok {
		return nil
	}
	switch kind {
	case promptComment:
		return m.comment(task, text)
	case promptCommentEdit:
		return m.editComment(text)
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
	if kind == promptCommentEdit {
		if _, ok := m.selectedComment(); !ok {
			m.err = m.noCommentReason()
			return m, nil
		}
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
	case promptCommentEdit:
		comment, ok := m.selectedComment()
		if !ok {
			return ""
		}
		// Not flattened. The field holds the newlines the comment was written
		// with, which is the whole reason it is more than one line.
		return comment.Body
	}
	return ""
}

// startPrompt focuses an input introduced by its own spec.
func (m Model) startPrompt(kind promptKind, initial string) Model {
	spec, ok := kind.Spec()
	if !ok {
		return m
	}
	m.prompt, m.err = kind, ""
	if spec.Multiline {
		m.area.Placeholder, m.area.CharLimit = spec.Placeholder, spec.Limit
		m.area.SetValue(initial)
		m.area.MoveToEnd()
		m.area.Focus()
		return m.fitArea()
	}
	// The panel names the input; the field carries only the value. A prompt
	// string here would repeat the panel's own title one line below it.
	m.input.Prompt, m.input.Placeholder, m.input.CharLimit = "", spec.Placeholder, spec.Limit
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
	m.area.SetValue("")
	m.area.Blur()
	return m
}

// handleChoiceKey picks a numbered option.
func (m Model) handleChoiceKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
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

// cyclePriority moves the selected task one place down the priority scale and
// sends it, with nothing to pick. It wraps at the bottom, so every priority is
// reachable from this one key; the picker on P is still the shorter trip to a
// priority the reader has already decided on.
func (m Model) cyclePriority() (Model, tea.Cmd) {
	task, ok := m.selectedTask()
	if !ok {
		m.err = "no task is selected"
		return m, nil
	}
	next := NextPriority(task.Priority)
	m.err = ""
	return m, m.updateTask(task, core.UpdateTaskInput{Priority: &next})
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
	ctx := ActionContext{HasProject: m.project.Key != "", May: m.permits(),
		HasComment: m.view == viewDetail && m.detail != nil && len(m.detail.comments) > 0,
		HasFields:  m.setup != nil && len(m.setup.fields) > 0,
		HasRows:    m.view == viewTenant && len(m.adminRows()) > 0}
	task, ok := m.selectedTask()
	if !ok {
		return ctx
	}
	ctx.HasTask = true
	ctx.IsDeleted = task.DeletedAt != nil
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
	if m.view == viewProject && m.setup != nil {
		return m.loadProject(m.setup.project.Key)
	}
	// The history is read once when the view opens, so refreshing anything else
	// from here would reload a screen the reader is not looking at.
	if m.view == viewHistory {
		return m.loadHistory(m.historySubj)
	}
	// The tenant screen is read once when the view opens, so refreshing
	// anything else from here would reload a screen the reader is not on.
	if m.view == viewTenant {
		return m.loadTenantInfo()
	}
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

// moveComment steps the cursor through the open task's thread.
func (m Model) moveComment(delta int) int {
	if m.detail == nil {
		return 0
	}
	return clamp(m.commentSel+delta, 0, len(m.detail.comments)-1)
}

// selectedComment is the comment the comment actions act on. Only the detail
// view has a thread, so only the detail view has one selected.
func (m Model) selectedComment() (core.Comment, bool) {
	if m.view != viewDetail || m.detail == nil {
		return core.Comment{}, false
	}
	if m.commentSel < 0 || m.commentSel >= len(m.detail.comments) {
		return core.Comment{}, false
	}
	return m.detail.comments[m.commentSel], true
}

// noCommentReason says why there is no comment to act on, which is a different
// answer on the board than on a task with an empty thread.
func (m Model) noCommentReason() string {
	switch {
	case m.view != viewDetail || m.detail == nil:
		return "open the task to work on its comments"
	case len(m.detail.comments) == 0:
		return "this task has no comments"
	default:
		return "no comment is selected"
	}
}

// commentTarget names the selected comment the way the reader sees it in the
// thread, which is what a confirmation has to say out loud.
func (m Model) commentTarget() string {
	comment, ok := m.selectedComment()
	if !ok {
		return ""
	}
	return CommentTarget(comment, m.detail.actors, m.timeStyle)
}

// openDeleteForm asks what to delete and how far the deletion should reach,
// offering only the subjects this reader may remove.
func (m Model) openDeleteForm() (Model, tea.Cmd) {
	task, ok := m.selectedTask()
	if !ok {
		m.err = "no task is selected"
		return m, nil
	}
	form := DeleteForm(task.Ref, m.commentTarget(),
		m.mayPerform("DeleteTask"), m.mayPerform("DeleteComment"))
	if !form.Open() {
		m.err = "nothing here can be deleted"
		return m, nil
	}
	m.form, m.err = form, ""
	return m, nil
}

// openDependencyForm offers the dependencies the open task waits on. The board
// knows a task has some and not which, so the form is offered where they are
// listed rather than where the marker is.
func (m Model) openDependencyForm() (Model, tea.Cmd) {
	if m.view != viewDetail || m.detail == nil {
		m.err = "open the task to see what it waits on"
		return m, nil
	}
	form := DependencyForm(dependencyRefs(m.detail.deps))
	if !form.Open() {
		m.err = "this task waits on nothing"
		return m, nil
	}
	m.form, m.err = form, ""
	return m, nil
}

// openTagForm fetches the tags the tenant has so the reader can pick one rather
// than remember one.
func (m Model) openTagForm() (Model, tea.Cmd) {
	if _, ok := m.selectedTask(); !ok {
		m.err = "no task is selected"
		return m, nil
	}
	return m, m.loadTags()
}

// onTags opens the tag form on a listing, or says the tenant has no tags yet
// rather than offering a picker with nothing to pick.
func (m Model) onTags(msg tagsMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.err = "listing tags: " + msg.err.Error()
		return m, nil
	}
	task, ok := m.selectedTask()
	if !ok {
		return m, nil
	}
	form := TagForm(tagNames(msg.tags), task.Tags, m.mayPerform("AddTag"), m.mayPerform("RemoveTag"))
	if !form.Open() {
		m.err = "no tags exist yet; " + m.keys.Tag.Help().Key + " adds one by name"
		return m, nil
	}
	m.form, m.err = form, ""
	return m, nil
}

// applyTaskEdit sends only what the reader changed. A form that resends every
// field it drew would record an edit to the title on a trip that touched the
// body, and the audit log is read.
func (m Model) applyTaskEdit(task core.Task, form Form) tea.Cmd {
	var in core.UpdateTaskInput
	if title := strings.TrimSpace(form.Value("title")); title != task.Title {
		in.Title = &title
	}
	if body := form.Value("body"); body != task.Body {
		in.Body = &body
	}
	if p, ok := PriorityFor(form.Value("priority")); ok && p != task.Priority {
		in.Priority = &p
	}
	if id := ActorIDFor(m.directory, form.Value("assignee")); form.Value("assignee") != "" &&
		id != task.AssigneeActorID {
		in.AssigneeActorID = &id
	}
	if len(updatedFields(in)) == 0 {
		return nil
	}
	return m.updateTask(task, in)
}

// openTaskEditForm gathers everything one update can change about a task on
// one screen, rather than asking for each of them in a separate trip. The
// directory is read first so the assignee is a colleague's handle rather than
// the identifier the service stores.
func (m Model) openTaskEditForm() (Model, tea.Cmd) {
	if _, ok := m.selectedTask(); !ok {
		m.err = "no task is selected"
		return m, nil
	}
	m.actorsFor = formTaskEdit
	return m, m.loadActors()
}

// openAssigneeForm reads the tenant's directory so the reader can pick a
// colleague rather than remember an identifier. The listing is fetched on the
// keystroke rather than held for the session, because who a tenant has changes
// while a board is open and a stale directory offers somebody who has left.
func (m Model) openAssigneeForm() (Model, tea.Cmd) {
	if _, ok := m.selectedTask(); !ok {
		m.err = "no task is selected"
		return m, nil
	}
	m.actorsFor = formAssignee
	return m, m.loadActors()
}

// onActors opens the assignee form on a directory, or says the tenant has
// nobody to assign to rather than offering a picker with nothing in it.
func (m Model) onActors(msg actorsMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.err = "listing actors: " + msg.err.Error()
		return m, nil
	}
	task, ok := m.selectedTask()
	if !ok {
		return m, nil
	}
	m.directory = msg.actors
	if m.actorsFor == formTaskEdit {
		return m.openForm(TaskEditForm(task, msg.actors))
	}
	form := AssigneeForm(msg.actors, task.AssigneeActorID)
	if !form.Open() {
		m.err = "this tenant has nobody to assign to"
		return m, nil
	}
	m.form, m.err = form, ""
	return m, nil
}

// openForm installs a form and hands the keyboard to its first field, so a form
// opening on a text field is typed into straight away.
func (m Model) openForm(form Form) (Model, tea.Cmd) {
	m.form, m.err = form, ""
	return m.focusField(), textinput.Blink
}

// focusField seeds whichever widget the selected field is gathered by, and
// blurs the other. A field that is cycled gathers nothing here.
func (m Model) focusField() Model {
	field, ok := m.form.Selected()
	m.input.Blur()
	m.area.Blur()
	if !ok || !field.Typed() {
		return m
	}
	if field.Kind == FieldProse {
		m.area.Placeholder, m.area.CharLimit = "", field.Limit
		m.area.SetValue(field.Value)
		m.area.MoveToEnd()
		m.area.Focus()
		return m.fitArea()
	}
	m.input.Prompt, m.input.Placeholder, m.input.CharLimit = "", "", field.Limit
	m.input.SetValue(field.Value)
	m.input.CursorEnd()
	m.input.Focus()
	return m
}

// stashField writes what the reader typed back into the form, so moving off a
// field keeps the answer and cancelling the form discards every one of them.
func (m Model) stashField() Model {
	field, ok := m.form.Selected()
	if !ok || !field.Typed() {
		return m
	}
	if field.Kind == FieldProse {
		m.form = m.form.SetValue(field.Key, m.area.Value())
		return m
	}
	m.form = m.form.SetValue(field.Key, m.input.Value())
	return m
}

// moveField carries the answer off the field being left and hands the keyboard
// to the one being arrived at.
func (m Model) moveField(delta int) (Model, tea.Cmd) {
	next := m.stashField()
	next.form = next.form.Move(delta)
	return next.focusField(), nil
}

// openArtifactForm classifies the artifact the prompt named.
func (m Model) openArtifactForm(name string) (Model, tea.Cmd) {
	if _, ok := m.selectedTask(); !ok {
		m.err = "no task is selected"
		return m, nil
	}
	m.pendingArtifact = name
	m.form, m.err = ArtifactForm(name), ""
	return m, nil
}

// restoreTask brings back a task a deletion took off the board, refusing a task
// that was never deleted rather than sending a call the service will refuse and
// saying where the deleted ones are.
func (m Model) restoreTask() (Model, tea.Cmd) {
	task, ok := m.selectedTask()
	if !ok {
		m.err = "no task is selected"
		return m, nil
	}
	if task.DeletedAt == nil {
		m.err = "only a deleted task can be restored; filter the board with is:deleted to see them"
		return m, nil
	}
	return m, m.restore(task)
}

// handleFormKey moves through the open form and submits it. It reuses the keys
// the settings screen moves and cycles with, so a form needs no bindings of its
// own and works under every scheme.
//
// A field the reader types into owns the arrows, because they move the cursor
// through the text, so tab and shift+tab are what move between fields. Enter
// applies the form everywhere except inside the multi-line field, where it
// inserts a newline and the commit key applies instead.
func (m Model) handleFormKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	typing, prose := m.form.Typing(), m.form.Prose()
	switch {
	case key.Matches(msg, m.keys.Cancel):
		return m.closeForm(), nil
	case key.Matches(msg, m.keys.Commit):
		return m.stashField().submitForm()
	case key.Matches(msg, m.keys.NextField):
		return m.moveField(1)
	case key.Matches(msg, m.keys.PrevField):
		return m.moveField(-1)
	case m.accepts(msg, prose):
		return m.stashField().submitForm()
	}
	if typing {
		return m.editField(msg)
	}
	switch {
	case key.Matches(msg, m.keys.Up):
		return m.moveField(-1)
	case key.Matches(msg, m.keys.Down):
		return m.moveField(1)
	case key.Matches(msg, m.keys.Left):
		m.form = m.form.Cycle(-1)
	case key.Matches(msg, m.keys.Right):
		m.form = m.form.Cycle(1)
	}
	return m, nil
}

// editField hands a keystroke to the widget the selected field is gathered by.
func (m Model) editField(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	if m.form.Prose() {
		m.area, cmd = m.area.Update(msg)
		return m.fitArea().stashField(), cmd
	}
	m.input, cmd = m.input.Update(msg)
	return m.stashField(), cmd
}

// closeForm dismisses the open form.
func (m Model) closeForm() Model {
	m.form = Form{}
	m.input.SetValue("")
	m.input.Blur()
	m.area.SetValue("")
	m.area.Blur()
	return m
}

// submitForm performs what the answered form asked for, which for a destructive
// form is to ask the question rather than to make the call.
func (m Model) submitForm() (Model, tea.Cmd) {
	form := m.form
	switch form.Kind {
	case formProject:
		return m.closeForm().applyProjectEdit(form)
	case formProjectRemove:
		return m.closeForm().confirmProjectRemoval(form)
	case formFieldPick:
		return m.closeForm().applyFieldPick(form)
	case formFieldDef:
		return m.closeForm().applyFieldDef(form)
	case formTenant:
		return m.closeForm().applyTenantEdit(form)
	case formTenantAdd:
		return m.closeForm().applyTenantAdd(form)
	case formMember:
		return m.closeForm().applyMember(form)
	}
	if form.Kind == formTaskEdit {
		next := m.closeForm()
		task, ok := next.selectedTask()
		if !ok {
			return next, nil
		}
		return next, next.applyTaskEdit(task, form)
	}
	if form.Kind == formAssignee {
		next := m.closeForm()
		task, ok := next.selectedTask()
		if !ok {
			return next, nil
		}
		id := ActorIDFor(next.directory, form.Value("assignee"))
		return next, next.updateTask(task, core.UpdateTaskInput{AssigneeActorID: &id})
	}
	task, ok := m.selectedTask()
	if !ok {
		return m.closeForm(), nil
	}
	next := m.closeForm()
	switch form.Kind {
	case formDelete:
		return next.openConfirm(form, task)
	case formTag:
		if form.Value("action") == "detach" {
			return next, next.untag(task, form.Value("tag"))
		}
		return next, next.tag(task, form.Value("tag"))
	case formDependency:
		return next, next.undepend(task, form.Value("dependency"))
	case formArtifact:
		return next, next.putArtifact(task, core.ArtifactInput{
			Kind: core.ArtifactKind(form.Value("kind")), Name: next.pendingArtifact,
		})
	}
	return next, nil
}

// openConfirm states what the delete form asked for and waits for agreement. A
// confirmation that cannot name its subject is not opened at all, because the
// naming is the whole of its value.
func (m Model) openConfirm(form Form, task core.Task) (Model, tea.Cmd) {
	confirm := Confirm{ref: core.TaskRef{ID: task.ID}, label: task.Ref}
	if form.Value("what") == "comment" {
		comment, ok := m.selectedComment()
		if !ok {
			m.err = m.noCommentReason()
			return m, nil
		}
		confirm.Kind, confirm.Target, confirm.commentID = confirmDeleteComment, m.commentTarget(), comment.ID
	} else {
		confirm.Kind, confirm.Target = confirmDeleteTask, task.Ref
		confirm.hard, confirm.cascade = form.Yes("hard"), form.Yes("cascade")
		confirm.Note = DeleteNote(confirm.hard, confirm.cascade)
	}
	if confirm.Question() == "" {
		m.err = "there is nothing named to delete"
		return m, nil
	}
	m.confirm, m.err = confirm, ""
	return m, nil
}

// handleConfirmKey answers the open confirmation. Every key that is not the
// agreement leaves it standing, so a stray press neither runs the action nor
// dismisses the question.
func (m Model) handleConfirmKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		return m.closeConfirm(), nil
	case key.Matches(msg, m.keys.Agree):
		confirm := m.confirm
		next := m.closeConfirm()
		return next, next.runConfirm(confirm)
	}
	return m, nil
}

// closeConfirm withdraws the question without running anything.
func (m Model) closeConfirm() Model {
	m.confirm = Confirm{}
	return m
}

// runConfirm makes the call the reader agreed to.
func (m Model) runConfirm(c Confirm) tea.Cmd {
	switch c.Kind {
	case confirmDeleteTask:
		return m.deleteTask(c)
	case confirmDeleteComment:
		return m.deleteComment(c)
	case confirmArchiveProject:
		return m.archiveProject(c.projectRef)
	case confirmDeleteProject:
		return m.deleteProject(c.projectRef)
	case confirmDeleteField:
		return m.deleteFieldDef(c.projectRef, c.fieldKey)
	case confirmRemoveDomain:
		return m.removeDomain(c.hostname)
	case confirmRemoveMember:
		return m.removeMember(c.actorID, c.Target)
	default:
		return nil
	}
}

// pendingField carries a custom field between the prompt that names it and the
// form that defines it. The key and the label are free text, which a form has no
// field kind for, so they are gathered first and held here.
type pendingField struct {
	key   string
	label string
	base  core.FieldDef
}

// openProjectSetup opens the screen that states how one project is set up: its
// own attributes, the workflow its tasks move through and the custom fields they
// carry. The subject is the row under the cursor on the listing and the open
// project anywhere else, so the key means the same thing wherever it is pressed.
func (m Model) openProjectSetup() (Model, tea.Cmd) {
	if !m.canReach(viewProject) {
		return m, nil
	}
	ref := m.setupTarget()
	if ref == "" {
		m.err = "open or select a project to see how it is set up"
		return m, nil
	}
	return m, m.loadProject(ref)
}

// setupTarget names the project the screen would open. Pressing the key on the
// screen itself is a refresh of its own subject rather than a jump to the
// board's, which may be another project entirely.
func (m Model) setupTarget() string {
	if m.view == viewProject && m.setup != nil {
		return m.setup.project.Key
	}
	if m.view == viewProjects && m.projectSel < len(m.projects) {
		return m.projects[m.projectSel].Key
	}
	return m.project.Key
}

// setupRef is the project the open setup screen acts on.
func (m Model) setupRef() string {
	if m.setup == nil {
		return ""
	}
	return m.setup.project.Key
}

// handleProjectKey scrolls the project screen and runs the actions that
// configure the container tasks live in.
func (m Model) handleProjectKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		return m.leave(nil)
	case key.Matches(msg, m.keys.Up):
		m.setupOff = max(0, m.setupOff-1)
	case key.Matches(msg, m.keys.Down):
		m.setupOff++
	case key.Matches(msg, m.keys.Top):
		m.setupOff = 0
	case key.Matches(msg, m.keys.Bottom):
		lines, rows := m.setupBody()
		m.setupOff = max(0, len(lines)-rows)
	default:
		return m.handleSetupKey(msg)
	}
	return m, nil
}

// setupBody is the project screen's lines and how many of them fit.
func (m Model) setupBody() ([]ProjectLine, int) {
	lines := ProjectView(m.setupState())
	height := LayoutFor(m.width, m.height, len(m.columns)).BodyHeight
	return lines, VisibleRows(height, len(lines))
}

// setupState is what the project screen renders from.
func (m Model) setupState() ProjectState {
	if m.setup == nil {
		return ProjectState{TimeStyle: m.timeStyle}
	}
	return ProjectState{
		Project: m.setup.project, Workflow: m.setup.workflow, Fields: m.setup.fields,
		WorkflowErr: m.setup.workflowErr, TimeStyle: m.timeStyle,
	}
}

// handleSetupKey runs the project screen's actions, refusing a key this reader's
// authority does not reach rather than letting the service refuse it.
func (m Model) handleSetupKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if !m.maySetupPress(msg) {
		return m, nil
	}
	switch {
	case key.Matches(msg, m.keys.Edit):
		return m.openProjectForm()
	case key.Matches(msg, m.keys.Delete):
		return m.openProjectRemoveForm()
	case key.Matches(msg, m.keys.New):
		return m.openPrompt(promptNewField)
	case key.Matches(msg, m.keys.Fields):
		return m.openFieldForm()
	}
	return m, nil
}

// maySetupPress reports whether this reader holds the authority the pressed key
// acts through, asked of the same list the footer and the help overlay filter
// with so the three cannot disagree.
func (m Model) maySetupPress(msg tea.KeyPressMsg) bool {
	for _, a := range m.keys.projectBindings() {
		if key.Matches(msg, a.binding) {
			return a.permitted(m.permits())
		}
	}
	return true
}

// openProjectForm offers the attributes an edit can change.
func (m Model) openProjectForm() (Model, tea.Cmd) {
	if m.setup == nil {
		m.err = "no project is open"
		return m, nil
	}
	m.form, m.err = ProjectEditForm(m.setup.project, m.setup.workflows), ""
	return m, nil
}

// openProjectRemoveForm asks whether the project should be hidden or destroyed,
// offering only what this reader may do and what the project's own state allows.
func (m Model) openProjectRemoveForm() (Model, tea.Cmd) {
	if m.setup == nil {
		m.err = "no project is open"
		return m, nil
	}
	form := ProjectRemoveForm(m.setup.project,
		m.mayPerform("ArchiveProject"), m.mayPerform("DeleteProject"))
	if !form.Open() {
		m.err = "this project is already archived and cannot be deleted from here"
		return m, nil
	}
	m.form, m.err = form, ""
	return m, nil
}

// openFieldForm offers the custom fields the project defines, which is what a
// reader who does not know their keys needs before changing one.
func (m Model) openFieldForm() (Model, tea.Cmd) {
	if m.setup == nil {
		m.err = "no project is open"
		return m, nil
	}
	form := FieldPickForm(m.setup.fields,
		m.mayPerform("PutFieldDef"), m.mayPerform("DeleteFieldDef"))
	if !form.Open() {
		m.err = "this project defines no custom fields; " + m.keys.New.Help().Key + " defines one"
		return m, nil
	}
	m.form, m.err = form, ""
	return m, nil
}

// openFieldDefForm names a new custom field and asks what its values look like.
// A key the project already defines is redefined, seeded from what it holds, so
// the prompt is a way to reach a field as well as a way to add one.
func (m Model) openFieldDefForm(text string) (Model, tea.Cmd) {
	fieldKey, label := SplitFieldEntry(text)
	if fieldKey == "" {
		return m, nil
	}
	if m.setup == nil {
		m.err = "no project is open"
		return m, nil
	}
	base, _ := FieldDefFor(m.setup.fields, fieldKey)
	if label == "" {
		label = base.Label
	}
	m.pending = pendingField{key: fieldKey, label: label, base: base}
	m.form, m.err = FieldDefForm(fieldKey, base), ""
	return m, nil
}

// applyProjectEdit performs what the edit form asked for. The two attributes
// drawn from fixed sets are answered by the form; the three that are free text
// hand over to the single-line prompt, seeded with the value they would replace.
func (m Model) applyProjectEdit(form Form) (Model, tea.Cmd) {
	if m.setup == nil {
		return m, nil
	}
	p := m.setup.project
	switch form.Value("attribute") {
	case attrColour:
		colour := ColourValue(form.Value(attrColour))
		return m, m.updateProject(core.UpdateProjectInput{Color: &colour}, attrColour)
	case attrWorkflow:
		flow := form.Value(attrWorkflow)
		return m, m.updateProject(core.UpdateProjectInput{WorkflowKey: &flow}, attrWorkflow)
	case attrName:
		return m.startPrompt(promptProjectName, p.Name), textinput.Blink
	case attrDescription:
		return m.startPrompt(promptProjectDesc, p.Description), textinput.Blink
	case attrIcon:
		return m.startPrompt(promptProjectIcon, p.Icon), textinput.Blink
	}
	return m, nil
}

// confirmProjectRemoval states what the removal form asked for and waits for
// agreement, because both branches take a project off the board.
func (m Model) confirmProjectRemoval(form Form) (Model, tea.Cmd) {
	if m.setup == nil {
		return m, nil
	}
	p := m.setup.project
	confirm := Confirm{Target: p.Key, label: p.Key, projectRef: p.Key, Kind: confirmArchiveProject}
	if form.Value("action") == "delete" {
		confirm.Kind, confirm.Note = confirmDeleteProject, ProjectDeleteNote
	}
	return m.askConfirm(confirm, "there is no project named to remove")
}

// applyFieldPick sends a picked field to the change it was picked for: a removal
// to the confirmation, and a redefinition to a second form seeded from what the
// definition already holds.
func (m Model) applyFieldPick(form Form) (Model, tea.Cmd) {
	if m.setup == nil {
		return m, nil
	}
	fieldKey := form.Value("field")
	base, ok := FieldDefFor(m.setup.fields, fieldKey)
	if !ok {
		m.err = "this project no longer defines " + fieldKey
		return m, nil
	}
	if form.Value("action") == "remove" {
		return m.askConfirm(Confirm{Kind: confirmDeleteField, Target: fieldKey,
			projectRef: m.setup.project.Key, fieldKey: fieldKey},
			"there is no custom field named to delete")
	}
	m.pending = pendingField{key: fieldKey, label: base.Label, base: base}
	m.form, m.err = FieldDefForm(fieldKey, base), ""
	return m, nil
}

// applyFieldDef writes the definition the form gathered, over everything the
// existing one held, because a put replaces the whole definition.
func (m Model) applyFieldDef(form Form) (Model, tea.Cmd) {
	if m.setup == nil || m.pending.key == "" {
		return m, nil
	}
	in := FieldDefUpdate(m.pending.key, m.pending.label, m.pending.base,
		core.FieldType(form.Value("type")), form.Yes("required"))
	return m, m.putFieldDef(in)
}

// askConfirm opens a confirmation, refusing one that cannot name its subject
// because the naming is the whole of its value.
func (m Model) askConfirm(confirm Confirm, unnamed string) (Model, tea.Cmd) {
	if confirm.Question() == "" {
		m.err = unnamed
		return m, nil
	}
	m.confirm, m.err = confirm, ""
	return m, nil
}
