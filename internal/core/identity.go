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

	// ScopeAccountRead lets a token read its owner's account: the log of
	// changes to it, which names every tenant the person belongs to. "*" does
	// not imply it, so an agent's token in one tenant learns nothing about the
	// others; it is carried only when named, and only by a token minted for
	// its owner's own actor.
	ScopeAccountRead Scope = "account:read"
	ScopeExport      Scope = "export"
	ScopeImport      Scope = "import"
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
	ScopeGlobalAdmin, ScopeAccountRead,
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
	// TokenName is the name of that token, kept for the access channel an
	// audit entry records. It is never serialised.
	TokenName string `json:"-" yaml:"-"`
	// SessionID is the stored browser session the request authenticated
	// with, never the cookie value. It is never serialised.
	SessionID string `json:"-" yaml:"-"`
	// ProjectID restricts the actor to one project when set, as a
	// project-scoped token does.
	ProjectID string `json:"project_id,omitempty" yaml:"project_id,omitempty"`
	// GlobalAdmin reports, on the answer to WhoAmI, that the actor may act as a
	// global admin right now. It is a report for interfaces deciding what to
	// offer; authorization never reads it, and asks the store instead.
	GlobalAdmin bool `json:"global_admin,omitempty" yaml:"global_admin,omitempty"`
	// UserID is the account a user actor belongs to, as the store holds it.
	// It is never serialised, and it is not set on the actor a credential
	// resolves to: the account is found through the store, never assumed to
	// share the actor's identifier.
	UserID string `json:"-" yaml:"-"`
	// DisabledAt is when the actor was disabled in its tenant, as the store
	// holds it. A disabled actor's credentials stop working in that tenant
	// alone; the account and its other tenants are untouched. Never
	// serialised.
	DisabledAt *time.Time `json:"-" yaml:"-"`
	// AccountID is the account behind a user actor, on the answer to WhoAmI.
	AccountID string `json:"account_id,omitempty" yaml:"account_id,omitempty"`
	// Email is that account's sign-in address, on the answer to WhoAmI.
	Email string `json:"email,omitempty" yaml:"email,omitempty"`
	// TenantKey and TenantName name the actor's tenant, on the answer to
	// WhoAmI.
	TenantKey  string `json:"tenant_key,omitempty" yaml:"tenant_key,omitempty"`
	TenantName string `json:"tenant_name,omitempty" yaml:"tenant_name,omitempty"`
	// Theme is the palette the actor's tenant resolves to, as the browser
	// renders it, on the answer to WhoAmI.
	Theme *Theme `json:"theme,omitempty" yaml:"theme,omitempty"`
	// Memberships are every tenant the account is a member of, on the answer
	// to WhoAmI for a session or a token carrying account:read.
	Memberships []AccountMembership `json:"memberships,omitempty" yaml:"memberships,omitempty"`
}

// HasScope reports whether the actor holds the scope, directly or by role.
func (a *Actor) HasScope(s Scope) bool {
	if a == nil {
		return false
	}
	if s == ScopeGlobalAdmin || s == ScopeAccountRead {
		return slices.Contains(a.Scopes, s)
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

// AccountMembership is one tenant an account belongs to: its actor there and
// the role its membership grants.
type AccountMembership struct {
	// TenantID is the tenant.
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	// TenantKey is the tenant's key.
	TenantKey string `json:"tenant_key" yaml:"tenant_key"`
	// TenantName is the tenant's name.
	TenantName string `json:"tenant_name" yaml:"tenant_name"`
	// ActorID is the account's actor in the tenant.
	ActorID string `json:"actor_id" yaml:"actor_id"`
	// Handle is that actor's handle.
	Handle string `json:"handle" yaml:"handle"`
	// Role is the role the membership grants.
	Role Role `json:"role" yaml:"role"`
	// Host is the tenant's own address, a domain not marked shared; empty when
	// it is served only on a shared host.
	Host string `json:"host" yaml:"host"`
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

// AccountAction names a change recorded in an account's log.
type AccountAction string

// Account actions.
const (
	AccountCreated            AccountAction = "account.created"
	AccountDeleted            AccountAction = "account.deleted"
	AccountPasswordSet        AccountAction = "account.password_set"
	AccountPasswordChangeSet  AccountAction = "account.password_change_required"
	AccountPasswordChangeLift AccountAction = "account.password_change_lifted"
	AccountDisabled           AccountAction = "account.disabled"
	AccountEnabled            AccountAction = "account.enabled"
	AccountFactorEnrolled     AccountAction = "account.second_factor_enrolled"
	AccountFactorCleared      AccountAction = "account.second_factor_cleared"
	AccountRecoveryIssued     AccountAction = "account.recovery_codes_issued"
	AccountRecoveryUsed       AccountAction = "account.recovery_code_used"
	AccountGlobalAdminGranted AccountAction = "account.global_admin_granted"
	AccountGlobalAdminRevoked AccountAction = "account.global_admin_revoked"
	AccountMemberAdded        AccountAction = "account.membership_added"
	AccountMemberRole         AccountAction = "account.membership_role_changed"
	AccountMemberRemoved      AccountAction = "account.membership_removed"
)

// AccountLogEntry is one change to an account, recorded in the same
// transaction as the change. It holds no secret: no password, hash, code or
// token, only what changed, where, by whom and through which channel.
type AccountLogEntry struct {
	// Seq orders the account's entries; a later change has a larger number.
	Seq int64 `json:"seq" yaml:"seq"`
	// UserID is the account the entry is about.
	UserID string `json:"user_id" yaml:"user_id"`
	// Action is what changed.
	Action AccountAction `json:"action" yaml:"action"`
	// TenantKey is the tenant the change was made from, or that a membership
	// change concerns.
	TenantKey string `json:"tenant_key,omitempty" yaml:"tenant_key,omitempty"`
	// ByUserID is the account of whoever made the change; empty for the
	// system or a shell holding the database.
	ByUserID string `json:"by_user_id,omitempty" yaml:"by_user_id,omitempty"`
	// ByEmail is that account's address when the change was made.
	ByEmail string `json:"by_email,omitempty" yaml:"by_email,omitempty"`
	// ByGlobalAdmin reports that a global admin made the change as one.
	ByGlobalAdmin bool `json:"by_global_admin,omitempty" yaml:"by_global_admin,omitempty"`
	// Via is the access channel the change arrived through.
	Via ViaKind `json:"via" yaml:"via"`
	// Detail carries what changed, such as a role; never a secret.
	Detail map[string]any `json:"detail,omitempty" yaml:"detail,omitempty"`
	// OccurredAt is when the change was made, RFC 3339.
	OccurredAt time.Time `json:"occurred_at" yaml:"occurred_at"`
}
