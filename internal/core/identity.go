// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"context"
	"slices"
	"strings"
	"time"
)

// ActorKind distinguishes the sort of worker an Actor represents.
type ActorKind string

// Actor kinds.
const (
	ActorUser   ActorKind = "user"
	ActorAgent  ActorKind = "agent"
	ActorSystem ActorKind = "system"
)

// Valid reports whether k is a known actor kind.
func (k ActorKind) Valid() bool {
	switch k {
	case ActorUser, ActorAgent, ActorSystem:
		return true
	default:
		return false
	}
}

// Scope is a coarse permission over a resource and verb, such as "task:claim".
type Scope string

// ScopeAll grants every scope.
const ScopeAll Scope = "*"

// Scopes.
const (
	ScopeTaskRead       Scope = "task:read"
	ScopeTaskWrite      Scope = "task:write"
	ScopeTaskTransition Scope = "task:transition"
	ScopeTaskClaim      Scope = "task:claim"
	ScopeTaskDelete     Scope = "task:delete"

	// ScopeTaskReclaim takes a live lease away from whoever holds it. It is
	// separate from task:claim because every worker holds that one, and a
	// worker that can end another worker's lease can end work it knows
	// nothing about. It is separate from tenant:admin because the authority
	// it grants reaches one task at a time and nothing else the tenant owns,
	// so an on-call operator can be given it without being given the tenant.
	ScopeTaskReclaim Scope = "task:reclaim"

	ScopeProjectRead  Scope = "project:read"
	ScopeProjectWrite Scope = "project:write"

	ScopeWorkflowRead  Scope = "workflow:read"
	ScopeWorkflowWrite Scope = "workflow:write"

	ScopeCommentWrite  Scope = "comment:write"
	ScopeArtifactWrite Scope = "artifact:write"

	ScopeEventSubscribe Scope = "event:subscribe"

	ScopeUserAdmin Scope = "user:admin"
	// ScopeTOTPAdmin clears another user's second factor, which returns them
	// to an unenrolled state. It is separate from user:admin because it is
	// the one recovery path when a person loses both their device and their
	// codes, and because granting it does not carry the authority to create,
	// disable or delete accounts.
	ScopeTOTPAdmin    Scope = "totp:admin"
	ScopeTokenAdmin   Scope = "token:admin"
	ScopeWebhookAdmin Scope = "webhook:admin"
	ScopeTenantAdmin  Scope = "tenant:admin"
	ScopeSyncAdmin    Scope = "sync:admin"

	// ScopeGlobalAdmin lets a token act for a global admin across tenants. "*"
	// does not imply it: it is carried only when named, and only by a token
	// whose actor is a global admin when the call is made.
	ScopeGlobalAdmin Scope = "global:admin"

	ScopeAuditRead Scope = "audit:read"
	ScopeExport    Scope = "export"
	ScopeImport    Scope = "import"
)

// AllScopes is the complete vocabulary, excluding ScopeAll.
var AllScopes = []Scope{
	ScopeTaskRead, ScopeTaskWrite, ScopeTaskTransition, ScopeTaskClaim, ScopeTaskDelete,
	ScopeTaskReclaim,
	ScopeProjectRead, ScopeProjectWrite,
	ScopeWorkflowRead, ScopeWorkflowWrite,
	ScopeCommentWrite, ScopeArtifactWrite,
	ScopeEventSubscribe,
	ScopeUserAdmin, ScopeTOTPAdmin, ScopeTokenAdmin, ScopeWebhookAdmin, ScopeTenantAdmin, ScopeSyncAdmin,
	ScopeAuditRead, ScopeExport, ScopeImport,
	ScopeGlobalAdmin,
}

// Role is a named bundle of scopes granted to a human within one tenant.
type Role string

// Roles.
const (
	RoleViewer Role = "viewer"
	RoleMember Role = "member"
	RoleAdmin  Role = "admin"
)

