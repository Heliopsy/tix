// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
	"github.com/heliopsy/tix/internal/store"
)

// Events and audit actions of invite links. No payload or snapshot carries the
// secret or its hash: core.Invite has no field for either.
const (
	eventInviteCreated  core.EventType = "user_invite.created"
	eventInviteReissued core.EventType = "user_invite.reissued"
	eventInviteRevoked  core.EventType = "user_invite.revoked"
	eventInviteAccepted core.EventType = "user_invite.accepted"

	auditInviteCreate  = "user_invite.create"
	auditInviteReissue = "user_invite.reissue"
	auditInviteRevoke  = "user_invite.revoke"
	auditInviteAccept  = "user_invite.accept"

	subjectInvite = "user_invite"
)

// invitee is the account an address resolves to before an invite is written.
type invitee struct {
	user *core.User
	hash string
}

// CreateInvite invites an address and returns its link, once.
func (l *Local) CreateInvite(ctx context.Context, in core.CreateInviteInput) (*core.IssuedInvite, error) {
	actor, err := l.authorize(ctx, authz.ActionUserAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	ttl, err := core.InviteExpiry(in.ExpiresIn)
	if err != nil {
		return nil, err
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	found, err := l.inviteeByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	var out *core.IssuedInvite
	err = l.write(ctx, actor, func(m *mutation) error {
		userID, existing, role, err := l.prepareInvitee(ctx, m, found, email, in)
		if err != nil {
			return err
		}
		inv := core.Invite{
			ID: id.NewAt(m.now), UserID: userID, Email: email, Role: role,
			InvitedBy: actor.ID, InvitedByHandle: actor.Handle, ExistingAccount: existing,
			CreatedAt: m.now, ExpiresAt: m.now.Add(ttl),
		}
		if out, err = issueInvite(ctx, m, inv); err != nil {
			return err
		}
		return m.Record(auditInviteCreate, eventInviteCreated, subjectInvite, inv.ID, "", nil, out.Invite,
			inviteTrail(out.Invite))
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// inviteeByEmail looks an address up across the installation, because a user
// row is global and its email unique everywhere.
func (l *Local) inviteeByEmail(ctx context.Context, email string) (invitee, error) {
	var out invitee
	err := l.store.ViewUnscoped(ctx, func(u store.UnscopedTx) error {
		user, hash, err := u.GetUserByEmail(ctx, email)
		switch {
		case err == nil:
			out = invitee{user: user, hash: hash}
			return nil
		case core.IsKind(err, core.KindNotFound):
			return nil
		default:
			return err
		}
	})
	return out, err
}

// prepareInvitee creates the invited account, or grants an existing account of
// this tenant the membership it lacks, and reports which.
func (l *Local) prepareInvitee(ctx context.Context, m *mutation, found invitee, email string,
	in core.CreateInviteInput,
) (string, bool, core.Role, error) {
	if found.user == nil {
		role := in.Role
		if role == "" {
			role = defaultUserRole
		}
		userID, err := l.createInvitedUser(ctx, m, email, in, role)
		return userID, false, role, err
	}
	userID := found.user.ID
	if _, err := m.tx.GetActor(ctx, userID); err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return "", false, "", core.Conflict("%s already belongs to a user, and a user belongs to one tenant; "+
				"invite another address", email).WithDetail(core.DetailField, "email")
		}
		return "", false, "", err
	}
	current, err := userRole(ctx, m.tx, userID)
	if err != nil {
		return "", false, "", err
	}
	existing := found.hash != ""
	if existing && current != "" {
		return "", false, "", core.Conflict("%s is already a member of this tenant and can sign in", email).
			WithDetail(core.DetailField, "email")
	}
	switch _, err := m.tx.PendingInviteFor(ctx, userID); {
	case err == nil:
		return "", false, "", core.Conflict("%s already has a pending invite; re-issue it for a new link", email).
			WithDetail(core.DetailField, "email")
	case !core.IsKind(err, core.KindNotFound):
		return "", false, "", err
	}
	role := in.Role
	switch {
	case role == "" && current != "":
		role = current
	case role == "":
		role = defaultUserRole
	}
	if role != current {
		if err := setUserRole(ctx, m.tx, userID, role); err != nil {
			return "", false, "", err
		}
	}
	return userID, existing, role, nil
}

// createInvitedUser writes a new account with no password, its actor and its
// membership, recorded as any other user creation is.
func (l *Local) createInvitedUser(ctx context.Context, m *mutation, email string, in core.CreateInviteInput,
	role core.Role,
) (string, error) {
	handle := userHandle(in.Handle, email)
	if handle == "" {
		return "", core.Invalid("a user needs a handle").WithDetail(core.DetailField, "handle")
	}
	switch _, err := m.tx.GetActorByHandle(ctx, handle); {
	case err == nil:
		return "", core.Conflict("handle %q is already in use; pass another handle", handle).
			WithDetail(core.DetailField, "handle")
	case !core.IsKind(err, core.KindNotFound):
		return "", err
	}
	display := strings.TrimSpace(in.DisplayName)
	userID := l.ids.New()
	u := &core.User{ID: userID, Email: email, DisplayName: display}
	if err := m.tx.CreateUser(ctx, u, ""); err != nil {
		return "", err
	}
	if err := m.tx.CreateActor(ctx, &core.Actor{ID: userID, Kind: core.ActorUser, Handle: handle, DisplayName: display}); err != nil {
		return "", err
	}
	if err := setUserRole(ctx, m.tx, userID, role); err != nil {
		return "", err
	}
	return userID, m.Record(auditUserCreate, eventUserCreated, "user", userID, "", nil, u,
		map[string]any{"email": email, "handle": handle, "role": string(role), "invited": true})
}

// issueInvite mints a secret and stores the invite behind its hash.
func issueInvite(ctx context.Context, m *mutation, inv core.Invite) (*core.IssuedInvite, error) {
	token, hash, err := auth.MintInviteToken()
	if err != nil {
		return nil, err
	}
	if err := m.tx.CreateInvite(ctx, &inv, hash); err != nil {
		return nil, err
	}
	return &core.IssuedInvite{Invite: inv, Token: token, Path: core.InvitePath(token)}, nil
}

// inviteTrail is what an invite's audit entry and event say about it.
func inviteTrail(inv core.Invite) map[string]any {
	return map[string]any{
		"invite_id": inv.ID, "user_id": inv.UserID, "email": inv.Email,
		"role": string(inv.Role), "expires_at": inv.ExpiresAt,
	}
}

// ListInvites lists the invites neither accepted nor revoked, oldest first.
func (l *Local) ListInvites(ctx context.Context) ([]core.Invite, error) {
	actor, err := l.authorize(ctx, authz.ActionUserAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out []core.Invite
	err = l.read(ctx, actor, func(tx store.Tx) error {
		if out, err = tx.ListPendingInvites(ctx); err != nil {
			return err
		}
		handles := map[string]string{}
		for i := range out {
			by := out[i].InvitedBy
			if _, ok := handles[by]; !ok {
				handles[by] = ""
				if a, err := tx.GetActor(ctx, by); err == nil {
					handles[by] = a.Handle
				} else if !core.IsKind(err, core.KindNotFound) {
					return err
				}
			}
			out[i].InvitedByHandle = handles[by]
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// openInvite returns an invite an administrator may still revoke or re-issue.
func openInvite(ctx context.Context, tx store.Tx, id string) (*core.Invite, error) {
	if strings.TrimSpace(id) == "" {
		return nil, core.Invalid("invite identifier is required")
	}
	inv, err := tx.GetInvite(ctx, id)
	if err != nil {
		return nil, err
	}
	switch {
	case inv.AcceptedAt != nil:
		return nil, core.Conflict("invite %s was already accepted", id)
	case inv.RevokedAt != nil:
		return nil, core.NotFound("invite %s was revoked", id)
	}
	return inv, nil
}

// RevokeInvite stops a pending invite's link. The account stays.
func (l *Local) RevokeInvite(ctx context.Context, id string) error {
	actor, err := l.authorize(ctx, authz.ActionUserAdmin, authz.Resource{})
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		inv, err := openInvite(ctx, m.tx, id)
		if err != nil {
			return err
		}
		if err := m.tx.RevokeInvite(ctx, inv.ID, m.now); err != nil {
			return err
		}
		after := *inv
		after.RevokedAt = &m.now
		return m.Record(auditInviteRevoke, eventInviteRevoked, subjectInvite, inv.ID, "", *inv, after, inviteTrail(*inv))
	})
}

// ReissueInvite replaces an invite's link with a new one and a fresh expiry.
func (l *Local) ReissueInvite(ctx context.Context, inviteID string, in core.ReissueInviteInput) (*core.IssuedInvite, error) {
	actor, err := l.authorize(ctx, authz.ActionUserAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	ttl, err := core.InviteExpiry(in.ExpiresIn)
	if err != nil {
		return nil, err
	}
	var out *core.IssuedInvite
	err = l.write(ctx, actor, func(m *mutation) error {
		old, err := openInvite(ctx, m.tx, inviteID)
		if err != nil {
			return err
		}
		if err := m.tx.RevokeInvite(ctx, old.ID, m.now); err != nil {
			return err
		}
		next := *old
		next.ID, next.InvitedBy, next.InvitedByHandle = id.NewAt(m.now), actor.ID, actor.Handle
		next.CreatedAt, next.ExpiresAt = m.now, m.now.Add(ttl)
		if out, err = issueInvite(ctx, m, next); err != nil {
			return err
		}
		trail := inviteTrail(out.Invite)
		trail["replaced"] = old.ID
		return m.Record(auditInviteReissue, eventInviteReissued, subjectInvite, out.ID, "", *old, out.Invite, trail)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// liveInvite is everything a usable invite token resolved to.
type liveInvite struct {
	invite *core.Invite
	user   *core.User
	hash   string
	actor  *core.Actor
	role   core.Role
	tenant *core.Tenant
}

// resolveInvite turns a presented token into the invite it accepts. Every way
// it can fail is auth.ErrNoInvite, so a link tells its holder nothing about
// any other link or account.
func (l *Local) resolveInvite(ctx context.Context, token string) (liveInvite, error) {
	var out liveInvite
	scope, err := core.RequireTenant(ctx)
	if err != nil {
		return out, err
	}
	if err := auth.ParseInviteToken(token); err != nil {
		return out, err
	}
	presented := auth.HashToken(token)
	err = l.read(ctx, core.SystemActor(scope.TenantID), func(tx store.Tx) error {
		inv, stored, err := tx.InviteByHash(ctx, presented)
		if err != nil {
			return err
		}
		if !auth.EqualHash(stored, presented) || !inv.Pending(l.clock.Now()) {
			return auth.ErrNoInvite()
		}
		if out.user, err = tx.GetUser(ctx, inv.UserID); err != nil {
			return err
		}
		if out.actor, err = tx.GetActor(ctx, inv.UserID); err != nil {
			return err
		}
		if out.role, err = userRole(ctx, tx, inv.UserID); err != nil {
			return err
		}
		if out.tenant, err = tx.GetTenant(ctx); err != nil {
			return err
		}
		out.invite = inv
		return nil
	})
	if core.IsKind(err, core.KindNotFound) {
		return out, auth.ErrNoInvite()
	}
	if err != nil {
		return out, err
	}
	if out.user.DisabledAt != nil || out.role == "" {
		return out, auth.ErrNoInvite()
	}
	found, err := l.inviteeByEmail(ctx, out.user.Email)
	if err != nil {
		return out, err
	}
	out.hash = found.hash
	return out, nil
}

// PreviewInvite describes the invite a token accepts.
func (l *Local) PreviewInvite(ctx context.Context, token string) (*core.InvitePreview, error) {
	live, err := l.resolveInvite(ctx, token)
	if err != nil {
		return nil, err
	}
	return &core.InvitePreview{
		Email: live.user.Email, DisplayName: live.user.DisplayName, TenantName: live.tenant.Name,
		ExistingAccount: live.hash != "", ExpiresAt: live.invite.ExpiresAt,
	}, nil
}

// AcceptInvite sets or confirms the invited account's password, uses up the
// link and signs the person in.
func (l *Local) AcceptInvite(ctx context.Context, token string, in core.AcceptInviteInput) (*core.LoginResult, error) {
	surface := in.Surface
	if surface == "" {
		surface = core.SurfaceCLI
	}
	if !surface.Valid() {
		return nil, core.Invalid("%q is not a sign-in surface", in.Surface)
	}
	live, err := l.resolveInvite(ctx, token)
	if err != nil {
		return nil, err
	}
	hash, err := l.inviteePassword(live, in.Password)
	if err != nil {
		return nil, err
	}
	scope := core.TenantScope{TenantID: live.tenant.ID}
	cand := candidate{user: live.user, actor: live.actor, role: live.role, tenant: scope.TenantID}
	sessionActor := cand.sessionActor(scope)

	accept := func(m *mutation) error {
		if !live.invite.Pending(m.now) {
			return auth.ErrNoInvite()
		}
		if err := m.tx.AcceptInvite(ctx, live.invite.ID, m.now); err != nil {
			return auth.ErrNoInvite()
		}
		before := *live.user
		if hash != "" {
			if display := strings.TrimSpace(in.DisplayName); display != "" {
				live.user.DisplayName = display
			}
			live.user.RequirePasswordChange = false
			if err := m.tx.UpdateUser(ctx, live.user, hash); err != nil {
				return err
			}
			if err := m.Audit(auditUserUpdate, "user", live.user.ID, before, live.user); err != nil {
				return err
			}
		}
		after := *live.invite
		after.AcceptedAt = &m.now
		return m.Record(auditInviteAccept, eventInviteAccepted, subjectInvite, live.invite.ID, "",
			*live.invite, after, inviteTrail(after))
	}

	challenge, err := l.secondFactorFor(ctx, scope, surface, sessionActor, live.user, core.LoginMethodPassword)
	if err != nil {
		return nil, err
	}
	if challenge == nil {
		return l.mintSession(ctx, scope, sessionActor, live.user, "", core.LoginMethodPassword, accept)
	}
	if err := l.write(ctx, sessionActor, accept); err != nil {
		return nil, err
	}
	return &core.LoginResult{Challenge: challenge}, nil
}

// inviteePassword checks the password an acceptance carries. A new account's
// is hashed for storing; an existing account's must match the one it has and
// is kept, so the answer is "".
func (l *Local) inviteePassword(live liveInvite, password string) (string, error) {
	if live.hash != "" {
		if auth.Verify(live.hash, password) != nil {
			return "", errInvalidCredentials()
		}
		return "", nil
	}
	if err := auth.ValidatePassword(password); err != nil {
		return "", core.Invalid("password must be at least %d characters", core.MinPasswordLength).
			WithDetail(core.DetailField, "password")
	}
	return l.hasher.Hash(password)
}
