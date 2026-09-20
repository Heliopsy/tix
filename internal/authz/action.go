// Package authz is the single place where tix decides whether an actor may act.
package authz

import "github.com/thereisnotime/tix/internal/core"

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
