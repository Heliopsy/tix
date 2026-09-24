// SPDX-License-Identifier: AGPL-3.0-or-later

// Package wire holds the tix HTTP wire contract: route patterns, header and
// content-type names, and the envelope types both the server and the client
// marshal. It carries no behaviour and depends on nothing but internal/core.
package wire

import "github.com/heliopsy/tix/internal/core"

// APIPrefix is the versioned namespace every resource route lives under.
const APIPrefix = "/api/v1"

// Route patterns, shared by the server that registers them and the client that
// calls them so the two cannot drift.
const (
	RouteHealth = "/healthz"
	RouteReady  = "/readyz"

	RouteWhoAmI = APIPrefix + "/whoami"

	RouteLogin  = APIPrefix + "/auth/login"
	RouteLogout = APIPrefix + "/auth/logout"

	RouteActor = APIPrefix + "/actors/{id}"

	RouteUsers = APIPrefix + "/users"
	RouteUser  = APIPrefix + "/users/{id}"

	RouteTokens = APIPrefix + "/tokens"
	RouteToken  = APIPrefix + "/tokens/{id}"

	RouteSSHKeys = APIPrefix + "/ssh-keys"
	RouteSSHKey  = APIPrefix + "/ssh-keys/{id}"

	RouteTenants = APIPrefix + "/tenants"
	RouteTenant  = APIPrefix + "/tenants/{ref}"
	RouteMembers = APIPrefix + "/members"
	RouteMember  = APIPrefix + "/members/{actorID}"
	RouteDomains = APIPrefix + "/domains"
	RouteDomain  = APIPrefix + "/domains/{hostname}"

	RouteProjects      = APIPrefix + "/projects"
	RouteProject       = APIPrefix + "/projects/{ref}"
	RouteProjectFields = APIPrefix + "/projects/{ref}/fields"
	RouteProjectField  = APIPrefix + "/projects/{ref}/fields/{key}"

	RouteWorkflows = APIPrefix + "/workflows"
	RouteWorkflow  = APIPrefix + "/workflows/{key}"

	RouteTasks          = APIPrefix + "/tasks"
	RouteTask           = APIPrefix + "/tasks/{ref}"
	RouteTaskTransition = APIPrefix + "/tasks/{ref}/transition"
	RouteTaskRestore    = APIPrefix + "/tasks/{ref}/restore"
	RouteTaskTree       = APIPrefix + "/tasks/{ref}/tree"
	RouteTaskDeps       = APIPrefix + "/tasks/{ref}/deps"
	RouteTaskDep        = APIPrefix + "/tasks/{ref}/deps/{dep}"
	RouteTaskLabels     = APIPrefix + "/tasks/{ref}/tags"
	RouteTaskLabel      = APIPrefix + "/tasks/{ref}/tags/{name}"
	RouteTaskComments   = APIPrefix + "/tasks/{ref}/comments"
	RouteTaskArtifacts  = APIPrefix + "/tasks/{ref}/artifacts"
	RouteTaskAudit      = APIPrefix + "/tasks/{ref}/audit"

	RouteComment = APIPrefix + "/comments/{id}"
	RouteLabels  = APIPrefix + "/tags"

	RouteTaskClaim        = APIPrefix + "/tasks/{ref}/claim"
	RouteTaskClaimRenew   = APIPrefix + "/tasks/{ref}/claim/renew"
	RouteTaskClaimRelease = APIPrefix + "/tasks/{ref}/claim/release"
	RouteClaimNext        = APIPrefix + "/claims/next"
	RouteClaimSweep       = APIPrefix + "/claims/sweep"

	RouteWebhooks          = APIPrefix + "/webhooks"
	RouteWebhook           = APIPrefix + "/webhooks/{id}"
	RouteDeliveries        = APIPrefix + "/webhooks/deliveries"
	RouteDeliveryRedeliver = APIPrefix + "/webhooks/deliveries/{id}/redeliver"

	RouteStats     = APIPrefix + "/stats"
	RouteAudit     = APIPrefix + "/audit"
	RouteRetention = APIPrefix + "/retention"
	RoutePrune     = APIPrefix + "/prune"

	RouteExport = APIPrefix + "/export"
	RouteImport = APIPrefix + "/import"

	RouteSyncSources = APIPrefix + "/sync/sources"
	RouteSyncSource  = APIPrefix + "/sync/sources/{id}"
	RouteSyncRun     = APIPrefix + "/sync/run"

	RouteEvents = APIPrefix + "/events"

	RouteConnections = APIPrefix + "/connections"
	RouteConnection  = APIPrefix + "/connections/{id}"
)

// Content types.
const (
	ContentJSON   = "application/json"
	ContentNDJSON = "application/x-ndjson"
)

// Header names.
const (
	HeaderAuth        = "Authorization"
	HeaderContentType = "Content-Type"
	HeaderAccept      = "Accept"
	SessionCookieName = "tix_session"
)

// ErrorBody is the single error envelope every failing response carries.
type ErrorBody struct {
	Code    core.Kind      `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// ErrorEnvelope wraps ErrorBody so the payload is self-describing.
type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// Page is the envelope every list response uses. NextCursor is empty on the
// final page.
type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// Route patterns for component sharing, which moves reusable configuration
// between projects, tenants and installations.
const (
	RouteBundleExport = APIPrefix + "/bundles/export"
	RouteBundleImport = APIPrefix + "/bundles/import"
)

// ContentBundle is the media type a component bundle is served as.
const ContentBundle = "application/vnd.tix.bundle+json"

// HeaderContentDisposition names the download a bundle response carries.
const HeaderContentDisposition = "Content-Disposition"

// TenantSelf is the reference that means "the tenant the caller is already in".
//
// The service contract lets an empty reference mean that, which a direct caller
// can express and a URL cannot: an empty path segment addresses no route at all.
// So the two sides agree on a stand-in. The client sends it in place of an empty
// reference and the server turns it back into one before the service sees it, so
// both transports honour the same convention.
//
// It starts with a character a tenant key may not: a key must begin with a
// letter and an identifier is alphanumeric, so this can never be mistaken for
// either. It is also unreserved in RFC 3986 and needs no escaping.
const TenantSelf = "~"
