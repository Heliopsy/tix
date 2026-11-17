// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"context"
	"strings"
	"time"
)

// SCIMPath is where tix serve answers SCIM 2.0 provisioning.
const SCIMPath = "/scim/v2"

// SCIMAgentHandle is the handle of the agent a tenant's provisioning token
// acts as when SCIM is enabled through tix rather than with tix token create.
const SCIMAgentHandle = "scim"

// SCIMTokenName is the name of the token enabling SCIM mints for that agent.
const SCIMTokenName = "scim" // #nosec G101 -- a token's label, not a credential

// SCIMService administers a tenant's SCIM provisioning: the switch, the
// provisioning token and the group to role mappings.
type SCIMService interface {
	// GetSCIM reports whether SCIM provisioning is on for this tenant, its
	// endpoint, the provisioning agent's live tokens, how many users and groups
	// a provider has pushed, when it last changed anything, and the group to
	// role mappings.
	GetSCIM(ctx context.Context) (*SCIMSettings, error)
	// EnableSCIM turns SCIM provisioning on. The first time, it creates the
	// tenant's provisioning agent, handle scim; whenever that agent holds no
	// live token it mints one named scim, carrying scim:provision, whose value
	// the answer carries once.
	EnableSCIM(ctx context.Context) (*SCIMSettings, error)
	// DisableSCIM turns SCIM provisioning off: every SCIM endpoint answers 404
	// until it is enabled again. The provisioning token is kept.
	DisableSCIM(ctx context.Context) (*SCIMSettings, error)
	// RotateSCIMToken revokes the provisioning agent's live tokens and mints a
	// new one, whose value the answer carries once. Configure the identity
	// provider with it; the old token stops at once.
	RotateSCIMToken(ctx context.Context) (*SCIMSettings, error)
	// MapSCIMGroup gives the members of the SCIM group with this display name,
	// ignoring case, a tenant role. A person provisioned over SCIM holds the
	// highest role mapped to their groups. Mapping a name again changes its role.
	MapSCIMGroup(ctx context.Context, in SCIMGroupRoleInput) (*SCIMGroupRole, error)
	// UnmapSCIMGroup removes a group's role mapping. Provisioned members left
	// in no mapped group drop to viewer.
	UnmapSCIMGroup(ctx context.Context, group string) error
}

// SCIMSettings is a tenant's SCIM provisioning as an administrator sees it.
type SCIMSettings struct {
	// Enabled reports whether the SCIM endpoints answer for this tenant.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Path is where the endpoints are served, /scim/v2.
	Path string `json:"path" yaml:"path"`
	// URL is the absolute endpoint to give an identity provider, when the
	// surface answering knows the address it was reached at.
	URL string `json:"url,omitempty" yaml:"url,omitempty"`
	// AgentID is the provisioning agent's actor, once SCIM has been enabled.
	AgentID string `json:"agent_id,omitempty" yaml:"agent_id,omitempty"`
	// Tokens are the provisioning agent's live tokens, as metadata.
	Tokens []APIToken `json:"tokens" yaml:"tokens"`
	// Token is a newly minted token's value, present once, on the answer to
	// the call that minted it.
	Token string `json:"token,omitempty" yaml:"token,omitempty"`
	// Users and Groups count what a provider has pushed.
	Users  int `json:"users" yaml:"users"`
	Groups int `json:"groups" yaml:"groups"`
	// LastActivityAt is when a provider last changed anything, RFC 3339.
	LastActivityAt *time.Time `json:"last_activity_at,omitempty" yaml:"last_activity_at,omitempty"`
	// GroupRoles are the group to role mappings, by group name.
	GroupRoles []SCIMGroupRole `json:"group_roles" yaml:"group_roles"`
}

// SCIMGroupRole maps a SCIM group, by display name, to a tenant role.
type SCIMGroupRole struct {
	// Group is the group's display name; it matches ignoring case.
	Group string `json:"group" yaml:"group"`
	// Role is the role the group's provisioned members are given.
	Role Role `json:"role" yaml:"role"`
	// CreatedAt is when the mapping was made, RFC 3339.
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
}

// SCIMGroupRoleInput maps a SCIM group to a role.
type SCIMGroupRoleInput struct {
	// Group is the SCIM group's display name. Required.
	Group string `json:"group" yaml:"group"`
	// Role is viewer, member or admin. Required.
	Role Role `json:"role" yaml:"role"`
}

// MaxSCIMName bounds a SCIM display name, userName or externalId.
const MaxSCIMName = 256

