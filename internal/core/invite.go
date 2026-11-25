// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"context"
	"strings"
	"time"
)

// Invite expiry bounds. A link lives a week unless told otherwise, and never
// longer than a month: an unused credential lying in somebody's inbox is a
// liability that grows with its age.
const (
	DefaultInviteExpiry = 7 * 24 * time.Hour
	MinInviteExpiry     = time.Hour
	MaxInviteExpiry     = 30 * 24 * time.Hour
)

// InvitePathPrefix is the browser route an invite link opens, followed by the token.
const InvitePathPrefix = "/invite/"

// InvitePath is the browser route an invite token is accepted at.
func InvitePath(token string) string { return InvitePathPrefix + token }

// Invite is a pending or finished invitation to sign in to this tenant. The
// secret that accepts it is never part of it: only IssuedInvite carries that,
// and only on the call that minted it.
type Invite struct {
	// ID identifies the invite, for listing, revoking and re-issuing it.
	ID string `json:"id" yaml:"id"`
	// TenantID is the tenant the invite admits to.
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	// UserID is the invited account, created when the invite was issued.
	UserID string `json:"user_id" yaml:"user_id"`
	// Email is the invited address.
	Email string `json:"email" yaml:"email"`
	// Role is the role the invite granted in this tenant.
	Role Role `json:"role" yaml:"role"`
	// InvitedBy is the actor who issued the invite.
	InvitedBy string `json:"invited_by" yaml:"invited_by"`
	// InvitedByHandle is that actor's handle, empty when it no longer exists.
	InvitedByHandle string `json:"invited_by_handle,omitempty" yaml:"invited_by_handle,omitempty"`
	// ExistingAccount reports that the account already had a password when it
	// was invited, so accepting confirms that password instead of setting one.
	ExistingAccount bool `json:"existing_account" yaml:"existing_account"`
	// GrantsMembership reports that the invited account belongs to another
	// tenant and holds no membership here until its owner accepts.
	GrantsMembership bool `json:"grants_membership" yaml:"grants_membership"`
	// CreatedAt is when the invite was issued, RFC 3339.
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	// ExpiresAt is when the link stops working, RFC 3339.
	ExpiresAt time.Time `json:"expires_at" yaml:"expires_at"`
	// AcceptedAt is when the link was used, RFC 3339; absent while pending.
	AcceptedAt *time.Time `json:"accepted_at,omitempty" yaml:"accepted_at,omitempty"`
	// RevokedAt is when the link was revoked or replaced, RFC 3339; absent while pending.
	RevokedAt *time.Time `json:"revoked_at,omitempty" yaml:"revoked_at,omitempty"`
}

// Pending reports whether the invite can still be accepted at now.
func (i Invite) Pending(now time.Time) bool {
	return i.AcceptedAt == nil && i.RevokedAt == nil && now.Before(i.ExpiresAt)
}

// IssuedInvite carries an invite together with its freshly minted secret. Path
// is the browser route that accepts it; URL is that route made absolute, under
// server.public_url (or the tenant's own domain) or the address a request
// arrived at, and is empty when neither is known.
type IssuedInvite struct {
	Invite
	// Token is the secret that accepts the invite, returned only by the call
	// that minted it.
	Token string `json:"token" yaml:"token"`
	// Path is the browser route the invited person opens: /invite/TOKEN.
	Path string `json:"path" yaml:"path"`
	// URL is Path made absolute, empty when no web address is known.
	URL string `json:"url,omitempty" yaml:"url,omitempty"`
}

