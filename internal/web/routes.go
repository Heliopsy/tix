package web

import "net/http"

// Binding records one browser route, the service operation it exposes and the
// template it renders. It is the web half of the operation registry.
type Binding struct {
	Method   string
	Pattern  string
	Services []string
	Template string
}

// Exemption records an operation deliberately absent from the browser
// interface together with the reason it is absent.
type Exemption struct {
	Service string
	Reason  string
}

// route is a binding plus the function serving it.
type route struct {
	Binding
	Public bool
	fn     func(http.ResponseWriter, *http.Request) error
}

// Bindings returns every route the browser interface serves.
func Bindings() []Binding {
	rts := (&handler{}).routes()
	out := make([]Binding, 0, len(rts))
	for _, rt := range rts {
		out = append(out, rt.Binding)
	}
	return out
}

// Exemptions returns the operations the browser interface does not expose,
// each with the justification for its absence.
func Exemptions() []Exemption {
	return []Exemption{
		{"CreateTenant", "a browser session is pinned to one tenant, so creating another tenant is an installation-level operation reserved for the CLI"},
		{"DeleteTenant", "deleting the tenant the session belongs to would destroy the operator's own access, so it is reserved for the CLI"},
		{"ResolveDomain", "hostname resolution runs in the request path before any screen is chosen and is not an operator action"},
		{"GetUser", "the user administration list already shows every field a single user screen would show"},
		{"ClaimTask", "claiming is the agent work queue; the lease token it returns belongs to the worker and is never displayed in a browser"},
		{"ClaimNext", "claiming is the agent work queue; a browser operator picks a task by name instead"},
		{"RenewLease", "a lease is renewed by the worker holding its token, which a browser session does not hold"},
		{"ReleaseLease", "a lease is released by the worker holding its token, which a browser session does not hold"},
		{"SweepLeases", "lease sweeping is a background maintenance loop, not an operator action"},
		{"Subscribe", "the browser consumes the event stream over the WebSocket route rather than calling the method directly"},
		{"Close", "closing the service is process lifecycle, not an operation"},
	}
}

// routes returns every route with the function that serves it.
func (h *handler) routes() []route {
	var out []route
	out = append(out, h.sessionRoutes()...)
	out = append(out, h.projectRoutes()...)
	out = append(out, h.workflowRoutes()...)
	out = append(out, h.taskRoutes()...)
	out = append(out, h.adminRoutes()...)
	out = append(out, h.webhookRoutes()...)
	out = append(out, h.transferRoutes()...)
	out = append(out, h.bundleRoutes()...)
	return out
}

// get builds a route rendering a screen, naming every operation it exposes.
func get(pattern, tmpl string, fn func(http.ResponseWriter, *http.Request) error, services ...string) route {
	return route{Binding: Binding{Method: http.MethodGet, Pattern: pattern,
		Services: services, Template: tmpl}, fn: fn}
}

// post builds a route performing a mutation.
func post(pattern string, fn func(http.ResponseWriter, *http.Request) error, services ...string) route {
	return route{Binding: Binding{Method: http.MethodPost, Pattern: pattern,
		Services: services}, fn: fn}
}

// public marks a route reachable without an authenticated actor.
func public(rt route) route {
	rt.Public = true
	return rt
}

// Route patterns the browser interface serves.
const (
	RouteRoot     = "/"
	RouteLogin    = "/login"
	RouteLogout   = "/logout"
	RouteActivity = "/activity"

	RouteProjects     = "/projects"
	RouteProject      = "/projects/{key}"
	RouteProjectEdit  = "/projects/{key}/update"
	RouteProjectArch  = "/projects/{key}/archive"
	RouteProjectDel   = "/projects/{key}/delete"
	RouteBoardMove    = "/projects/{key}/move"
	RouteFields       = "/projects/{key}/fields"
	RouteFieldDelete  = "/projects/{key}/fields/delete"
	RouteWorkflows    = "/workflows"
	RouteWorkflow     = "/workflows/{key}"
	RouteWorkflowDel  = "/workflows/{key}/delete"
	RouteTasks        = "/tasks"
	RouteTask         = "/tasks/{ref}"
	RouteTaskMove     = "/tasks/{ref}/transition"
	RouteTaskDelete   = "/tasks/{ref}/delete"
	RouteTaskRestore  = "/tasks/{ref}/restore"
	RouteTaskDeps     = "/tasks/{ref}/deps"
	RouteTaskDepDel   = "/tasks/{ref}/deps/remove"
	RouteTaskTags     = "/tasks/{ref}/tags"
	RouteTaskTagDel   = "/tasks/{ref}/tags/remove"
	RouteTaskComments = "/tasks/{ref}/comments"
	RouteCommentEdit  = "/tasks/{ref}/comments/edit"
	RouteCommentDel   = "/tasks/{ref}/comments/delete"
	RouteTaskArts     = "/tasks/{ref}/artifacts"

	RouteTenant        = "/admin/tenant"
	RouteMembers       = "/admin/tenant/members"
	RouteMemberRemove  = "/admin/tenant/members/remove"
	RouteRetention     = "/admin/tenant/retention"
	RoutePrune         = "/admin/tenant/prune"
	RouteDomains       = "/admin/domains"
	RouteDomainRemove  = "/admin/domains/remove"
	RouteUsers         = "/admin/users"
	RouteUserUpdate    = "/admin/users/update"
	RouteUserDelete    = "/admin/users/delete"
	RouteTokens        = "/admin/tokens"
	RouteTokenRevoke   = "/admin/tokens/revoke" // #nosec G101 -- a url path, not a credential
	RouteWebhooks      = "/admin/webhooks"
	RouteWebhookDelete = "/admin/webhooks/delete"
	RouteRedeliver     = "/admin/webhooks/deliveries/redeliver"

	RouteTransfer    = "/transfer"
	RouteExport      = "/transfer/export"
	RouteImport      = "/transfer/import"
	RouteSync        = "/sync"
	RouteSyncSources = "/sync/sources"
	RouteSyncDelete  = "/sync/sources/delete"
	RouteSyncRun     = "/sync/run"
)

// Route patterns for component sharing.
const (
	RouteBundles      = "/bundles"
	RouteBundleExport = "/bundles/export"
	RouteBundleImport = "/bundles/import"
)