// Roles lists every role, least privileged first.
var Roles = []Role{RoleViewer, RoleMember, RoleAdmin}

// Valid reports whether r is a known role.
func (r Role) Valid() bool { return slices.Contains(Roles, r) }

// Scopes returns the scopes granted by the role.
func (r Role) Scopes() []Scope {
	switch r {
	case RoleViewer:
		return []Scope{
			ScopeTaskRead, ScopeProjectRead, ScopeWorkflowRead,
			ScopeEventSubscribe, ScopeAuditRead,
		}
	case RoleMember:
		return []Scope{
			ScopeTaskRead, ScopeTaskWrite, ScopeTaskTransition, ScopeTaskClaim,
			ScopeProjectRead, ScopeWorkflowRead,
			ScopeCommentWrite, ScopeArtifactWrite,
			ScopeEventSubscribe, ScopeAuditRead, ScopeExport,
		}
	case RoleAdmin:
		return []Scope{ScopeAll}
	default:
		return nil
	}
}

// Actor is the authenticated identity performing an operation.
type Actor struct {
	// ID is the actor's stable identifier.
	ID string `json:"id" yaml:"id"`
	// TenantID is the tenant the actor acts in.
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	// Kind says whether the actor is a user, an agent or the system.
	Kind ActorKind `json:"kind" yaml:"kind"`
	// Handle is the actor's short unique name within the tenant.
	Handle string `json:"handle" yaml:"handle"`
	// DisplayName is the human-readable name, when one is set.
	DisplayName string `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	// Scopes are the scopes granted directly, as an API token's are; empty
	// when authority comes from Role alone.
	Scopes []Scope `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	// Role is the human's role in the tenant, whose scopes add to Scopes.
	Role Role `json:"role,omitempty" yaml:"role,omitempty"`
	// TokenID is the API token the request authenticated with, if any.
	TokenID string `json:"token_id,omitempty" yaml:"token_id,omitempty"`
	// ProjectID restricts the actor to one project when set, as a
	// project-scoped token does.
	ProjectID string `json:"project_id,omitempty" yaml:"project_id,omitempty"`
	// GlobalAdmin reports, on the answer to WhoAmI, that the actor may act as a
	// global admin right now. It is a report for interfaces deciding what to
	// offer; authorization never reads it, and asks the store instead.
	GlobalAdmin bool `json:"global_admin,omitempty" yaml:"global_admin,omitempty"`
}

// HasScope reports whether the actor holds the scope, directly or by role.
func (a *Actor) HasScope(s Scope) bool {
	if a == nil {
		return false
	}
	if s == ScopeGlobalAdmin {
		return slices.Contains(a.Scopes, ScopeGlobalAdmin)
	}
	if slices.Contains(a.Scopes, ScopeAll) {
		return true
	}
	if slices.Contains(a.Scopes, s) {
		return true
	}
	roleScopes := a.Role.Scopes()
	if slices.Contains(roleScopes, ScopeAll) {
		return true
	}
	return slices.Contains(roleScopes, s)
}

// ScopedToProject reports whether the actor is restricted to a single project.
func (a *Actor) ScopedToProject() bool { return a != nil && a.ProjectID != "" }

// SystemActor builds the actor that imports, sweeps and pruning act as.
func SystemActor(tenantID string) *Actor {
	return &Actor{
		ID:       "system",
		TenantID: tenantID,
		Kind:     ActorSystem,
		Handle:   "system",
		Scopes:   []Scope{ScopeAll},
	}
}

// TenantScope pins an operation to one tenant.
type TenantScope struct {
	TenantID string
}

// Valid reports whether the scope names a tenant.
func (t TenantScope) Valid() bool { return strings.TrimSpace(t.TenantID) != "" }

type ctxKey int

const (
	ctxKeyActor ctxKey = iota
	ctxKeyTenant
	ctxKeySource
	ctxKeyAudience
	ctxKeyDirectStore
)

