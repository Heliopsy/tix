// SPDX-License-Identifier: AGPL-3.0-or-later

// Package authz is the single place where tix decides whether an actor may act.
package authz

import "github.com/heliopsy/tix/internal/core"

// Action names one authorization decision point in the product surface.
type Action string

// Actions.
const (
	ActionTaskRead       Action = "task.read"
	ActionTaskCreate     Action = "task.create"
	ActionTaskUpdate     Action = "task.update"
	ActionTaskTransition Action = "task.transition"
	ActionTaskClaim      Action = "task.claim"
	ActionTaskDelete     Action = "task.delete"
	ActionTaskReclaim    Action = "task.reclaim"
	// ActionTaskStar sets or clears the caller's own star. It changes nothing
	// anybody else sees in a listing, so reading the task is authority enough.
	ActionTaskStar Action = "task.star"
	// ActionViewArrange reads and changes the caller's own arrangement of a
	// view's groups. Like a star it changes nothing anybody else sees, so it
	// needs only task:read; a view spans projects, so it is not confinable.
	ActionViewArrange Action = "view.arrange"
	// ActionFilterSave reads, saves and changes the caller's own saved
	// filters, and reads the ones shared with the tenant. A filter selects
	// nothing by itself, so reading tasks is authority enough; a filter spans
	// projects, so it is not confinable.
	ActionFilterSave Action = "filter.save"
	// ActionFilterManage changes or removes a shared filter somebody else
	// owns, which is administering what the whole tenant sees.
	ActionFilterManage Action = "filter.manage"
	// ActionCalendarFeed manages the caller's own calendar feed. The feed
	// reads no more than its owner may, so reading tasks is authority enough;
	// it lists every project the owner reads, so it is not confinable.
	ActionCalendarFeed Action = "calendar.feed"

	ActionProjectRead  Action = "project.read"
	ActionProjectWrite Action = "project.write"

	ActionWorkflowRead  Action = "workflow.read"
	ActionWorkflowWrite Action = "workflow.write"

	ActionFieldWrite    Action = "field.write"
	ActionCommentWrite  Action = "comment.write"
	ActionArtifactWrite Action = "artifact.write"

	ActionEventSubscribe Action = "event.subscribe"
	ActionAuditRead      Action = "audit.read"

	ActionUserAdmin    Action = "user.admin"
	ActionTOTPAdmin    Action = "totp.admin"
	ActionTokenAdmin   Action = "token.admin"
	ActionWebhookAdmin Action = "webhook.admin"
	ActionTenantAdmin  Action = "tenant.admin"
	ActionSyncAdmin    Action = "sync.admin"

	// ActionServerRead reads the installation: which servers are registered,
	// what the store is, and how big the tenant's work is. It is not confinable
	// to a project, because a server serves every project there is.
	ActionServerRead Action = "server.read"

	ActionExport         Action = "export"
	ActionImport         Action = "import"
	ActionRetentionWrite Action = "retention.write"
)