// Validate checks the input.
func (in SCIMGroupRoleInput) Validate() error {
	name := strings.TrimSpace(in.Group)
	switch {
	case name == "":
		return Invalid("a group name is required").WithDetail(DetailField, "group")
	case len(name) > MaxSCIMName:
		return Invalid("a group name is at most %d bytes", MaxSCIMName).WithDetail(DetailField, "group")
	case !in.Role.Valid():
		return Invalid("role %q must be %q, %q or %q", in.Role, RoleViewer, RoleMember, RoleAdmin).
			WithDetail(DetailField, "role")
	}
	return nil
}

// SCIMKey is the form a SCIM name is compared in: trimmed and lower-cased.
func SCIMKey(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

// SCIMUser is a member of the tenant as SCIM provisions them.
type SCIMUser struct {
	// ID is the person's user id in this tenant.
	ID string
	// ExternalID is the provider's own identifier.
	ExternalID string
	// UserName is the provider's unique name for the person.
	UserName string
	// Email is the address the provider sent; at creation it is the account's.
	Email string
	// GivenName and FamilyName are the parts of the person's name.
	GivenName  string
	FamilyName string
	// DisplayName is the person's display name in this tenant.
	DisplayName string
	// Active is false while the person is disabled in this tenant.
	Active bool
	// Groups are the SCIM groups the person belongs to.
	Groups []SCIMRef
	// Created and Modified are when the provisioning record was written.
	Created, Modified time.Time
}

// SCIMGroup is a tenant group a provider pushed.
type SCIMGroup struct {
	ID          string
	ExternalID  string
	DisplayName string
	// Members are the group's people, each a user of the tenant.
	Members           []SCIMRef
	Created, Modified time.Time
}

// SCIMRef names one resource from another: a member of a group or a group of
// a user.
type SCIMRef struct {
	ID      string
	Display string
}

// SCIM attributes a filter can name.
const (
	SCIMAttrID          = "id"
	SCIMAttrUserName    = "userName"
	SCIMAttrExternalID  = "externalId"
	SCIMAttrEmail       = "email"
	SCIMAttrDisplayName = "displayName"
	SCIMAttrMember      = "member"
)

// SCIMCondition is one equality a SCIM filter asks for.
type SCIMCondition struct {
	// Attr is one of the SCIMAttr names.
	Attr string
	// Value is compared ignoring case for names and addresses, exactly for
	// identifiers.
	Value string
}

// SCIMQuery selects SCIM resources: every condition must hold, and the
// matches are paged by a one-based start index and a count.
type SCIMQuery struct {
	Conditions []SCIMCondition
	StartIndex int
	Count      int
	// WithoutMembers leaves a group's members out of the answer.
	WithoutMembers bool
}

// Provisioner is what the SCIM endpoints call. It is not part of Service:
// its operations are the SCIM protocol, reached through /scim/v2 with a
// scim:provision token and through nothing else.
type Provisioner interface {
	// SCIMReady refuses a caller that may not provision, and answers not
	// found while SCIM is off for the tenant.
	SCIMReady(ctx context.Context) error
	SCIMListUsers(ctx context.Context, q SCIMQuery) ([]SCIMUser, int, error)
	SCIMGetUser(ctx context.Context, id string) (*SCIMUser, error)
	// SCIMCreateUser links the account with the user's address, or creates
	// one with no password.
	SCIMCreateUser(ctx context.Context, in SCIMUser) (*SCIMUser, error)
	// SCIMUpdateUser applies a change to a user inside one transaction.
	SCIMUpdateUser(ctx context.Context, id string, apply func(*SCIMUser) error) (*SCIMUser, error)
	// SCIMDeleteUser removes the person's membership, never their history.
	SCIMDeleteUser(ctx context.Context, id string) error
	SCIMListGroups(ctx context.Context, q SCIMQuery) ([]SCIMGroup, int, error)
	SCIMGetGroup(ctx context.Context, id string, members bool) (*SCIMGroup, error)
	SCIMCreateGroup(ctx context.Context, in SCIMGroup) (*SCIMGroup, error)
	// SCIMUpdateGroup applies a change to a group inside one transaction.
	SCIMUpdateGroup(ctx context.Context, id string, apply func(*SCIMGroup) error) (*SCIMGroup, error)
	SCIMDeleteGroup(ctx context.Context, id string) error
}

type scimRequestKey struct{}

// WithSCIMRequest marks a request that arrived under SCIMPath. Only the HTTP
// layer sets it, from the path; a provisioning token is refused anywhere it
// is not set.
func WithSCIMRequest(ctx context.Context) context.Context {
	return context.WithValue(ctx, scimRequestKey{}, true)
}

// SCIMRequestFrom reports whether ctx was marked by WithSCIMRequest.
func SCIMRequestFrom(ctx context.Context) bool {
	v, _ := ctx.Value(scimRequestKey{}).(bool)
	return v
}
