// Package httpapi serves the tix REST API and event stream.
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/thereisnotime/tix/internal/core"
)

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

	RouteUsers = APIPrefix + "/users"
	RouteUser  = APIPrefix + "/users/{id}"

	RouteTokens = APIPrefix + "/tokens"
	RouteToken  = APIPrefix + "/tokens/{id}"

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

	RouteAudit     = APIPrefix + "/audit"
	RouteRetention = APIPrefix + "/retention"
	RoutePrune     = APIPrefix + "/prune"

	RouteExport = APIPrefix + "/export"
	RouteImport = APIPrefix + "/import"

	RouteSyncSources = APIPrefix + "/sync/sources"
	RouteSyncSource  = APIPrefix + "/sync/sources/{id}"
	RouteSyncRun     = APIPrefix + "/sync/run"

	RouteEvents = APIPrefix + "/events"
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

// WriteError renders err as the standard envelope with its mapped status. An
// internal failure is reported generically, so no SQL text, stack trace or
// credential can reach a client.
func WriteError(w http.ResponseWriter, err error) {
	kind := core.KindOf(err)
	body := ErrorBody{Code: kind, Message: err.Error()}

	var domain *core.Error
	if errors.As(err, &domain) {
		body.Message = domain.Message
		body.Details = domain.Details
	}
	if kind == core.KindInternal {
		body = ErrorBody{Code: core.KindInternal, Message: "internal error"}
	}

	w.Header().Set(HeaderContentType, ContentJSON)
	w.WriteHeader(kind.HTTPStatus())
	_ = json.NewEncoder(w).Encode(ErrorEnvelope{Error: body})
}

// WriteJSON renders v with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set(HeaderContentType, ContentJSON)
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}
