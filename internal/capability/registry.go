// SPDX-License-Identifier: AGPL-3.0-or-later

package capability

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/web"
	"github.com/heliopsy/tix/internal/wire"
)

// apiGet and its siblings name a route on the REST surface.
func apiGet(pattern string) Route    { return Route{Method: http.MethodGet, Pattern: pattern} }
func apiPost(pattern string) Route   { return Route{Method: http.MethodPost, Pattern: pattern} }
func apiPut(pattern string) Route    { return Route{Method: http.MethodPut, Pattern: pattern} }
func apiPatch(pattern string) Route  { return Route{Method: http.MethodPatch, Pattern: pattern} }
func apiDelete(pattern string) Route { return Route{Method: http.MethodDelete, Pattern: pattern} }

// apiDeleteWhen names a DELETE route that one query parameter picks out of
// several an operation shares. The archive branch of DELETE on a project is
// reachable only this way, and the branch without the flag destroys the
// project, so an operation that omits the discriminator is not merely
// imprecise: it addresses the deletion.
func apiDeleteWhen(pattern, query string) Route {
	return Route{Method: http.MethodDelete, Pattern: pattern, Query: query}
}

// webGet names a browser screen and the template it renders.
func webGet(pattern, tmpl string) Route {
	return Route{Method: http.MethodGet, Pattern: pattern, Template: tmpl}
}

// webPost names a browser route performing a mutation.
func webPost(pattern string) Route { return Route{Method: http.MethodPost, Pattern: pattern} }

// off records a surface an operation is genuinely inapplicable to.
func off(s Surface, reason string) Exemption { return Exemption{Surface: s, Reason: reason} }

// gap records a binding that ought to exist and does not.
func gap(s Surface, reason string) Exemption {
	return Exemption{Surface: s, Reason: reason, Gap: true}
}

// Templates the browser screens render, named once so a binding cannot invent
// a file the embedded assets do not hold.
const (
	tplTenant    = "tenant.html"
	tplDomains   = "domains.html"
	tplUsers     = "users.html"
	tplTokens    = "tokens.html"
	tplSSHKeys   = "sshkeys.html"
	tplProjects  = "projects.html"
	tplBoard     = "board.html"
	tplFields    = "fields.html"
	tplWorkflows = "workflows.html"
	tplWorkflow  = "workflow.html"
	tplTasks     = "tasks.html"
	tplTask      = "task.html"
	tplActors    = "actors.html"
	tplActivity  = "activity.html"
	tplStats     = "stats.html"
	tplWebhooks  = "webhooks.html"
	tplSync      = "sync.html"
	tplConns     = "connections.html"
)

