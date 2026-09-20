package tui

import "github.com/thereisnotime/tix/internal/core"

// viewKind names one of the interface's screens.
type viewKind int

// Views.
const (
	viewProjects viewKind = iota
	viewBoard
	viewDetail
	viewHelp
)

// actionKind names a board action whose result is reported back.
type actionKind int

// Board actions.
const (
	actionClaim actionKind = iota
	actionRelease
	actionTransition
)

// Label renders an action for a message.
func (a actionKind) Label() string {
	switch a {
	case actionClaim:
		return "claim"
	case actionRelease:
		return "release"
	default:
		return "transition"
	}
}

// projectsMsg carries the project listing.
type projectsMsg struct {
	projects []core.Project
}

// boardMsg carries a project, its workflow and its tasks.
type boardMsg struct {
	project  core.Project
	workflow *core.WorkflowDefinition
	tasks    []core.Task
}

// tasksMsg carries a refreshed task listing for the open project.
type tasksMsg struct {
	tasks []core.Task
}

// detailMsg carries everything the detail view shows.
type detailMsg struct {
	task      core.Task
	subtasks  []core.Task
	deps      []core.Dependency
	comments  []core.Comment
	fieldDefs []core.FieldDef
}

// eventMsg carries one event from the subscription.
type eventMsg struct {
	event core.Event
}

// streamMsg reports the state of the event subscription.
type streamMsg struct {
	connected bool
	err       error
	events    <-chan core.Event
}

// reconnectMsg asks for a dropped subscription to be opened again.
type reconnectMsg struct{}

// actionMsg reports the outcome of a board action.
type actionMsg struct {
	kind  actionKind
	ref   core.TaskRef
	token string
	err   error
}

// errMsg carries a failure to the interface.
type errMsg struct {
	err   error
	fatal bool
}