// CreateInviteInput invites an email address to this tenant.
type CreateInviteInput struct {
	// Email is the address to invite, lowercased. Required.
	Email string `json:"email" yaml:"email"`
	// DisplayName is the name shown for a new account until its owner
	// chooses one.
	DisplayName string `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	// Handle is a new account's short name; empty uses the part of the email
	// before the "@".
	Handle string `json:"handle,omitempty" yaml:"handle,omitempty"`
	// Role is the role granted in this tenant; empty means "viewer".
	Role Role `json:"role,omitempty" yaml:"role,omitempty"`
	// ExpiresIn is how long the link works, such as "48h" or "7d", between
	// one hour and 30 days; omitted means 7 days.
	ExpiresIn Duration `json:"expires_in,omitempty" yaml:"expires_in,omitempty"`
}

// Validate checks the input.
func (in CreateInviteInput) Validate() error {
	if strings.TrimSpace(in.Email) == "" {
		return Invalid("email is required").WithDetail(DetailField, "email")
	}
	if !strings.Contains(in.Email, "@") {
		return Invalid("email %q is not an address", in.Email).WithDetail(DetailField, "email")
	}
	if in.Role != "" && !in.Role.Valid() {
		return Invalid("role %q is not recognised", in.Role).WithDetail(DetailField, "role")
	}
	_, err := InviteExpiry(in.ExpiresIn)
	return err
}

// ReissueInviteInput replaces a pending invite's link.
type ReissueInviteInput struct {
	// ExpiresIn is how long the new link works, between one hour and 30
	// days; omitted means 7 days.
	ExpiresIn Duration `json:"expires_in,omitempty" yaml:"expires_in,omitempty"`
}

// InviteExpiry resolves a requested lifetime, refusing one out of bounds.
func InviteExpiry(d Duration) (time.Duration, error) {
	switch {
	case d == 0:
		return DefaultInviteExpiry, nil
	case d.D() < MinInviteExpiry || d.D() > MaxInviteExpiry:
		return 0, Invalid("an invite expires between 1h and 30d after it is issued, not %s", d).
			WithDetail(DetailField, "expires_in")
	default:
		return d.D(), nil
	}
}

// InvitePreview is what the person holding a live invite link is shown before
// accepting it. It names the account and nothing about who else exists.
type InvitePreview struct {
	// Email is the invited address.
	Email string `json:"email" yaml:"email"`
	// DisplayName is the account's current display name.
	DisplayName string `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	// TenantName is the tenant the invite admits to.
	TenantName string `json:"tenant_name" yaml:"tenant_name"`
	// ExistingAccount reports that the account already has a password, which
	// accepting confirms rather than replaces.
	ExistingAccount bool `json:"existing_account" yaml:"existing_account"`
	// CodeRequired reports that accepting needs a code from the account's
	// second factor: the account belongs to another tenant and holds one.
	CodeRequired bool `json:"code_required,omitempty" yaml:"code_required,omitempty"`
	// ExpiresAt is when the link stops working, RFC 3339.
	ExpiresAt time.Time `json:"expires_at" yaml:"expires_at"`
}

// AcceptInviteInput accepts an invite.
type AcceptInviteInput struct {
	// Password is the new password, at least 12 characters, or for an
	// existing account its current password.
	Password string `json:"password" yaml:"password"`
	// DisplayName replaces the account's display name when set; ignored for
	// an existing account.
	DisplayName string `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	// Surface is the sign-in path the session is for, which decides whether a
	// second factor is required; omitted means the command line.
	Surface Surface `json:"surface,omitempty" yaml:"surface,omitempty"`
	// Code is a code from the account's second factor, required when the
	// preview says so: accepting an invite to an account of another tenant
	// asks for everything signing in to it would.
	Code string `json:"code,omitempty" yaml:"code,omitempty"`
}

// InviteService issues, lists and accepts invite links.
type InviteService interface {
	// CreateInvite invites an email address to this tenant and returns a
	// single-use link, once. A new address gets an account with no password;
	// an account of this tenant without a membership is granted one. An
	// account of another tenant is invited, and gains the membership only when
	// its owner accepts with their own password and second factor. An address
	// of a member who has a password, of an account with a pending invite, or
	// of another tenant's account that has no password yet is a conflict.
	CreateInvite(ctx context.Context, in CreateInviteInput) (*IssuedInvite, error)
	// ListInvites lists this tenant's invites that are neither accepted nor
	// revoked, expired ones included, oldest first. No secret is returned.
	ListInvites(ctx context.Context) ([]Invite, error)
	// RevokeInvite stops a pending invite's link at once. The account it
	// created stays, without a password.
	RevokeInvite(ctx context.Context, id string) error
	// ReissueInvite replaces a pending or expired invite's link with a new one
	// carrying a fresh expiry, and returns it once. The old link stops.
	ReissueInvite(ctx context.Context, id string, in ReissueInviteInput) (*IssuedInvite, error)
	// PreviewInvite describes the invite an invite token accepts, without a
	// credential. An unknown, expired, used or revoked token is the same not
	// found.
	PreviewInvite(ctx context.Context, token string) (*InvitePreview, error)
	// AcceptInvite sets the invited account's password and display name, or
	// confirms an existing account's password, marks the invite used and
	// signs the person in. It answers as Login does: a session, or the second
	// factor challenge the tenant requires.
	AcceptInvite(ctx context.Context, token string, in AcceptInviteInput) (*LoginResult, error)
}
