// SPDX-License-Identifier: AGPL-3.0-or-later

// Package wire holds the tix HTTP wire contract: route patterns, header and
// content-type names, and the envelope types both the server and the client
// marshal. It carries no behaviour and depends on nothing but internal/core.
package wire

import (
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// APIPrefix is the versioned namespace every resource route lives under.
const APIPrefix = "/api/v1"

// Route patterns, shared by the server that registers them and the client that
// calls them so the two cannot drift.
const (
	RouteHealth = "/healthz"
	RouteReady  = "/readyz"

	RouteWhoAmI = APIPrefix + "/whoami"

	// RouteOpenAPI serves the OpenAPI document of this API, without a
	// credential.
	RouteOpenAPI = APIPrefix + "/openapi.json"

	RouteLogin  = APIPrefix + "/auth/login"
	RouteLogout = APIPrefix + "/auth/logout"
	RouteSwitch = APIPrefix + "/auth/switch"

	RouteSSHChallenge = APIPrefix + "/auth/ssh/challenge"
	RouteSSHVerify    = APIPrefix + "/auth/ssh/verify"

	RouteActors = APIPrefix + "/actors"
	RouteActor  = APIPrefix + "/actors/{id}"

	RouteUsers = APIPrefix + "/users"
	RouteUser  = APIPrefix + "/users/{id}"

	// Global admins administer every tenant; the collection is the
	// installation's, not the caller's tenant's.
	RouteGlobalAdmins = APIPrefix + "/global-admins"
	RouteGlobalAdmin  = APIPrefix + "/global-admins/{email}"
	// Invites to sign in, administered by user:admin, and the public routes
	// the invited person previews and accepts one at.
	RouteInvites       = APIPrefix + "/invites"
	RouteInvite        = APIPrefix + "/invites/{id}"
	RouteInviteReissue = APIPrefix + "/invites/{id}/reissue"
	RouteInviteToken   = InviteTokenPrefix + "{token}"

	RouteTokens = APIPrefix + "/tokens"
	RouteToken  = APIPrefix + "/tokens/{id}"
	// RouteTokenRotate replaces a token with a new one of the same name.
	RouteTokenRotate = APIPrefix + "/tokens/{id}/rotate" // #nosec G101 -- a url path, not a credential

	// The second factor of the caller's own account, and the one route that
	// reaches another user's: an administrative clear.
	RouteTOTP        = APIPrefix + "/auth/totp"
	RouteTOTPConfirm = APIPrefix + "/auth/totp/confirm"
	RouteTOTPVerify  = APIPrefix + "/auth/totp/verify"
	RouteTOTPCodes   = APIPrefix + "/auth/totp/recovery-codes"
	RouteUserTOTP    = APIPrefix + "/users/{id}/totp"

	RouteSSHKeys = APIPrefix + "/ssh-keys"
	RouteSSHKey  = APIPrefix + "/ssh-keys/{id}"

	RouteTenants = APIPrefix + "/tenants"
	RouteTenant  = APIPrefix + "/tenants/{ref}"
	// The users of a named tenant, which only a global admin reaches.
	RouteTenantUsers = APIPrefix + "/tenants/{ref}/users"
	RouteTenantUser  = APIPrefix + "/tenants/{ref}/users/{id}"
	// RouteTenantDomains lists a named tenant's domains, for a global admin.
	RouteTenantDomains = APIPrefix + "/tenants/{ref}/domains"
	// RouteAccountLog is the caller's own account log.
	RouteAccountLog = APIPrefix + "/account/log"
	// RouteAccountSessions lists the caller's own sessions in this tenant.
	RouteAccountSessions = APIPrefix + "/account/sessions"
	// RouteAccountSession ends one of the caller's own sessions.
	RouteAccountSession = APIPrefix + "/account/sessions/{id}"
	// RouteAccountLogFor is any account's log, for a global admin.
	RouteAccountLogFor = APIPrefix + "/accounts/{email}/log"
	// RouteTenantMembers adds an existing account to a tenant.
	RouteTenantMembers = APIPrefix + "/tenants/{ref}/members"
	RouteMembers       = APIPrefix + "/members"
	RouteMember        = APIPrefix + "/members/{actorID}"
	RouteDomains       = APIPrefix + "/domains"
	RouteDomain        = APIPrefix + "/domains/{hostname}"

	RouteProjects      = APIPrefix + "/projects"
	RouteProject       = APIPrefix + "/projects/{ref}"
	RouteProjectIcon   = APIPrefix + "/projects/{ref}/icon"
	RouteProjectFields = APIPrefix + "/projects/{ref}/fields"
	// RouteProjectDeletion plans a project's deletion without performing it.
	RouteProjectDeletion = APIPrefix + "/projects/{ref}/deletion"
	RouteProjectField    = APIPrefix + "/projects/{ref}/fields/{key}"
	// RouteProjectStar sets (PUT) and clears (DELETE) the caller's own star.
	RouteProjectStar = APIPrefix + "/projects/{ref}/star"
	// RouteStarredProjects lists the projects the caller starred.
	RouteStarredProjects = APIPrefix + "/me/starred-projects"

	RouteWorkflows = APIPrefix + "/workflows"
	RouteWorkflow  = APIPrefix + "/workflows/{key}"

	// RouteWorkflowTemplates lists the built-in workflow templates.
	RouteWorkflowTemplates = APIPrefix + "/workflow-templates"
	// RouteWorkflowTemplateApply creates a workflow from one template.
	RouteWorkflowTemplateApply = APIPrefix + "/workflow-templates/{key}/apply"

	RouteTasks          = APIPrefix + "/tasks"
	RouteTask           = APIPrefix + "/tasks/{ref}"
	RouteTaskTransition = APIPrefix + "/tasks/{ref}/transition"
	RouteTaskRoute      = APIPrefix + "/tasks/{ref}/route"
	RouteTaskRestore    = APIPrefix + "/tasks/{ref}/restore"
	RouteTaskTrash      = APIPrefix + "/tasks/{ref}/trash"
	RouteTaskTree       = APIPrefix + "/tasks/{ref}/tree"
	RouteTaskDeps       = APIPrefix + "/tasks/{ref}/deps"
	RouteTaskDep        = APIPrefix + "/tasks/{ref}/deps/{dep}"
	RouteTaskLabels     = APIPrefix + "/tasks/{ref}/tags"
	RouteTaskLabel      = APIPrefix + "/tasks/{ref}/tags/{name}"
	RouteTaskStar       = APIPrefix + "/tasks/{ref}/star"
	RouteTaskChecklist  = APIPrefix + "/tasks/{ref}/checklist"
	RouteTaskCopy       = APIPrefix + "/tasks/{ref}/copy"
	RouteTaskMove       = APIPrefix + "/tasks/{ref}/move"
	RouteTaskBulk       = APIPrefix + "/tasks/bulk"
	RouteTaskSnooze     = APIPrefix + "/tasks/{ref}/snooze"
	RouteSnoozeWake     = APIPrefix + "/snoozes/wake"
	RouteTaskComments   = APIPrefix + "/tasks/{ref}/comments"
	RouteTaskArtifacts  = APIPrefix + "/tasks/{ref}/artifacts"
	RouteTaskAudit      = APIPrefix + "/tasks/{ref}/audit"

	// RouteArtifactLimits answers the size and media types a new artifact
	// blob is held to on the tenant.
	RouteArtifactLimits = APIPrefix + "/artifact-limits"

	// RouteTrash answers whether the tenant keeps deleted tasks.
	RouteTrash = APIPrefix + "/trash"

	RouteComment = APIPrefix + "/comments/{id}"
	RouteLabels  = APIPrefix + "/tags"

	RouteTaskClaim        = APIPrefix + "/tasks/{ref}/claim"
	RouteTaskClaimRenew   = APIPrefix + "/tasks/{ref}/claim/renew"
	RouteTaskClaimRelease = APIPrefix + "/tasks/{ref}/claim/release"
	RouteTaskClaimReclaim = APIPrefix + "/tasks/{ref}/claim/reclaim"
	RouteTaskLease        = APIPrefix + "/tasks/{ref}/lease"
	RouteClaimNext        = APIPrefix + "/claims/next"
	RouteClaimSweep       = APIPrefix + "/claims/sweep"

	RouteWebhooks          = APIPrefix + "/webhooks"
	RouteWebhook           = APIPrefix + "/webhooks/{id}"
	RouteWebhookPing       = APIPrefix + "/webhooks/{id}/ping"
	RouteDeliveries        = APIPrefix + "/webhooks/deliveries"
	RouteDeliveryRedeliver = APIPrefix + "/webhooks/deliveries/{id}/redeliver"

	RouteStatus    = APIPrefix + "/status"
	RouteStats     = APIPrefix + "/stats"
	RouteAudit     = APIPrefix + "/audit"
	RouteRetention = APIPrefix + "/retention"
	RoutePrune     = APIPrefix + "/prune"

	// RouteHousekeeping reads and sets the housekeeping policy, and
	// RouteHousekeepingRun applies or previews it once.
	RouteHousekeeping    = APIPrefix + "/housekeeping"
	RouteHousekeepingRun = APIPrefix + "/housekeeping/run"

	RouteExport = APIPrefix + "/export"
	RouteImport = APIPrefix + "/import"

	// RouteProjectTaskExport writes one project's tasks as a file.
	RouteProjectTaskExport = APIPrefix + "/projects/{ref}/tasks/export"
	// RouteProjectTaskImport reads a file of tasks into a project.
	RouteProjectTaskImport = APIPrefix + "/projects/{ref}/tasks/import"

	RouteSyncSources = APIPrefix + "/sync/sources"
	RouteSyncSource  = APIPrefix + "/sync/sources/{id}"
	RouteSyncRun     = APIPrefix + "/sync/run"

	RouteEvents     = APIPrefix + "/events"
	RouteEventsPoll = APIPrefix + "/events/poll"

	RouteConnections = APIPrefix + "/connections"
	RouteConnection  = APIPrefix + "/connections/{id}"

	RouteViewGroups = APIPrefix + "/views/{view}/groups"
	RouteViewGroup  = APIPrefix + "/views/{view}/groups/{group}"
	RouteViewOrder  = APIPrefix + "/views/{view}/order"

	RouteFilters        = APIPrefix + "/filters"
	RouteFilter         = APIPrefix + "/filters/{filter}"
	RouteFilterOrder    = APIPrefix + "/filters/order"
	RouteCalendar       = APIPrefix + "/calendar"
	RouteCalendarRotate = APIPrefix + "/calendar/rotate"
	// RouteCalendarFeed is read by calendar apps with no credential. Its one
	// segment is the feed secret followed by CalendarFeedSuffix, because some
	// clients refuse an address that does not look like a file.
	RouteCalendarFeed = CalendarFeedPrefix + "{file}"
	RouteInbox        = APIPrefix + "/inbox"
	RouteInboxCount   = APIPrefix + "/inbox/count"
	RouteInboxRead    = APIPrefix + "/inbox/{id}/read"
	RouteInboxReadAll = APIPrefix + "/inbox/read-all"
	RouteInboxPrefs   = APIPrefix + "/inbox/prefs"

	// RouteTimeSettings is the caller's date format and timezone.
	RouteTimeSettings = APIPrefix + "/me/time"

	RouteChannels          = APIPrefix + "/channels"
	RouteChannel           = APIPrefix + "/channels/{id}"
	RouteChannelTest       = APIPrefix + "/channels/{id}/test"
	RouteChannelDeliveries = APIPrefix + "/channels/{id}/deliveries"
	RouteChannelRedeliver  = APIPrefix + "/channel-deliveries/{id}/redeliver"

	RouteTemplates   = APIPrefix + "/templates"
	RouteTemplate    = APIPrefix + "/templates/{ref}"
	RouteTemplateUse = APIPrefix + "/templates/{ref}/use"
)

// InviteTokenPrefix starts every public invite route; the secret follows it.
const InviteTokenPrefix = APIPrefix + "/auth/invites/"

// CalendarFeedPrefix starts every feed address, and CalendarFeedSuffix ends it.
const (
	CalendarFeedPrefix = APIPrefix + "/calendar/feeds/"
	CalendarFeedSuffix = ".ics"
)

// CalendarFeedPath is the path a feed secret is read at.
func CalendarFeedPath(token string) string { return CalendarFeedPrefix + token + CalendarFeedSuffix }

// Content types.
const (
	ContentJSON   = "application/json"
	ContentNDJSON = "application/x-ndjson"
)

// IconPolicy is the content security policy a project icon image is served
// under, so an SVG opened on its own runs no script and loads nothing.
const IconPolicy = "default-src 'none'; style-src 'unsafe-inline'; sandbox"

// IconETag is the strong entity tag of a project icon image.
func IconETag(digest string) string { return `"` + digest + `"` }

// Header names.
const (
	HeaderAuth        = "Authorization"
	HeaderContentType = "Content-Type"
	HeaderAccept      = "Accept"
	SessionCookieName = "tix_session"
)

// Idempotency headers. A client sends HeaderIdempotencyKey on a write it may
// retry; a response replayed from an earlier request with the same key carries
// HeaderIdempotentReplayed, and a refusal because the first request is still
// running carries HeaderRetryAfter in seconds.
const (
	HeaderIdempotencyKey     = "Idempotency-Key"
	HeaderIdempotentReplayed = "Idempotent-Replayed"
	HeaderRetryAfter         = "Retry-After"
)

// HeaderClient names the tix client making a request: cli or mcp. The server
// treats it as a hint that refines a token request's access channel and never
// as a credential.
const HeaderClient = "X-Tix-Client"

// ErrorBody is the single error envelope every failing response carries.
type ErrorBody struct {
	// Code is the error kind, which decides the HTTP status.
	Code core.Kind `json:"code"`
	// Message is a human-readable explanation.
	Message string `json:"message"`
	// Details carries structured context when there is some, such as "field"
	// naming the input at fault or "reason" refining the kind.
	Details map[string]any `json:"details,omitempty"`
}

// ErrorEnvelope wraps ErrorBody so the payload is self-describing.
type ErrorEnvelope struct {
	// Error describes what went wrong.
	Error ErrorBody `json:"error"`
}

// Page is the envelope every list response uses. NextCursor is empty on the
// final page.
type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
	// Total is how many items the listing's filter selects, on a listing
	// that counts (the task listing). It is absent where nothing is counted.
	Total *int `json:"total,omitempty"`
	// TotalCapped is true when the count stopped at its cap, 10000 tasks, so
	// total is a floor rather than the whole.
	TotalCapped bool `json:"total_capped,omitempty"`
	// Page is the page number served, from one, when the request asked by
	// page; a number past the end is served as the last page.
	Page int `json:"page,omitempty"`
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

// ContentCSV is the media type a task listing is served as when a client asks
// for CSV.
const ContentCSV = "text/csv"

// ContentMultipart is the media type of a form upload.
const ContentMultipart = "multipart/form-data"

// ContentMarkdown is the media type a task export is served as in Markdown.
const ContentMarkdown = "text/markdown"

// TaskExportFilename is the download name of a project's task export. Only
// letters, digits, dashes and underscores of the project reference survive,
// so the name cannot break out of the header that carries it.
func TaskExportFilename(project, ext string) string {
	var b strings.Builder
	for _, r := range project {
		if r == '-' || r == '_' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "tasks." + ext
	}
	return strings.ToLower(b.String()) + "-tasks." + ext
}

// TaskFormatContent is the media type a task export in a format is served as.
func TaskFormatContent(format string) string {
	switch format {
	case "json":
		return ContentJSON
	case "ndjson":
		return ContentNDJSON
	case "markdown":
		return ContentMarkdown
	default:
		return ContentCSV
	}
}

// HeaderNextCursor names the cursor of the next page on a CSV listing, which
// has no envelope to carry it in.
const HeaderNextCursor = "X-Tix-Next-Cursor"
