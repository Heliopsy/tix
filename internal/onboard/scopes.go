// SPDX-License-Identifier: AGPL-3.0-or-later

package onboard

import "github.com/heliopsy/tix/internal/core"

// ScopeGroup is a headed set of scopes, in the order a person looks for them.
type ScopeGroup struct {
	ID     string
	Name   string
	Scopes []core.Scope
}

// ScopeNotes says what each scope allows, in one line. The browser's scope
// picker and the terminal's draw from it, and a test fails on a scope that
// ScopeGroups places and this does not describe.
var ScopeNotes = map[core.Scope]string{
	core.ScopeTaskRead:       "Read tasks, their comments and history.",
	core.ScopeTaskWrite:      "Create and edit tasks.",
	core.ScopeTaskTransition: "Move a task from one state to another.",
	core.ScopeTaskClaim:      "Claim tasks, renew the lease and release it.",
	core.ScopeTaskDelete:     "Delete tasks and restore them from the trash.",
	core.ScopeTaskReclaim:    "End a lease another worker holds.",
	core.ScopeProjectRead:    "Read projects.",
	core.ScopeProjectWrite:   "Create and change projects.",
	core.ScopeWorkflowRead:   "Read workflows.",
	core.ScopeWorkflowWrite:  "Create and change workflows.",
	core.ScopeCommentWrite:   "Comment on tasks.",
	core.ScopeArtifactWrite:  "Attach artifacts to tasks.",
	core.ScopeEventSubscribe: "Follow the live event stream.",
	core.ScopeUserAdmin:      "Create, change and remove users.",
	core.ScopeTOTPAdmin:      "Clear another person's second factor.",
	core.ScopeTokenAdmin:     "Issue and revoke API tokens.",
	core.ScopeWebhookAdmin:   "Manage outgoing webhooks.",
	core.ScopeTenantAdmin:    "Manage the tenant, its members and domains.",
	core.ScopeSyncAdmin:      "Manage and run imports from other trackers.",
	core.ScopeAuditRead:      "Read the audit log.",
	core.ScopeExport:         "Export a snapshot of the whole tenant.",
	core.ScopeImport:         "Import a snapshot into the tenant.",
	core.ScopeGlobalAdmin:    "Administer every tenant on this server. Only a global admin's token can carry it.",
	core.ScopeAccountRead:    "Read your own account's history, which names every tenant you belong to. Only a token for your own account can carry it.",
}

// ScopeGroups orders the scopes the way a person looks for them.
var ScopeGroups = []ScopeGroup{
	{"tasks", "Tasks", []core.Scope{core.ScopeTaskRead, core.ScopeTaskWrite, core.ScopeTaskTransition,
		core.ScopeTaskClaim, core.ScopeTaskDelete, core.ScopeTaskReclaim}},
	{"projects", "Projects", []core.Scope{core.ScopeProjectRead, core.ScopeProjectWrite}},
	{"workflows", "Workflows", []core.Scope{core.ScopeWorkflowRead, core.ScopeWorkflowWrite}},
	{"comments", "Comments and artifacts", []core.Scope{core.ScopeCommentWrite, core.ScopeArtifactWrite}},
	{"events", "Events", []core.Scope{core.ScopeEventSubscribe}},
	{"admin", "Administration", []core.Scope{core.ScopeUserAdmin, core.ScopeTOTPAdmin, core.ScopeTokenAdmin,
		core.ScopeWebhookAdmin, core.ScopeTenantAdmin, core.ScopeSyncAdmin, core.ScopeAuditRead,
		core.ScopeExport, core.ScopeImport, core.ScopeGlobalAdmin}},
	{"account", "Your account", []core.Scope{core.ScopeAccountRead}},
}

// TokenExpiry is one expiry a token form offers. Days is how long the token
// lasts; zero means it never expires.
//
// Whole days rather than an instant, because "90 days" is the decision an
// operator is making and a datetime field makes them do the arithmetic, in a
// zone neither side has agreed on. The command line takes an absolute date.
type TokenExpiry struct {
	Key   string
	Label string
	Days  int
}

// TokenExpiries are the expiries a token form offers, and the only values it
// accepts.
var TokenExpiries = []TokenExpiry{
	{Key: "7d", Label: "7 days", Days: 7},
	{Key: "30d", Label: "30 days", Days: 30},
	{Key: "90d", Label: "90 days", Days: 90},
	{Key: "365d", Label: "1 year", Days: 365},
	{Key: "never", Label: "Never expires", Days: 0},
}

// DefaultTokenExpiry is what a token form proposes before anybody chooses: an
// expiry, not "never", because a token is normally handed to something outside
// the operator's control.
const DefaultTokenExpiry = "90d"