// Source records which access path an operation arrived through.
type Source string

// Sources.
const (
	SourceCLI    Source = "cli"
	SourceAPI    Source = "api"
	SourceWeb    Source = "web"
	SourceTUI    Source = "tui"
	SourceSystem Source = "system"
)

// WithActor returns a context carrying the actor.
func WithActor(ctx context.Context, a *Actor) context.Context {
	ctx = context.WithValue(ctx, ctxKeyActor, a)
	if a != nil && a.TenantID != "" {
		ctx = context.WithValue(ctx, ctxKeyTenant, TenantScope{TenantID: a.TenantID})
	}
	return ctx
}

// ActorFrom returns the actor carried by ctx, if any.
func ActorFrom(ctx context.Context) (*Actor, bool) {
	a, ok := ctx.Value(ctxKeyActor).(*Actor)
	return a, ok && a != nil
}

// RequireActor returns the actor carried by ctx, or an unauthenticated error.
func RequireActor(ctx context.Context) (*Actor, error) {
	a, ok := ActorFrom(ctx)
	if !ok {
		return nil, Unauthenticated("no authenticated actor")
	}
	return a, nil
}

// WithAudience returns a context naming the host a request was made to, which
// an SSH sign-in binds its challenge and signature to. Only a transport sets
// it, from the request itself and never from the body.
func WithAudience(ctx context.Context, host string) context.Context {
	return context.WithValue(ctx, ctxKeyAudience, host)
}

// AudienceFrom returns the audience carried by ctx, or AudienceLocal when the
// call did not arrive over a network.
func AudienceFrom(ctx context.Context) string {
	if a, ok := ctx.Value(ctxKeyAudience).(string); ok && strings.TrimSpace(a) != "" {
		return strings.ToLower(strings.TrimSpace(a))
	}
	return AudienceLocal
}

// WithDirectStore marks a call made by a process that opened the database
// itself, with no credential: whoever can open the file already holds every
// row in it, so such a call stands as a global admin. Only the command line's
// local connection sets it; nothing arriving over a network can.
func WithDirectStore(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKeyDirectStore, true)
}

// DirectStoreFrom reports whether ctx was marked by WithDirectStore.
func DirectStoreFrom(ctx context.Context) bool {
	v, _ := ctx.Value(ctxKeyDirectStore).(bool)
	return v
}

// WithTenant returns a context carrying the tenant scope.
func WithTenant(ctx context.Context, t TenantScope) context.Context {
	return context.WithValue(ctx, ctxKeyTenant, t)
}

// TenantFrom returns the tenant scope carried by ctx, if any.
func TenantFrom(ctx context.Context) (TenantScope, bool) {
	t, ok := ctx.Value(ctxKeyTenant).(TenantScope)
	return t, ok && t.Valid()
}

// RequireTenant returns the tenant scope carried by ctx, or an error.
func RequireTenant(ctx context.Context) (TenantScope, error) {
	t, ok := TenantFrom(ctx)
	if !ok {
		return TenantScope{}, Invalid("no tenant in context")
	}
	return t, nil
}

// WithSource returns a context recording the access path.
func WithSource(ctx context.Context, s Source) context.Context {
	return context.WithValue(ctx, ctxKeySource, s)
}

// SourceFrom returns the access path in ctx, or SourceSystem.
func SourceFrom(ctx context.Context) Source {
	if s, ok := ctx.Value(ctxKeySource).(Source); ok && s != "" {
		return s
	}
	return SourceSystem
}

// Membership grants a human a role within one tenant.
type Membership struct {
	// TenantID is the tenant the membership belongs to.
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	// ActorID is the member's actor ID.
	ActorID string `json:"actor_id" yaml:"actor_id"`
	// Role is the role granted, which decides the member's scopes.
	Role Role `json:"role" yaml:"role"`
	// CreatedAt is when the membership was granted, RFC 3339.
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
}
