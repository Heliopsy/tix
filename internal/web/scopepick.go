// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"slices"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/onboard"
)

// scopeOption is one checkbox of the scope picker.
type scopeOption struct {
	Value core.Scope
	// Note says what the scope allows, in one line.
	Note string
	// Held is whether the reader holds the scope and so may grant it. One
	// they do not hold is drawn disabled with the reason, never left out.
	Held    bool
	Checked bool
}

// scopeGroup is a headed set of checkboxes with its own select-all.
type scopeGroup struct {
	ID     string
	Name   string
	Scopes []scopeOption
}

// scopePreset ticks a named bundle of scopes and nothing else.
type scopePreset struct {
	ID     string
	Name   string
	Note   string
	Scopes string
}

// scopePicker is the one scope control every token form draws: the tokens
// screen's dialog and the connect screen.
type scopePicker struct {
	Groups  []scopeGroup
	Presets []scopePreset
	// All is whether the reader holds *, which is then offered as its own
	// choice with a warning. Select all never ticks it.
	All        bool
	AllChecked bool
}

// scopeNotes says what each scope allows. scopeGroups places each one, and
// a test fails on a scope in neither.
var scopeNotes = map[core.Scope]string{
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

// scopeGroups orders the scopes the way a person looks for them.
var scopeGroups = []struct {
	ID     string
	Name   string
	Scopes []core.Scope
}{
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

// newScopePicker draws the picker for a reader, with chosen ticked.
func newScopePicker(actor *core.Actor, chosen []core.Scope) scopePicker {
	out := scopePicker{All: actor.HasScope(core.ScopeAll), AllChecked: slices.Contains(chosen, core.ScopeAll)}
	for _, g := range scopeGroups {
		group := scopeGroup{ID: g.ID, Name: g.Name}
		for _, s := range g.Scopes {
			group.Scopes = append(group.Scopes, scopeOption{Value: s, Note: scopeNotes[s],
				Held: actor.HasScope(s), Checked: slices.Contains(chosen, s)})
		}
		out.Groups = append(out.Groups, group)
	}
	for _, p := range onboard.Presets {
		if heldAll(actor, p.Scopes) {
			names := make([]string, len(p.Scopes))
			for i, s := range p.Scopes {
				names[i] = string(s)
			}
			out.Presets = append(out.Presets, scopePreset{ID: p.ID, Name: p.Name, Note: p.Note,
				Scopes: strings.Join(names, " ")})
		}
	}
	return out
}

// heldAll reports whether the actor holds every scope.
func heldAll(actor *core.Actor, scopes []core.Scope) bool {
	for _, s := range scopes {
		if !actor.HasScope(s) {
			return false
		}
	}
	return true
}

// heldScopes are the scopes of list the actor holds, in order.
func heldScopes(actor *core.Actor, list []core.Scope) []core.Scope {
	var out []core.Scope
	for _, s := range list {
		if actor.HasScope(s) {
			out = append(out, s)
		}
	}
	return out
}
