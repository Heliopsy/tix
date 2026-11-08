// SPDX-License-Identifier: AGPL-3.0-or-later

package authz

import "github.com/heliopsy/tix/internal/core"

// Resource identifies what an action is being performed on.
type Resource struct {
	TenantID  string
	ProjectID string
	OwnerID   string
}

// Policy answers authorization questions. It holds no state and is safe to share.
type Policy struct{}

// New returns the authorization policy.
func New() Policy { return Policy{} }

// Can returns nil when the actor may perform the action on the resource.
func (p Policy) Can(actor *core.Actor, action Action, res Resource) error {
	if actor == nil {
		return core.Unauthenticated("no authenticated actor")
	}
	if action == ActionGlobalAdmin {
		return p.deny(action, res, "global admin is decided by CanGlobal, not by a scope")
	}
	if action == ActionAccountRead {
		return p.deny(action, res, "reading an account is decided by CanReadAccount, not by a role")
	}
	if res.TenantID != "" && res.TenantID != actor.TenantID {
		return core.NotFound("resource not found")
	}
	if actor.ScopedToProject() {
		switch {
		case !action.ProjectConfinable():
			return p.deny(action, res, "token is pinned to a project and this action is not confined to one")
		case res.ProjectID != "" && res.ProjectID != actor.ProjectID:
			return p.deny(action, res, "token is restricted to another project")
		}
	}
	scope, ok := action.Scope()
	if !ok {
		return p.deny(action, res, "unknown action")
	}
	if !actor.HasScope(scope) {
		return p.deny(action, res, "missing required scope")
	}
	return nil
}

// GlobalStanding is what the caller of CanGlobal found out about the actor,
// since the policy holds no state and cannot look.
type GlobalStanding struct {
	// DirectStore is true when the call comes from a process that opened the
	// database itself with no credential.
	DirectStore bool
	// GlobalAdmin is true when the actor's user is a global admin right now.
	GlobalAdmin bool
}

// CanGlobal returns nil when the actor may act as a global admin.
//
// Holding the database is enough, because the file already holds every row.
// Otherwise the actor's user has to be a global admin at the moment of the
// call, so a revoke takes effect on live sessions and tokens at once. A token
// needs global:admin named among its scopes as well: "*" does not carry it,
// so no token minted before global admins existed gained the authority.
func (p Policy) CanGlobal(actor *core.Actor, standing GlobalStanding) error {
	if actor == nil {
		return core.Unauthenticated("no authenticated actor")
	}
	if standing.DirectStore && actor.TokenID == "" {
		return nil
	}
	if actor.ScopedToProject() {
		return p.deny(ActionGlobalAdmin, Resource{}, "token is pinned to a project and this action is not confined to one")
	}
	if !standing.GlobalAdmin {
		return p.deny(ActionGlobalAdmin, Resource{}, "only a global admin may do this")
	}
	if actor.TokenID != "" && !actor.HasScope(core.ScopeGlobalAdmin) {
		return p.deny(ActionGlobalAdmin, Resource{}, "the token does not carry the global:admin scope")
	}
	return nil
}

// CanReadAccount decides whether the caller may read their own account. A
// browser session is the person and may. A token needs account:read named
// among its scopes, because "*" does not carry it: an agent's token in one
// tenant must learn nothing about the person's other tenants. A token pinned
// to a project is confined to it and never reads the account.
func (p Policy) CanReadAccount(actor *core.Actor) error {
	if actor == nil {
		return core.Unauthenticated("no authenticated actor")
	}
	if actor.TokenID == "" {
		return nil
	}
	if actor.ScopedToProject() {
		return p.deny(ActionAccountRead, Resource{}, "token is pinned to a project and this action is not confined to one")
	}
	if !actor.HasScope(core.ScopeAccountRead) {
		return p.deny(ActionAccountRead, Resource{},
			"reading your account log with a token needs the account:read scope")
	}
	return nil
}

// CanRotateToken decides whether the actor may rotate a token held by owner
// and pinned to project, which is empty for an unpinned token. The token's own
// actor may, so an agent rotates its credential without token:admin; a pinned
// caller only a token pinned to its own project, or it could mint itself an
// unpinned one through a sibling. Anybody else needs token:admin. Which scopes
// the new token may carry is the caller's ceiling, decided by the service.
func (p Policy) CanRotateToken(actor *core.Actor, owner, project string) error {
	if actor == nil {
		return core.Unauthenticated("no authenticated actor")
	}
	if owner != actor.ID {
		return p.Can(actor, ActionTokenAdmin, Resource{ProjectID: project, OwnerID: owner})
	}
	if actor.ScopedToProject() && project != actor.ProjectID {
		return p.deny(ActionTokenAdmin, Resource{ProjectID: project, OwnerID: owner},
			"a token pinned to a project may rotate only a token pinned to the same project")
	}
	return nil
}

// Allowed reports whether the actor may act, for UI affordances that need no error.
func (p Policy) Allowed(actor *core.Actor, action Action, res Resource) bool {
	return p.Can(actor, action, res) == nil
}

// deny builds the forbidden error, naming the action and the scope it needed.
func (p Policy) deny(action Action, res Resource, msg string) error {
	err := core.Forbidden("%s", msg).WithDetail("action", string(action))
	if scope, ok := action.Scope(); ok {
		err = err.WithDetail("scope", string(scope))
	}
	if res.ProjectID != "" {
		err = err.WithDetail("project_id", res.ProjectID)
	}
	if res.OwnerID != "" {
		err = err.WithDetail("owner_id", res.OwnerID)
	}
	return err
}