// registry declares every operation the product offers. Every exported method
// of core.Service appears exactly once; the parity tests fail the build when
// one does not.
var registry = []Operation{
	{
		Name: "identity.whoami", Method: "WhoAmI",
		CLI:  "tix doctor",
		HTTP: apiGet(wire.RouteWhoAmI),
		Exempt: []Exemption{
			gap(SurfaceWeb, "GAP: no browser binding; the root route calls it for its error and discards the identity, then redirects, so no screen states who the reader is"),
		},
		// The tenant view asks the tenant it is switching to who the actor is
		// there, which is the only way to tell a tenant that does not exist
		// from one this actor cannot see.
		TUI: "tenant",
	},
	{
		Name: "service.close", Method: "Close",
		Exempt: []Exemption{
			off(SurfaceCLI, "closing the service is process lifecycle, not an operation an operator invokes"),
			off(SurfaceHTTP, "closing the service is process lifecycle; the server owns its own shutdown"),
			off(SurfaceWeb, "closing the service is process lifecycle, not a screen"),
			off(SurfaceTUI, "closing the service is process lifecycle, not a view"),
		},
	},

	{
		Name: "tenant.create", Method: "CreateTenant",
		Scope: core.ScopeTenantAdmin,
		CLI:   "tix tenant create",
		HTTP:  apiPost(wire.RouteTenants),
		Exempt: []Exemption{
			off(SurfaceWeb, "a browser session is pinned to one tenant, so creating another is an installation-level operation reserved for the CLI"),
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no tenant administration view"),
		},
	},
	{
		Name: "tenant.show", Method: "GetTenant",
		Scope: core.ScopeTenantAdmin,
		CLI:   "tix tenant show",
		HTTP:  apiGet(wire.RouteTenant),
		Web:   webGet(web.RouteTenant, tplTenant),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no tenant administration view"),
		},
	},
	{
		Name: "tenant.list", Method: "ListTenants",
		Scope: core.ScopeTenantAdmin,
		CLI:   "tix tenant ls",
		HTTP:  apiGet(wire.RouteTenants),
		Web:   webGet(web.RouteTenant, tplTenant),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no tenant administration view"),
		},
	},
	{
		Name: "tenant.update", Method: "UpdateTenant",
		Scope: core.ScopeTenantAdmin,
		CLI:   "tix tenant edit",
		HTTP:  apiPatch(wire.RouteTenant),
		Web:   webPost(web.RouteTenant),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no tenant administration view"),
		},
	},
	{
		Name: "tenant.delete", Method: "DeleteTenant",
		Scope: core.ScopeTenantAdmin,
		CLI:   "tix tenant rm",
		HTTP:  apiDelete(wire.RouteTenant),
		Exempt: []Exemption{
			off(SurfaceWeb, "deleting the tenant the session belongs to would destroy the operator's own access, so it is reserved for the CLI"),
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no tenant administration view"),
		},
	},
	{
		Name: "domain.add", Method: "AddDomain",
		Scope: core.ScopeTenantAdmin,
		CLI:   "tix domain add",
		HTTP:  apiPost(wire.RouteDomains),
		Web:   webPost(web.RouteDomains),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no domain administration view"),
		},
	},
	{
		Name: "domain.list", Method: "ListDomains",
		Scope: core.ScopeTenantAdmin,
		CLI:   "tix domain ls",
		HTTP:  apiGet(wire.RouteDomains),
		Web:   webGet(web.RouteDomains, tplDomains),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no domain administration view"),
		},
	},
	{
		Name: "domain.remove", Method: "RemoveDomain",
		Scope: core.ScopeTenantAdmin,
		CLI:   "tix domain rm",
		HTTP:  apiDelete(wire.RouteDomain),
		Web:   webPost(web.RouteDomainRemove),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no domain administration view"),
		},
	},
	{
		Name: "domain.resolve", Method: "ResolveDomain",
		HTTP: apiGet(wire.RouteDomain),
		Exempt: []Exemption{
			off(SurfaceCLI, "hostname resolution runs in the server request path; an operator inspects the mappings with tix domain ls"),
			off(SurfaceWeb, "hostname resolution runs before any screen is chosen and is not an operator action"),
			off(SurfaceTUI, "hostname resolution runs in the server request path and is not an operator action"),
		},
	},
	{
		Name: "member.add", Method: "AddMember",
		Scope: core.ScopeTenantAdmin,
		CLI:   "tix member add",
		HTTP:  apiPost(wire.RouteMembers),
		Web:   webPost(web.RouteMembers),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no membership administration view"),
		},
	},
	{
		Name: "member.list", Method: "ListMembers",
		Scope: core.ScopeTenantAdmin,
		CLI:   "tix member ls",
		HTTP:  apiGet(wire.RouteMembers),
		Web:   webGet(web.RouteTenant, tplTenant),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no membership administration view"),
		},
	},
	{
		Name: "member.remove", Method: "RemoveMember",
		Scope: core.ScopeTenantAdmin,
		CLI:   "tix member rm",
		HTTP:  apiDelete(wire.RouteMember),
		Web:   webPost(web.RouteMemberRemove),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no membership administration view"),
		},
	},

	{
		Name: "project.create", Method: "CreateProject",
		Scope: core.ScopeProjectWrite,
		CLI:   "tix project create",
		HTTP:  apiPost(wire.RouteProjects),
		Web:   webPost(web.RouteProjects),
		TUI:   "projects",
	},
	{
		Name: "project.show", Method: "GetProject",
		Scope: core.ScopeProjectRead,
		CLI:   "tix project show",
		HTTP:  apiGet(wire.RouteProject),
		Web:   webGet(web.RouteProject, tplBoard),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; the project list opens a board from the listing it already holds and never fetches one project"),
		},
	},
	{
		Name: "project.list", Method: "ListProjects",
		Scope: core.ScopeProjectRead,
		CLI:   "tix project ls",
		HTTP:  apiGet(wire.RouteProjects),
		Web:   webGet(web.RouteProjects, tplProjects),
		TUI:   "projects",
	},
	{
		Name: "project.update", Method: "UpdateProject",
		Scope: core.ScopeProjectWrite,
		CLI:   "tix project edit",
		HTTP:  apiPatch(wire.RouteProject),
		Web:   webPost(web.RouteProjectEdit),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; the project list creates and opens projects but cannot edit one"),
		},
	},
	{
		Name: "project.archive", Method: "ArchiveProject",
		Scope: core.ScopeProjectWrite,
		CLI:   "tix project archive",
		HTTP:  apiDeleteWhen(wire.RouteProject, "archive=true"),
		Web:   webPost(web.RouteProjectArch),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; the project list creates and opens projects but cannot archive one"),
		},
	},
	{
		Name: "project.delete", Method: "DeleteProject",
		Scope: core.ScopeProjectWrite,
		CLI:   "tix project rm",
		HTTP:  apiDelete(wire.RouteProject),
		Web:   webPost(web.RouteProjectDel),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; the project list creates and opens projects but cannot delete one"),
		},
	},
	{
		Name: "field.put", Method: "PutFieldDef",
		Scope: core.ScopeProjectWrite,
		CLI:   "tix field put",
		HTTP:  apiPut(wire.RouteProjectFields),
		Web:   webPost(web.RouteFields),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; the detail view shows custom field values but there is no field definition editor"),
		},
	},
	{
		Name: "field.list", Method: "ListFieldDefs",
		Scope: core.ScopeProjectRead,
		CLI:   "tix field ls",
		HTTP:  apiGet(wire.RouteProjectFields),
		Web:   webGet(web.RouteFields, tplFields),
		TUI:   "detail",
	},
	{
		Name: "field.delete", Method: "DeleteFieldDef",
		Scope: core.ScopeProjectWrite,
		CLI:   "tix field rm",
		HTTP:  apiDelete(wire.RouteProjectField),
		Web:   webPost(web.RouteFieldDelete),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; the detail view shows custom field values but there is no field definition editor"),
		},
	},

	{
		Name: "workflow.put", Method: "PutWorkflow",
		Scope: core.ScopeWorkflowWrite,
		CLI:   "tix workflow put",
		HTTP:  apiPut(wire.RouteWorkflows),
		Web:   webPost(web.RouteWorkflows),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; the board renders a workflow but there is no workflow editor"),
		},
	},
	{
		Name: "workflow.get", Method: "GetWorkflow",
		Scope: core.ScopeWorkflowRead,
		CLI:   "tix workflow get",
		HTTP:  apiGet(wire.RouteWorkflow),
		Web:   webGet(web.RouteWorkflow, tplWorkflow),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; the board resolves its workflow from the listing and never fetches one by key"),
		},
	},
	{
		Name: "workflow.list", Method: "ListWorkflows",
		Scope: core.ScopeWorkflowRead,
		CLI:   "tix workflow ls",
		HTTP:  apiGet(wire.RouteWorkflows),
		Web:   webGet(web.RouteWorkflows, tplWorkflows),
		TUI:   "board",
	},
	{
		Name: "workflow.delete", Method: "DeleteWorkflow",
		Scope: core.ScopeWorkflowWrite,
		CLI:   "tix workflow rm",
		HTTP:  apiDelete(wire.RouteWorkflow),
		Web:   webPost(web.RouteWorkflowDel),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; the board renders a workflow but there is no workflow editor"),
		},
	},

	{
		Name: "task.create", Method: "CreateTask",
		Scope: core.ScopeTaskWrite,
		CLI:   "tix task add",
		HTTP:  apiPost(wire.RouteTasks),
		Web:   webPost(web.RouteTasks),
		TUI:   "board",
	},
	{
		Name: "task.show", Method: "GetTask",
		Scope: core.ScopeTaskRead,
		CLI:   "tix task show",
		HTTP:  apiGet(wire.RouteTask),
		Web:   webGet(web.RouteTask, tplTask),
		TUI:   "detail",
	},
	{
		Name: "task.list", Method: "ListTasks",
		Scope: core.ScopeTaskRead,
		CLI:   "tix task ls",
		HTTP:  apiGet(wire.RouteTasks),
		Web:   webGet(web.RouteTasks, tplTasks),
		TUI:   "board",
	},
	{
		Name: "task.update", Method: "UpdateTask",
		Scope: core.ScopeTaskWrite,
		CLI:   "tix task edit",
		HTTP:  apiPatch(wire.RouteTask),
		Web:   webPost(web.RouteTask),
		TUI:   "board",
	},
	{
		Name: "task.transition", Method: "TransitionTask",
		Scope: core.ScopeTaskTransition,
		CLI:   "tix task mv",
		HTTP:  apiPost(wire.RouteTaskTransition),
		Web:   webPost(web.RouteTaskMove),
		TUI:   "board",
	},
	{
		Name: "task.delete", Method: "DeleteTask",
		Scope: core.ScopeTaskDelete,
		CLI:   "tix task rm",
		HTTP:  apiDelete(wire.RouteTask),
		Web:   webPost(web.RouteTaskDelete),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; deleting a task needs a confirmation step the interface does not have yet"),
		},
	},
	{
		Name: "task.restore", Method: "RestoreTask",
		Scope: core.ScopeTaskDelete,
		CLI:   "tix task restore",
		HTTP:  apiPost(wire.RouteTaskRestore),
		Web:   webPost(web.RouteTaskRestore),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no view of deleted tasks to restore one from"),
		},
	},
	{
		Name: "task.tree", Method: "TaskTree",
		Scope: core.ScopeTaskRead,
		CLI:   "tix task tree",
		HTTP:  apiGet(wire.RouteTaskTree),
		Web:   webGet(web.RouteTask, tplTask),
		TUI:   "detail",
		Limits: []Limitation{{SurfaceWeb,
			"the task screen asks for depth 1, so it shows direct subtasks only where the cli and the api walk the tree to any depth"}},
	},
	{
		Name: "dependency.add", Method: "AddDependency",
		Scope: core.ScopeTaskWrite,
		CLI:   "tix dep add",
		HTTP:  apiPost(wire.RouteTaskDeps),
		Web:   webPost(web.RouteTaskDeps),
		TUI:   "detail",
	},
	{
		Name: "dependency.remove", Method: "RemoveDependency",
		Scope: core.ScopeTaskWrite,
		CLI:   "tix dep rm",
		HTTP:  apiDelete(wire.RouteTaskDep),
		Web:   webPost(web.RouteTaskDepDel),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; the detail view lists dependencies and can add one but cannot remove one"),
		},
	},
	{
		Name: "dependency.list", Method: "ListDependencies",
		Scope: core.ScopeTaskRead,
		CLI:   "tix dep ls",
		HTTP:  apiGet(wire.RouteTaskDeps),
		Web:   webGet(web.RouteTask, tplTask),
		TUI:   "detail",
	},
	{
		Name: "tag.add", Method: "AddTag",
		Scope: core.ScopeTaskWrite,
		CLI:   "tix tag add",
		HTTP:  apiPost(wire.RouteTaskLabels),
		Web:   webPost(web.RouteTaskTags),
		TUI:   "detail",
	},
	{
		Name: "tag.remove", Method: "RemoveTag",
		Scope: core.ScopeTaskWrite,
		CLI:   "tix tag rm",
		HTTP:  apiDelete(wire.RouteTaskLabel),
		Web:   webPost(web.RouteTaskTagDel),
		TUI:   "detail",
	},
	{
		Name: "tag.list", Method: "ListTags",
		Scope: core.ScopeTaskRead,
		CLI:   "tix tag ls",
		HTTP:  apiGet(wire.RouteLabels),
		Web:   webGet(web.RouteTasks, tplTasks),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; tags are added and removed by name; the interface offers no tag listing to choose from"),
		},
	},
	{
		Name: "comment.add", Method: "AddComment",
		Scope: core.ScopeCommentWrite,
		CLI:   "tix comment add",
		HTTP:  apiPost(wire.RouteTaskComments),
		Web:   webPost(web.RouteTaskComments),
		TUI:   "detail",
	},
	{
		Name: "comment.list", Method: "ListComments",
		Scope: core.ScopeTaskRead,
		CLI:   "tix comment ls",
		HTTP:  apiGet(wire.RouteTaskComments),
		Web:   webGet(web.RouteTask, tplTask),
		TUI:   "detail",
	},
	{
		Name: "comment.edit", Method: "EditComment",
		Scope: core.ScopeCommentWrite,
		CLI:   "tix comment edit",
		HTTP:  apiPatch(wire.RouteComment),
		Web:   webPost(web.RouteCommentEdit),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; comments are shown as a thread with no way to select one to edit"),
		},
	},
	{
		Name: "comment.delete", Method: "DeleteComment",
		Scope: core.ScopeCommentWrite,
		CLI:   "tix comment rm",
		HTTP:  apiDelete(wire.RouteComment),
		Web:   webPost(web.RouteCommentDel),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; comments are shown as a thread with no way to select one to delete"),
		},
	},
	{
		Name: "artifact.put", Method: "PutArtifact",
		Scope: core.ScopeArtifactWrite,
		CLI:   "tix artifact put",
		HTTP:  apiPut(wire.RouteTaskArtifacts),
		Web:   webPost(web.RouteTaskArts),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; the detail view lists artifacts but recording one needs structured input the single-line prompt cannot gather"),
		},
	},
	{
		Name: "artifact.list", Method: "ListArtifacts",
		Scope: core.ScopeTaskRead,
		CLI:   "tix artifact ls",
		HTTP:  apiGet(wire.RouteTaskArtifacts),
		Web:   webGet(web.RouteTask, tplTask),
		TUI:   "detail",
	},

	{
		Name: "claim.task", Method: "ClaimTask",
		Scope: core.ScopeTaskClaim,
		CLI:   "tix claim task",
		HTTP:  apiPost(wire.RouteTaskClaim),
		TUI:   "board",
		Exempt: []Exemption{
			off(SurfaceWeb, "claiming is the agent work queue; the lease token it returns belongs to the worker and is never displayed in a browser"),
		},
	},
	{
		Name: "claim.next", Method: "ClaimNext",
		Scope: core.ScopeTaskClaim,
		CLI:   "tix claim next",
		HTTP:  apiPost(wire.RouteClaimNext),
		TUI:   "board",
		Exempt: []Exemption{
			off(SurfaceWeb, "claiming is the agent work queue; a browser operator picks a task by name instead"),
		},
	},
	{
		Name: "claim.renew", Method: "RenewLease",
		Scope: core.ScopeTaskClaim,
		CLI:   "tix claim renew",
		HTTP:  apiPost(wire.RouteTaskClaimRenew),
		TUI:   "board",
		Exempt: []Exemption{
			off(SurfaceWeb, "a lease is renewed by the worker holding its token, which a browser session does not hold"),
		},
	},
	{
		Name: "claim.release", Method: "ReleaseLease",
		Scope: core.ScopeTaskClaim,
		CLI:   "tix claim release",
		HTTP:  apiPost(wire.RouteTaskClaimRelease),
		TUI:   "board",
		Exempt: []Exemption{
			off(SurfaceWeb, "a lease is released by the worker holding its token, which a browser session does not hold"),
		},
	},
	{
		Name: "claim.sweep", Method: "SweepLeases",
		Scope: core.ScopeTaskWrite,
		CLI:   "tix claim sweep",
		HTTP:  apiPost(wire.RouteClaimSweep),
		Exempt: []Exemption{
			off(SurfaceWeb, "lease sweeping is a background maintenance loop, not an operator action"),
			off(SurfaceTUI, "lease sweeping is a background maintenance loop, not an operator action"),
		},
	},

	{
		Name: "stats.read", Method: "Stats",
		Scope: core.ScopeTaskRead,
		CLI:   "tix stats",
		HTTP:  apiGet(wire.RouteStats),
		Web:   webGet(web.RouteStats, tplStats),
		TUI:   "stats",
	},

	{
		Name: "audit.list", Method: "ListAudit",
		Scope: core.ScopeAuditRead,
		CLI:   "tix audit ls",
		HTTP:  apiGet(wire.RouteAudit),
		Web:   webGet(web.RouteActivity, tplActivity),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no history view on a task or a project"),
		},
	},
	{
		Name: "event.subscribe", Method: "Subscribe",
		Scope: core.ScopeEventSubscribe,
		CLI:   "tix watch",
		HTTP:  apiGet(wire.RouteEvents),
		// The subscription runs under every view, but the activity screen is
		// the one built to draw it, so that is where the binding belongs.
		TUI: "activity",
		Exempt: []Exemption{
			off(SurfaceWeb, "the browser consumes the event stream over the websocket route rather than calling the method from a screen"),
		},
	},
	{
		Name: "retention.prune", Method: "Prune",
		Scope: core.ScopeTenantAdmin,
		CLI:   "tix prune",
		HTTP:  apiPost(wire.RoutePrune),
		Web:   webPost(web.RoutePrune),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no retention view"),
		},
	},
	{
		Name: "retention.get", Method: "GetRetention",
		Scope: core.ScopeTenantAdmin,
		CLI:   "tix retention show",
		HTTP:  apiGet(wire.RouteRetention),
		Web:   webGet(web.RouteTenant, tplTenant),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no retention view"),
		},
	},
	{
		Name: "retention.put", Method: "PutRetention",
		Scope: core.ScopeTenantAdmin,
		CLI:   "tix retention set",
		HTTP:  apiPut(wire.RouteRetention),
		Web:   webPost(web.RouteRetention),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no retention view"),
		},
	},

	{
		Name: "actor.list", Method: "ListActors",
		CLI:  "tix actor ls",
		HTTP: apiGet(wire.RouteActors),
		Web:  webGet(web.RouteActors, tplActors),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; the interface has no directory view"),
		},
	},
	{
		Name: "actor.show", Method: "GetActor",
		CLI:  "tix actor show",
		HTTP: apiGet(wire.RouteActor),
		// The detail view resolves every identifier a task and its comments
		// name to a handle, which is this lookup and nothing else.
		TUI: "detail",
		Exempt: []Exemption{
			gap(SurfaceWeb, "GAP: no browser binding; the screens call it only to label an identifier, keeping the handle and discarding the rest, and there is no actor screen to open"),
		},
	},
	{
		Name: "user.create", Method: "CreateUser",
		Scope: core.ScopeUserAdmin,
		CLI:   "tix user create",
		HTTP:  apiPost(wire.RouteUsers),
		Web:   webPost(web.RouteUsers),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no user administration view"),
		},
	},
	{
		Name: "user.show", Method: "GetUser",
		Scope: core.ScopeUserAdmin,
		CLI:   "tix user show",
		HTTP:  apiGet(wire.RouteUser),
		Exempt: []Exemption{
			off(SurfaceWeb, "the user administration list already shows every field a single user screen would show"),
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no user administration view"),
		},
	},
	{
		Name: "user.list", Method: "ListUsers",
		Scope: core.ScopeUserAdmin,
		CLI:   "tix user ls",
		HTTP:  apiGet(wire.RouteUsers),
		Web:   webGet(web.RouteUsers, tplUsers),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no user administration view"),
		},
	},
	{
		Name: "user.update", Method: "UpdateUser",
		Scope: core.ScopeUserAdmin,
		CLI:   "tix user edit",
		HTTP:  apiPatch(wire.RouteUser),
		Web:   webPost(web.RouteUserUpdate),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no user administration view"),
		},
	},
	{
		Name: "user.delete", Method: "DeleteUser",
		Scope: core.ScopeUserAdmin,
		CLI:   "tix user rm",
		HTTP:  apiDelete(wire.RouteUser),
		Web:   webPost(web.RouteUserDelete),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no user administration view"),
		},
	},
	{
		Name: "session.login", Method: "Login",
		CLI:  "tix login",
		HTTP: apiPost(wire.RouteLogin),
		Web:  webPost(web.RouteLogin),
		Exempt: []Exemption{
			off(SurfaceTUI, "the interface opens on a session that already exists; tix login establishes it before the program starts"),
		},
	},
	{
		Name: "session.logout", Method: "Logout",
		CLI:  "tix logout",
		HTTP: apiPost(wire.RouteLogout),
		Web:  webPost(web.RouteLogout),
		Exempt: []Exemption{
			off(SurfaceTUI, "ending the session would revoke the credential the running interface is using; tix logout is the way out"),
		},
	},
	{
		Name: "token.create", Method: "CreateToken",
		Scope: core.ScopeTokenAdmin,
		CLI:   "tix token create",
		HTTP:  apiPost(wire.RouteTokens),
		Web:   webPost(web.RouteTokens),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no token administration view"),
		},
	},
	{
		Name: "token.list", Method: "ListTokens",
		Scope: core.ScopeTokenAdmin,
		CLI:   "tix token ls",
		HTTP:  apiGet(wire.RouteTokens),
		Web:   webGet(web.RouteTokens, tplTokens),
		Limits: []Limitation{{SurfaceWeb,
			"the screen lists the signed-in actor's own tokens only, where the cli and the api take another actor's identifier"}},
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no token administration view"),
		},
	},
	{
		Name: "token.revoke", Method: "RevokeToken",
		Scope: core.ScopeTokenAdmin,
		CLI:   "tix token rm",
		HTTP:  apiDelete(wire.RouteToken),
		Web:   webPost(web.RouteTokenRevoke),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no token administration view"),
		},
	},

	{
		Name: "sshkey.enrol", Method: "EnrolSSHKey",
		Scope: core.ScopeTokenAdmin,
		CLI:   "tix user key add",
		HTTP:  apiPost(wire.RouteSSHKeys),
		Web:   webPost(web.RouteSSHKeys),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no credential administration view"),
		},
	},
	{
		Name: "sshkey.list", Method: "ListSSHKeys",
		Scope: core.ScopeTokenAdmin,
		CLI:   "tix user key ls",
		HTTP:  apiGet(wire.RouteSSHKeys),
		Web:   webGet(web.RouteSSHKeys, tplSSHKeys),
		Limits: []Limitation{{SurfaceWeb,
			"the screen lists the signed-in actor's own keys only, where the cli and the api take another actor's identifier"}},
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no credential administration view"),
		},
	},
	{
		Name: "sshkey.revoke", Method: "RevokeSSHKey",
		Scope: core.ScopeTokenAdmin,
		CLI:   "tix user key rm",
		HTTP:  apiDelete(wire.RouteSSHKey),
		Web:   webPost(web.RouteSSHKeyRevoke),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no credential administration view"),
		},
	},

	{
		Name: "webhook.put", Method: "PutWebhook",
		Scope: core.ScopeWebhookAdmin,
		CLI:   "tix webhook put",
		HTTP:  apiPut(wire.RouteWebhooks),
		Web:   webPost(web.RouteWebhooks),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no webhook administration view"),
		},
	},
	{
		Name: "webhook.list", Method: "ListWebhooks",
		Scope: core.ScopeWebhookAdmin,
		CLI:   "tix webhook ls",
		HTTP:  apiGet(wire.RouteWebhooks),
		Web:   webGet(web.RouteWebhooks, tplWebhooks),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no webhook administration view"),
		},
	},
	{
		Name: "webhook.delete", Method: "DeleteWebhook",
		Scope: core.ScopeWebhookAdmin,
		CLI:   "tix webhook rm",
		HTTP:  apiDelete(wire.RouteWebhook),
		Web:   webPost(web.RouteWebhookDelete),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no webhook administration view"),
		},
	},
	{
		Name: "webhook.deliveries", Method: "ListDeliveries",
		Scope: core.ScopeWebhookAdmin,
		CLI:   "tix webhook deliveries",
		HTTP:  apiGet(wire.RouteDeliveries),
		Web:   webGet(web.RouteWebhooks, tplWebhooks),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no webhook delivery view"),
		},
	},
	{
		Name: "webhook.redeliver", Method: "RedeliverWebhook",
		Scope: core.ScopeWebhookAdmin,
		CLI:   "tix webhook redeliver",
		HTTP:  apiPost(wire.RouteDeliveryRedeliver),
		Web:   webPost(web.RouteRedeliver),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no webhook delivery view"),
		},
	},

	{
		Name: "snapshot.export", Method: "ExportTo",
		Scope: core.ScopeExport,
		CLI:   "tix export",
		HTTP:  apiPost(wire.RouteExport),
		Web:   webPost(web.RouteExport),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no import or export view"),
		},
	},
	{
		Name: "snapshot.import", Method: "ImportFrom",
		Scope: core.ScopeImport,
		CLI:   "tix import",
		HTTP:  apiPost(wire.RouteImport),
		Web:   webPost(web.RouteImport),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no import or export view"),
		},
	},
	{
		Name: "bundle.export", Method: "ExportBundle",
		Scope: core.ScopeWorkflowRead,
		CLI:   "tix bundle export",
		HTTP:  apiPost(wire.RouteBundleExport),
		Web:   webPost(web.RouteBundleExport),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no import or export view"),
		},
	},
	{
		// The scope this needs is decided by the component kinds the bundle
		// carries, so no single one describes it. authority_test.go names it
		// rather than letting an empty scope read as "needs nothing".
		Name: "bundle.import", Method: "ImportBundle",
		CLI:  "tix bundle import",
		HTTP: apiPost(wire.RouteBundleImport),
		Web:  webPost(web.RouteBundleImport),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no import or export view"),
		},
	},

	{
		Name: "sync.source.put", Method: "PutSyncSource",
		Scope: core.ScopeSyncAdmin,
		CLI:   "tix sync source add",
		HTTP:  apiPut(wire.RouteSyncSources),
		Web:   webPost(web.RouteSyncSources),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no external sync view"),
		},
	},
	{
		Name: "sync.source.list", Method: "ListSyncSources",
		Scope: core.ScopeSyncAdmin,
		CLI:   "tix sync source ls",
		HTTP:  apiGet(wire.RouteSyncSources),
		Web:   webGet(web.RouteSync, tplSync),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no external sync view"),
		},
	},
	{
		Name: "sync.source.delete", Method: "DeleteSyncSource",
		Scope: core.ScopeSyncAdmin,
		CLI:   "tix sync source rm",
		HTTP:  apiDelete(wire.RouteSyncSource),
		Web:   webPost(web.RouteSyncDelete),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no external sync view"),
		},
	},
	{
		Name: "sync.run", Method: "RunSync",
		Scope: core.ScopeSyncAdmin,
		CLI:   "tix sync run",
		HTTP:  apiPost(wire.RouteSyncRun),
		Web:   webPost(web.RouteSyncRun),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no external sync view"),
		},
	},

	{
		Name: "connection.list", Method: "ListConnections",
		Scope: core.ScopeTenantAdmin,
		CLI:   "tix connection ls",
		HTTP:  apiGet(wire.RouteConnections),
		Web:   webGet(web.RouteConnections, tplConns),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no server administration view"),
		},
	},
	{
		Name: "connection.end", Method: "EndConnection",
		Scope: core.ScopeTenantAdmin,
		CLI:   "tix connection kill",
		HTTP:  apiDelete(wire.RouteConnection),
		Web:   webPost(web.RouteConnectionEnd),
		Exempt: []Exemption{
			gap(SurfaceTUI, "GAP: no tui binding yet; there is no server administration view"),
		},
	},
}
