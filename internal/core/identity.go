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

	ScopeProjectRead  Scope = "project:read"
	ScopeProjectWrite Scope = "project:write"

	ScopeWorkflowRead  Scope = "workflow:read"
	ScopeWorkflowWrite Scope = "workflow:write"

	ScopeCommentWrite  Scope = "comment:write"
	ScopeArtifactWrite Scope = "artifact:write"

	ScopeEventSubscribe Scope = "event:subscribe"

	ScopeUserAdmin    Scope = "user:admin"
	ScopeTokenAdmin   Scope = "token:admin"
	ScopeWebhookAdmin Scope = "webhook:admin"
	ScopeTenantAdmin  Scope = "tenant:admin"
	ScopeSyncAdmin    Scope = "sync:admin"

	ScopeAuditRead Scope = "audit:read"
	ScopeExport    Scope = "export"
	ScopeImport    Scope = "import"
)

// AllScopes is the complete vocabulary, excluding ScopeAll.
var AllScopes = []Scope{
	ScopeTaskRead, ScopeTaskWrite, ScopeTaskTransition, ScopeTaskClaim, ScopeTaskDelete,
	ScopeProjectRead, ScopeProjectWrite,
	ScopeWorkflowRead, ScopeWorkflowWrite,
	ScopeCommentWrite, ScopeArtifactWrite,
	ScopeEventSubscribe,
	ScopeUserAdmin, ScopeTokenAdmin, ScopeWebhookAdmin, ScopeTenantAdmin, ScopeSyncAdmin,
	ScopeAuditRead, ScopeExport, ScopeImport,
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
	ID          string    `json:"id" yaml:"id"`
	TenantID    string    `json:"tenant_id" yaml:"tenant_id"`
	Kind        ActorKind `json:"kind" yaml:"kind"`
	Handle      string    `json:"handle" yaml:"handle"`
	DisplayName string    `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	Scopes      []Scope   `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	Role        Role      `json:"role,omitempty" yaml:"role,omitempty"`
	TokenID     string    `json:"token_id,omitempty" yaml:"token_id,omitempty"`
	ProjectID   string    `json:"project_id,omitempty" yaml:"project_id,omitempty"`
}

// HasScope reports whether the actor holds the scope, directly or by role.
func (a *Actor) HasScope(s Scope) bool {
	if a == nil {
		return false
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
	TenantID  string    `json:"tenant_id" yaml:"tenant_id"`
	ActorID   string    `json:"actor_id" yaml:"actor_id"`
	Role      Role      `json:"role" yaml:"role"`
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
}
