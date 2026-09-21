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
