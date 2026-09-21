package capability

import (
	"net/http"

	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/web"
)

// apiGet and its siblings name a route on the REST surface.
func apiGet(pattern string) Route    { return Route{Method: http.MethodGet, Pattern: pattern} }
func apiPost(pattern string) Route   { return Route{Method: http.MethodPost, Pattern: pattern} }
func apiPut(pattern string) Route    { return Route{Method: http.MethodPut, Pattern: pattern} }
func apiPatch(pattern string) Route  { return Route{Method: http.MethodPatch, Pattern: pattern} }
func apiDelete(pattern string) Route { return Route{Method: http.MethodDelete, Pattern: pattern} }

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
	tplProjects  = "projects.html"
	tplBoard     = "board.html"
	tplFields    = "fields.html"
	tplWorkflows = "workflows.html"
	tplWorkflow  = "workflow.html"
	tplTasks     = "tasks.html"
	tplTask      = "task.html"
	tplActivity  = "activity.html"
	tplWebhooks  = "webhooks.html"
	tplSync      = "sync.html"
)

// registry declares every operation the product offers. Every exported method
// of core.Service appears exactly once; the parity tests fail the build when
// one does not.
var registry = []Operation{
	{
		Name: "identity.whoami", Method: "WhoAmI",
		CLI:  "tix doctor",
		HTTP: apiGet(httpapi.RouteWhoAmI),
		Web:  webGet(web.RouteRoot, ""),
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
		CLI:  "tix tenant create",
		HTTP: apiPost(httpapi.RouteTenants),
		Exempt: []Exemption{
			off(SurfaceWeb, "a browser session is pinned to one tenant, so creating another is an installation-level operation reserved for the CLI"),
		},
	},
	{
		Name: "tenant.show", Method: "GetTenant",
		CLI:  "tix tenant show",
		HTTP: apiGet(httpapi.RouteTenant),
		Web:  webGet(web.RouteTenant, tplTenant),
	},
	{
		Name: "tenant.list", Method: "ListTenants",
		CLI:  "tix tenant ls",
		HTTP: apiGet(httpapi.RouteTenants),
		Web:  webGet(web.RouteTenant, tplTenant),
	},
	{
		Name: "tenant.update", Method: "UpdateTenant",
		CLI:  "tix tenant edit",
		HTTP: apiPatch(httpapi.RouteTenant),
		Web:  webPost(web.RouteTenant),
	},
	{
		Name: "tenant.delete", Method: "DeleteTenant",
		CLI:  "tix tenant rm",
		HTTP: apiDelete(httpapi.RouteTenant),
		Exempt: []Exemption{
			off(SurfaceWeb, "deleting the tenant the session belongs to would destroy the operator's own access, so it is reserved for the CLI"),
		},
	},
	{
		Name: "domain.add", Method: "AddDomain",
		CLI:  "tix domain add",
		HTTP: apiPost(httpapi.RouteDomains),
		Web:  webPost(web.RouteDomains),
	},
	{
		Name: "domain.list", Method: "ListDomains",
		CLI:  "tix domain ls",
		HTTP: apiGet(httpapi.RouteDomains),
		Web:  webGet(web.RouteDomains, tplDomains),
	},
	{
		Name: "domain.remove", Method: "RemoveDomain",
		CLI:  "tix domain rm",
		HTTP: apiDelete(httpapi.RouteDomain),
		Web:  webPost(web.RouteDomainRemove),
	},
	{
		Name: "domain.resolve", Method: "ResolveDomain",
		HTTP: apiGet(httpapi.RouteDomain),
		Exempt: []Exemption{
			off(SurfaceCLI, "hostname resolution runs in the server request path; an operator inspects the mappings with tix domain ls"),
			off(SurfaceWeb, "hostname resolution runs before any screen is chosen and is not an operator action"),
		},
	},
	{
		Name: "member.add", Method: "AddMember",
		CLI:  "tix member add",
		HTTP: apiPost(httpapi.RouteMembers),
		Web:  webPost(web.RouteMembers),
	},
	{
		Name: "member.list", Method: "ListMembers",
		CLI:  "tix member ls",
		HTTP: apiGet(httpapi.RouteMembers),
		Web:  webGet(web.RouteTenant, tplTenant),
	},
	{
		Name: "member.remove", Method: "RemoveMember",
		CLI:  "tix member rm",
		HTTP: apiDelete(httpapi.RouteMember),
		Web:  webPost(web.RouteMemberRemove),
	},

	{
		Name: "project.create", Method: "CreateProject",
		CLI:  "tix project create",
		HTTP: apiPost(httpapi.RouteProjects),
		Web:  webPost(web.RouteProjects),
	},
	{
		Name: "project.show", Method: "GetProject",
		CLI:  "tix project show",
		HTTP: apiGet(httpapi.RouteProject),
		Web:  webGet(web.RouteProject, tplBoard),
	},
	{
		Name: "project.list", Method: "ListProjects",
		CLI:  "tix project ls",
		HTTP: apiGet(httpapi.RouteProjects),
		Web:  webGet(web.RouteProjects, tplProjects),
		TUI:  "projects",
	},
	{
		Name: "project.update", Method: "UpdateProject",
		CLI:  "tix project edit",
		HTTP: apiPatch(httpapi.RouteProject),
		Web:  webPost(web.RouteProjectEdit),
	},
	{
		Name: "project.archive", Method: "ArchiveProject",
		CLI:  "tix project archive",
		HTTP: apiDelete(httpapi.RouteProject),
		Web:  webPost(web.RouteProjectArch),
	},
	{
		Name: "project.delete", Method: "DeleteProject",
		CLI:  "tix project rm",
		HTTP: apiDelete(httpapi.RouteProject),
		Web:  webPost(web.RouteProjectDel),
	},
	{
		Name: "field.put", Method: "PutFieldDef",
		CLI:  "tix field put",
		HTTP: apiPut(httpapi.RouteProjectFields),
		Web:  webPost(web.RouteFields),
	},
	{
		Name: "field.list", Method: "ListFieldDefs",
		CLI:  "tix field ls",
		HTTP: apiGet(httpapi.RouteProjectFields),
		Web:  webGet(web.RouteFields, tplFields),
		TUI:  "detail",
	},
	{
		Name: "field.delete", Method: "DeleteFieldDef",
		CLI:  "tix field rm",
		HTTP: apiDelete(httpapi.RouteProjectField),
		Web:  webPost(web.RouteFieldDelete),
	},

	{
		Name: "workflow.put", Method: "PutWorkflow",
		CLI:  "tix workflow put",
		HTTP: apiPut(httpapi.RouteWorkflows),
		Web:  webPost(web.RouteWorkflows),
	},
	{
		Name: "workflow.get", Method: "GetWorkflow",
		CLI:  "tix workflow get",
		HTTP: apiGet(httpapi.RouteWorkflow),
		Web:  webGet(web.RouteWorkflow, tplWorkflow),
	},
	{
		Name: "workflow.list", Method: "ListWorkflows",
		CLI:  "tix workflow ls",
		HTTP: apiGet(httpapi.RouteWorkflows),
		Web:  webGet(web.RouteWorkflows, tplWorkflows),
		TUI:  "board",
	},
	{
		Name: "workflow.delete", Method: "DeleteWorkflow",
		CLI:  "tix workflow rm",
		HTTP: apiDelete(httpapi.RouteWorkflow),
		Web:  webPost(web.RouteWorkflowDel),
	},

	{
		Name: "task.create", Method: "CreateTask",
		CLI:  "tix task add",
		HTTP: apiPost(httpapi.RouteTasks),
		Web:  webPost(web.RouteTasks),
	},
	{
		Name: "task.show", Method: "GetTask",
		CLI:  "tix task show",
		HTTP: apiGet(httpapi.RouteTask),
		Web:  webGet(web.RouteTask, tplTask),
		TUI:  "detail",
	},
	{
		Name: "task.list", Method: "ListTasks",
		CLI:  "tix task ls",
		HTTP: apiGet(httpapi.RouteTasks),
		Web:  webGet(web.RouteTasks, tplTasks),
		TUI:  "board",
	},
	{
		Name: "task.update", Method: "UpdateTask",
		CLI:  "tix task edit",
		HTTP: apiPatch(httpapi.RouteTask),
		Web:  webPost(web.RouteTask),
	},
	{
		Name: "task.transition", Method: "TransitionTask",
		CLI:  "tix task mv",
		HTTP: apiPost(httpapi.RouteTaskTransition),
		Web:  webPost(web.RouteTaskMove),
		TUI:  "board",
	},
	{
		Name: "task.delete", Method: "DeleteTask",
		CLI:  "tix task rm",
		HTTP: apiDelete(httpapi.RouteTask),
		Web:  webPost(web.RouteTaskDelete),
	},
	{
		Name: "task.restore", Method: "RestoreTask",
		CLI:  "tix task restore",
		HTTP: apiPost(httpapi.RouteTaskRestore),
		Web:  webPost(web.RouteTaskRestore),
	},
	{
		Name: "task.tree", Method: "TaskTree",
		CLI:  "tix task tree",
		HTTP: apiGet(httpapi.RouteTaskTree),
		Web:  webGet(web.RouteTask, tplTask),
		TUI:  "detail",
	},
	{
		Name: "dependency.add", Method: "AddDependency",
		CLI:  "tix dep add",
		HTTP: apiPost(httpapi.RouteTaskDeps),
		Web:  webPost(web.RouteTaskDeps),
	},
	{
		Name: "dependency.remove", Method: "RemoveDependency",
		CLI:  "tix dep rm",
		HTTP: apiDelete(httpapi.RouteTaskDep),
		Web:  webPost(web.RouteTaskDepDel),
	},
	{
		Name: "dependency.list", Method: "ListDependencies",
		CLI:  "tix dep ls",
		HTTP: apiGet(httpapi.RouteTaskDeps),
		Web:  webGet(web.RouteTask, tplTask),
		TUI:  "detail",
	},
	{
		Name: "tag.add", Method: "AddTag",
		CLI:  "tix tag add",
		HTTP: apiPost(httpapi.RouteTaskLabels),
		Web:  webPost(web.RouteTaskTags),
	},
	{
		Name: "tag.remove", Method: "RemoveTag",
		CLI:  "tix tag rm",
		HTTP: apiDelete(httpapi.RouteTaskLabel),
		Web:  webPost(web.RouteTaskTagDel),
	},
	{
		Name: "tag.list", Method: "ListTags",
		CLI:  "tix tag ls",
		HTTP: apiGet(httpapi.RouteLabels),
		Web:  webGet(web.RouteTasks, tplTasks),
	},
	{
		Name: "comment.add", Method: "AddComment",
		CLI:  "tix comment add",
		HTTP: apiPost(httpapi.RouteTaskComments),
		Web:  webPost(web.RouteTaskComments),
	},
	{
		Name: "comment.list", Method: "ListComments",
		CLI:  "tix comment ls",
		HTTP: apiGet(httpapi.RouteTaskComments),
		Web:  webGet(web.RouteTask, tplTask),
		TUI:  "detail",
	},
	{
		Name: "comment.edit", Method: "EditComment",
		CLI:  "tix comment edit",
		HTTP: apiPatch(httpapi.RouteComment),
		Web:  webPost(web.RouteCommentEdit),
	},
	{
		Name: "comment.delete", Method: "DeleteComment",
		CLI:  "tix comment rm",
		HTTP: apiDelete(httpapi.RouteComment),
		Web:  webPost(web.RouteCommentDel),
	},
	{
		Name: "artifact.put", Method: "PutArtifact",
		HTTP: apiPut(httpapi.RouteTaskArtifacts),
		Web:  webPost(web.RouteTaskArts),
		Exempt: []Exemption{
			gap(SurfaceCLI, "GAP: no cli binding yet; artifacts can be attached over http and from the browser but tix has no artifact command"),
		},
	},
	{
		Name: "artifact.list", Method: "ListArtifacts",
		HTTP: apiGet(httpapi.RouteTaskArtifacts),
		Web:  webGet(web.RouteTask, tplTask),
		Exempt: []Exemption{
			gap(SurfaceCLI, "GAP: no cli binding yet; artifacts are listed over http and on the task screen but tix has no artifact command"),
		},
	},

	{
		Name: "claim.task", Method: "ClaimTask",
		CLI:  "tix claim task",
		HTTP: apiPost(httpapi.RouteTaskClaim),
		TUI:  "board",
		Exempt: []Exemption{
			off(SurfaceWeb, "claiming is the agent work queue; the lease token it returns belongs to the worker and is never displayed in a browser"),
		},
	},
	{
		Name: "claim.next", Method: "ClaimNext",
		CLI:  "tix claim next",
		HTTP: apiPost(httpapi.RouteClaimNext),
		Exempt: []Exemption{
			off(SurfaceWeb, "claiming is the agent work queue; a browser operator picks a task by name instead"),
		},
	},
	{
		Name: "claim.renew", Method: "RenewLease",
		CLI:  "tix claim renew",
		HTTP: apiPost(httpapi.RouteTaskClaimRenew),
		Exempt: []Exemption{
			off(SurfaceWeb, "a lease is renewed by the worker holding its token, which a browser session does not hold"),
		},
	},
	{
		Name: "claim.release", Method: "ReleaseLease",
		CLI:  "tix claim release",
		HTTP: apiPost(httpapi.RouteTaskClaimRelease),
		TUI:  "board",
		Exempt: []Exemption{
			off(SurfaceWeb, "a lease is released by the worker holding its token, which a browser session does not hold"),
		},
	},
	{
		Name: "claim.sweep", Method: "SweepLeases",
		CLI:  "tix claim sweep",
		HTTP: apiPost(httpapi.RouteClaimSweep),
		Exempt: []Exemption{
			off(SurfaceWeb, "lease sweeping is a background maintenance loop, not an operator action"),
		},
	},

	{
		Name: "audit.list", Method: "ListAudit",
		CLI:  "tix audit ls",
		HTTP: apiGet(httpapi.RouteAudit),
		Web:  webGet(web.RouteActivity, tplActivity),
	},
	{
		Name: "event.subscribe", Method: "Subscribe",
		HTTP: apiGet(httpapi.RouteEvents),
		TUI:  "board",
		Exempt: []Exemption{
			gap(SurfaceCLI, "GAP: no cli binding yet; the event stream is reachable over the websocket route and from the tui but tix has no watch command"),
			off(SurfaceWeb, "the browser consumes the event stream over the websocket route rather than calling the method from a screen"),
		},
	},
	{
		Name: "retention.prune", Method: "Prune",
		CLI:  "tix prune",
		HTTP: apiPost(httpapi.RoutePrune),
		Web:  webPost(web.RoutePrune),
	},
	{
		Name: "retention.get", Method: "GetRetention",
		HTTP: apiGet(httpapi.RouteRetention),
		Web:  webGet(web.RouteTenant, tplTenant),
		Exempt: []Exemption{
			gap(SurfaceCLI, "GAP: no cli binding yet; the policy is readable over http and on the tenant screen but tix prune cannot show it"),
		},
	},
	{
		Name: "retention.put", Method: "PutRetention",
		HTTP: apiPut(httpapi.RouteRetention),
		Web:  webPost(web.RouteRetention),
		Exempt: []Exemption{
			gap(SurfaceCLI, "GAP: no cli binding yet; the policy is writable over http and on the tenant screen but tix has no retention command"),
		},
	},

	{
		Name: "actor.show", Method: "GetActor",
		CLI:  "tix actor show",
		HTTP: apiGet(httpapi.RouteActor),
		Web:  webGet(web.RouteActivity, tplActivity),
	},
	{
		Name: "user.create", Method: "CreateUser",
		CLI:  "tix user create",
		HTTP: apiPost(httpapi.RouteUsers),
		Web:  webPost(web.RouteUsers),
	},
	{
		Name: "user.show", Method: "GetUser",
		CLI:  "tix user show",
		HTTP: apiGet(httpapi.RouteUser),
		Exempt: []Exemption{
			off(SurfaceWeb, "the user administration list already shows every field a single user screen would show"),
		},
	},
	{
		Name: "user.list", Method: "ListUsers",
		CLI:  "tix user ls",
		HTTP: apiGet(httpapi.RouteUsers),
		Web:  webGet(web.RouteUsers, tplUsers),
	},
	{
		Name: "user.update", Method: "UpdateUser",
		CLI:  "tix user edit",
		HTTP: apiPatch(httpapi.RouteUser),
		Web:  webPost(web.RouteUserUpdate),
	},
	{
		Name: "user.delete", Method: "DeleteUser",
		CLI:  "tix user rm",
		HTTP: apiDelete(httpapi.RouteUser),
		Web:  webPost(web.RouteUserDelete),
	},
	{
		Name: "session.login", Method: "Login",
		CLI:  "tix login",
		HTTP: apiPost(httpapi.RouteLogin),
		Web:  webPost(web.RouteLogin),
	},
	{
		Name: "session.logout", Method: "Logout",
		HTTP: apiPost(httpapi.RouteLogout),
		Web:  webPost(web.RouteLogout),
		Exempt: []Exemption{
			gap(SurfaceCLI, "GAP: no cli binding yet; tix login stores a session the CLI offers no way to end"),
		},
	},
	{
		Name: "token.create", Method: "CreateToken",
		CLI:  "tix token create",
		HTTP: apiPost(httpapi.RouteTokens),
		Web:  webPost(web.RouteTokens),
	},
	{
		Name: "token.list", Method: "ListTokens",
		CLI:  "tix token ls",
		HTTP: apiGet(httpapi.RouteTokens),
		Web:  webGet(web.RouteTokens, tplTokens),
	},
	{
		Name: "token.revoke", Method: "RevokeToken",
		CLI:  "tix token rm",
		HTTP: apiDelete(httpapi.RouteToken),
		Web:  webPost(web.RouteTokenRevoke),
	},

	{
		Name: "webhook.put", Method: "PutWebhook",
		CLI:  "tix webhook put",
		HTTP: apiPut(httpapi.RouteWebhooks),
		Web:  webPost(web.RouteWebhooks),
	},
	{
		Name: "webhook.list", Method: "ListWebhooks",
		CLI:  "tix webhook ls",
		HTTP: apiGet(httpapi.RouteWebhooks),
		Web:  webGet(web.RouteWebhooks, tplWebhooks),
	},
	{
		Name: "webhook.delete", Method: "DeleteWebhook",
		CLI:  "tix webhook rm",
		HTTP: apiDelete(httpapi.RouteWebhook),
		Web:  webPost(web.RouteWebhookDelete),
	},
	{
		Name: "webhook.deliveries", Method: "ListDeliveries",
		CLI:  "tix webhook deliveries",
		HTTP: apiGet(httpapi.RouteDeliveries),
		Web:  webGet(web.RouteWebhooks, tplWebhooks),
	},
	{
		Name: "webhook.redeliver", Method: "RedeliverWebhook",
		CLI:  "tix webhook redeliver",
		HTTP: apiPost(httpapi.RouteDeliveryRedeliver),
		Web:  webPost(web.RouteRedeliver),
	},

	{
		Name: "snapshot.export", Method: "ExportTo",
		CLI:  "tix export",
		HTTP: apiPost(httpapi.RouteExport),
		Web:  webPost(web.RouteExport),
	},
	{
		Name: "snapshot.import", Method: "ImportFrom",
		CLI:  "tix import",
		HTTP: apiPost(httpapi.RouteImport),
		Web:  webPost(web.RouteImport),
	},
	{
		Name: "bundle.export", Method: "ExportBundle",
		CLI:  "tix bundle export",
		HTTP: apiPost(httpapi.RouteBundleExport),
		Web:  webPost(web.RouteBundleExport),
	},
	{
		Name: "bundle.import", Method: "ImportBundle",
		CLI:  "tix bundle import",
		HTTP: apiPost(httpapi.RouteBundleImport),
		Web:  webPost(web.RouteBundleImport),
	},

	{
		Name: "sync.source.put", Method: "PutSyncSource",
		CLI:  "tix sync source add",
		HTTP: apiPut(httpapi.RouteSyncSources),
		Web:  webPost(web.RouteSyncSources),
	},
	{
		Name: "sync.source.list", Method: "ListSyncSources",
		CLI:  "tix sync source ls",
		HTTP: apiGet(httpapi.RouteSyncSources),
		Web:  webGet(web.RouteSync, tplSync),
	},
	{
		Name: "sync.source.delete", Method: "DeleteSyncSource",
		CLI:  "tix sync source rm",
		HTTP: apiDelete(httpapi.RouteSyncSource),
		Web:  webPost(web.RouteSyncDelete),
	},
	{
		Name: "sync.run", Method: "RunSync",
		CLI:  "tix sync run",
		HTTP: apiPost(httpapi.RouteSyncRun),
		Web:  webPost(web.RouteSyncRun),
	},
}
