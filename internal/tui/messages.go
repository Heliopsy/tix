// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// viewKind names one of the interface's screens.
type viewKind int

// Views.
const (
	viewProjects viewKind = iota
	viewBoard
	viewDetail
	viewHelp
	viewSettings
	viewActivity
	viewTenant
	viewStats
	viewProject
)

// actionKind names a board action whose result is reported back.
type actionKind int

// Board actions.
const (
	actionClaim actionKind = iota
	actionRelease
	actionTransition
	actionCreate
	actionUpdate
	actionComment
	actionTag
	actionUntag
	actionDepend
	actionUndepend
	actionEditComment
	actionDeleteComment
	actionDelete
	actionRenew
	actionNewProject
	actionEditProject
	actionArchiveProject
	actionDeleteProject
	actionPutField
	actionDeleteField
)

// actionLabels name each action in the present tense, for a refusal, and in
// the past tense, for the status bar.
var actionLabels = map[actionKind][2]string{
	actionClaim:         {"claim", "claimed"},
	actionRelease:       {"release", "released"},
	actionTransition:    {"transition", "transitioned"},
	actionCreate:        {"create the task", "created"},
	actionUpdate:        {"edit the task", "edited"},
	actionComment:       {"comment", "commented on"},
	actionTag:           {"tag", "tagged"},
	actionUntag:         {"untag", "untagged"},
	actionDepend:        {"add the dependency", "added a dependency to"},
	actionUndepend:      {"remove the dependency", "removed a dependency from"},
	actionEditComment:   {"edit the comment", "edited a comment on"},
	actionDeleteComment: {"delete the comment", "deleted a comment on"},
	actionDelete:        {"delete the task", "deleted"},
	actionRenew:         {"renew the lease", "renewed the lease on"},
	actionNewProject:    {"create the project", "created"},

	// The project screen's own actions. Their subject is a project or a field
	// definition rather than a task, so each carries its own sentence and the
	// generic name is only a fallback.
	actionEditProject:    {"edit the project", "edited"},
	actionArchiveProject: {"archive the project", "archived"},
	actionDeleteProject:  {"delete the project", "deleted"},
	actionPutField:       {"define the field", "defined"},
	actionDeleteField:    {"delete the field", "deleted"},
}

// Label renders an action for a message.
func (a actionKind) Label() string { return actionLabels[a][0] }

// Past renders an action for the status bar once it has succeeded.
func (a actionKind) Past() string { return actionLabels[a][1] }

// Mutates reports whether an action changes the task it names, so the detail
// view knows to fetch it again.
func (a actionKind) Mutates() bool {
	switch a {
	case actionClaim, actionRelease, actionRenew:
		return false
	default:
		return true
	}
}

// ChangesSetup reports whether an action changes what the project screen shows,
// so the screen is read again rather than left stating what it used to be.
func (a actionKind) ChangesSetup() bool {
	switch a {
	case actionEditProject, actionArchiveProject, actionPutField, actionDeleteField:
		return true
	default:
		return false
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
	artifacts []core.Artifact
	// actors maps an actor identifier to its handle, for every actor the
	// task or its comments name, so the detail view never has to show a raw
	// identifier for a lookup it already made.
	actors map[string]string
}

// projectMsg carries one project, the workflow it runs on, the custom fields
// its tasks carry and the workflows an edit could move it to. The workflow is
// separate from its error because a reader refused workflows still gets the rest
// of the screen.
type projectMsg struct {
	project   core.Project
	workflow  *core.Workflow
	fields    []core.FieldDef
	workflows []string
	// workflowErr is why the workflow is missing, which the screen says rather
	// than drawing a project that runs on nothing.
	workflowErr string
}

// eventMsg carries one event from the subscription. gen names the connection
// it was read from, so a switch to another tenant cannot be followed by an
// event the previous tenant's stream was already holding.
type eventMsg struct {
	event core.Event
	gen   int
}

// streamMsg reports the state of the event subscription.
type streamMsg struct {
	connected bool
	err       error
	events    <-chan core.Event
	gen       int
}

// reconnectMsg asks for a dropped subscription to be opened again.
type reconnectMsg struct{ gen int }

// tenantMsg reports the outcome of switching the session to another tenant.
// A failure carries the key that was asked for, so the refusal can name it.
type tenantMsg struct {
	key   string
	conn  TenantConn
	actor *core.Actor
	err   error
}

// actionMsg reports the outcome of a board action.
type actionMsg struct {
	kind  actionKind
	ref   core.TaskRef
	label string
	token string
	err   error

	// sentence is the status bar text a command already knows enough to write
	// for itself: an update names the fields it sent, a transition names the
	// states it moved between. It wins over kind.Past()+name() whenever it is
	// set, which is what lets "edited homelab-3" become "edited the title of
	// homelab-3" without every action needing its own sentence.
	sentence string
}

// name is what an action calls its subject on screen. A task's identifier is
// what the service is addressed by; its ref is what a person recognises.
func (a actionMsg) name() string {
	if a.label != "" {
		return a.label
	}
	return a.ref.String()
}

// statusText is what the status bar shows once this action has succeeded:
// the action's own sentence when it built one, and the generic "past tense
// verb plus subject" otherwise.
func (a actionMsg) statusText() string {
	if a.sentence != "" {
		return a.sentence
	}
	return strings.TrimSpace(a.kind.Past() + " " + a.name())
}

// errMsg carries a failure to the interface.
type errMsg struct {
	err   error
	fatal bool
}

// statsMsg carries one statistics read back to the model.
type statsMsg struct {
	stats *core.Stats
	err   error
}

// tagsMsg carries the tenant's tags, which the tag form offers to pick from.
type tagsMsg struct {
	tags []core.Tag
	err  error
}
