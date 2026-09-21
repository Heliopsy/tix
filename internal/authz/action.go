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
	ActionTokenAdmin   Action = "token.admin"
	ActionWebhookAdmin Action = "webhook.admin"
	ActionTenantAdmin  Action = "tenant.admin"
	ActionSyncAdmin    Action = "sync.admin"

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
	ActionTokenAdmin:   core.ScopeTokenAdmin,
	ActionWebhookAdmin: core.ScopeWebhookAdmin,
	ActionTenantAdmin:  core.ScopeTenantAdmin,
	ActionSyncAdmin:    core.ScopeSyncAdmin,

	ActionExport:         core.ScopeExport,
	ActionImport:         core.ScopeImport,
	ActionRetentionWrite: core.ScopeTenantAdmin,
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
	ActionTaskClaim, ActionTaskDelete,
	ActionProjectRead, ActionProjectWrite,
	ActionWorkflowRead, ActionWorkflowWrite,
	ActionFieldWrite, ActionCommentWrite, ActionArtifactWrite,
	ActionEventSubscribe, ActionAuditRead,
	ActionUserAdmin, ActionTokenAdmin, ActionWebhookAdmin, ActionTenantAdmin, ActionSyncAdmin,
	ActionExport, ActionImport, ActionRetentionWrite,
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