// actionScopes maps every action to the one scope it requires.
var actionScopes = map[Action]core.Scope{
	ActionTaskRead:       core.ScopeTaskRead,
	ActionTaskCreate:     core.ScopeTaskWrite,
	ActionTaskUpdate:     core.ScopeTaskWrite,
	ActionTaskTransition: core.ScopeTaskTransition,
	ActionTaskClaim:      core.ScopeTaskClaim,
	ActionTaskDelete:     core.ScopeTaskDelete,
	ActionTaskReclaim:    core.ScopeTaskReclaim,
	ActionTaskStar:       core.ScopeTaskRead,
	ActionViewArrange:    core.ScopeTaskRead,
	ActionFilterSave:     core.ScopeTaskRead,
	ActionFilterManage:   core.ScopeTenantAdmin,
	ActionCalendarFeed:   core.ScopeTaskRead,

	ActionProjectRead:  core.ScopeProjectRead,
	ActionProjectWrite: core.ScopeProjectWrite,

	ActionWorkflowRead:  core.ScopeWorkflowRead,
	ActionWorkflowWrite: core.ScopeWorkflowWrite,

	ActionFieldWrite:    core.ScopeProjectWrite,
	ActionCommentWrite:  core.ScopeCommentWrite,
	ActionArtifactWrite: core.ScopeArtifactWrite,

	ActionEventSubscribe: core.ScopeEventSubscribe,
	ActionAuditRead:      core.ScopeAuditRead,

	ActionUserAdmin:    core.ScopeUserAdmin,
	ActionTOTPAdmin:    core.ScopeTOTPAdmin,
	ActionTokenAdmin:   core.ScopeTokenAdmin,
	ActionWebhookAdmin: core.ScopeWebhookAdmin,
	ActionTenantAdmin:  core.ScopeTenantAdmin,
	ActionSyncAdmin:    core.ScopeSyncAdmin,

	ActionExport:         core.ScopeExport,
	ActionImport:         core.ScopeImport,
	ActionRetentionWrite: core.ScopeTenantAdmin,
	// Reading the installation is administration of the installation, which is
	// what tenant:admin already means here. A scope of its own would be a
	// second name for the same authority and one more thing to grant.
	ActionServerRead: core.ScopeTenantAdmin,
}

// projectConfinable enumerates the actions a project-pinned API token may
// still perform, because everything they reach belongs to one project and the
// policy can hold the token inside its own.
//
// It is an allow list on purpose. Confinement used to depend on each call site
// remembering to name a project, which made it opt-in and left a pinned token
// able to export the tenant, read the tenant's audit log and mint itself an
// unpinned token. Anything absent here reaches tenant-wide state, so a pinned
// token is refused it outright rather than trusted to have been passed a
// project identifier.
//
// Workflow definitions are tenant-wide configuration a pinned agent must read
// to drive its own project's tasks, so reading them is confinable while
// rewriting them, which changes every project, is not.
var projectConfinable = map[Action]bool{
	ActionTaskRead:       true,
	ActionTaskCreate:     true,
	ActionTaskUpdate:     true,
	ActionTaskTransition: true,
	ActionTaskClaim:      true,
	ActionTaskDelete:     true,
	// A forced reclaim reaches one task, so a token pinned to a project can
	// be trusted with it the way it is trusted with claiming: the policy
	// holds it inside its own project on the way through.
	ActionTaskReclaim: true,
	ActionTaskStar:    true,

	ActionProjectRead:  true,
	ActionProjectWrite: true,

	ActionWorkflowRead: true,

	ActionFieldWrite:    true,
	ActionCommentWrite:  true,
	ActionArtifactWrite: true,
}

// ProjectConfinable reports whether a project-pinned token may perform the
// action at all. An unknown action is not confinable.
func (a Action) ProjectConfinable() bool { return projectConfinable[a] }

// allActions is the action vocabulary in a stable order.
var allActions = []Action{
	ActionTaskRead, ActionTaskCreate, ActionTaskUpdate, ActionTaskTransition,
	ActionTaskClaim, ActionTaskDelete, ActionTaskReclaim, ActionTaskStar, ActionViewArrange,
	ActionFilterSave, ActionFilterManage, ActionCalendarFeed,
	ActionProjectRead, ActionProjectWrite,
	ActionWorkflowRead, ActionWorkflowWrite,
	ActionFieldWrite, ActionCommentWrite, ActionArtifactWrite,
	ActionEventSubscribe, ActionAuditRead,
	ActionUserAdmin, ActionTOTPAdmin, ActionTokenAdmin, ActionWebhookAdmin, ActionTenantAdmin, ActionSyncAdmin,
	ActionExport, ActionImport, ActionRetentionWrite,
	ActionServerRead,
}

// Actions returns every known action in a stable order.
func Actions() []Action { return append([]Action(nil), allActions...) }

// Scope returns the scope the action requires, and whether the action is known.
func (a Action) Scope() (core.Scope, bool) {
	s, ok := actionScopes[a]
	return s, ok
}

// String returns the action name.
func (a Action) String() string { return string(a) }
