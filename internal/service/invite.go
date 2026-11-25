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

	// auditActorCreate records the actor an invite writes for an account of
	// another tenant, which holds no membership until accepted.
	auditActorCreate = "actor.create"
)

// invitee is the account an address resolves to before an invite is written.
type invitee struct {
	user *core.User
	hash string
	// elsewhere is whether the account holds a membership in another tenant.
	elsewhere bool
}

// invited is what preparing an invitee settled: the actor the invite names,
// whether the account already had a password, the role, and whether the
// membership waits for the person to accept.
type invited struct {
	actorID  string
	existing bool
	role     core.Role
	grants   bool
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
	if found.user != nil {
		if found.elsewhere, err = l.memberElsewhere(ctx, found.user.ID, actor.TenantID); err != nil {
			return nil, err
		}
	}

	var out *core.IssuedInvite
	err = l.write(ctx, actor, func(m *mutation) error {
		got, err := l.prepareInvitee(ctx, m, found, email, in)
		if err != nil {
			return err
		}
		inv := core.Invite{
			ID: id.NewAt(m.now), UserID: got.actorID, Email: email, Role: got.role,
			InvitedBy: actor.ID, InvitedByHandle: actor.Handle, ExistingAccount: got.existing,
			GrantsMembership: got.grants, CreatedAt: m.now, ExpiresAt: m.now.Add(ttl),
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
	l.linkInvite(ctx, out)
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

// prepareInvitee creates the invited account, or readies an existing one, and
// reports what it settled.
//
// An account that belongs to another tenant is never given a membership here
// by an administrator of this tenant: its actor is written so the invite has
// something to name, and the membership waits for its owner to accept with
// their own password and second factor. Such an account must already have a
// password, because accepting an invite to an account without one sets it, and
// the administrator holding the link would then hold the account in every
// tenant it belongs to.
func (l *Local) prepareInvitee(ctx context.Context, m *mutation, found invitee, email string,
	in core.CreateInviteInput,
) (invited, error) {
	role := in.Role
	if found.user == nil {
		if role == "" {
			role = defaultUserRole
		}
		actorID, err := l.createInvitedUser(ctx, m, email, in, role)
		return invited{actorID: actorID, role: role}, err
	}
	existing := found.hash != ""
	if found.elsewhere && !existing {
		return invited{}, core.Conflict("%s belongs to an account in another tenant that has no password yet, "+
			"so it cannot confirm an invite; ask a global admin to add it", email).
			WithDetail(core.DetailField, "email")
	}
	member, err := m.tx.GetActorByUser(ctx, found.user.ID)
	switch {
	case core.IsKind(err, core.KindNotFound) && found.elsewhere:
		if member, err = createInvitedActor(ctx, m, found.user, email, in.Handle); err != nil {
			return invited{}, err
		}
	case core.IsKind(err, core.KindNotFound):
		return invited{}, core.Conflict("%s belongs to an account with no tenant; ask a global admin to add it",
			email).WithDetail(core.DetailField, "email")
	case err != nil:
		return invited{}, err
	}
	current, err := userRole(ctx, m.tx, member.ID)
	if err != nil {
		return invited{}, err
	}
	if existing && current != "" {
		return invited{}, core.Conflict("%s is already a member of this tenant and can sign in", email).
			WithDetail(core.DetailField, "email")
	}
	switch _, err := m.tx.PendingInviteFor(ctx, member.ID); {
	case err == nil:
		return invited{}, core.Conflict("%s already has a pending invite; re-issue it for a new link", email).
			WithDetail(core.DetailField, "email")
	case !core.IsKind(err, core.KindNotFound):
		return invited{}, err
	}
	switch {
	case role == "" && current != "":
		role = current
	case role == "":
		role = defaultUserRole
	}
	if found.elsewhere {
		return invited{actorID: member.ID, existing: existing, role: role, grants: true}, nil
	}
	if role != current {
		if err := setUserRole(ctx, m.tx, member.ID, role); err != nil {
			return invited{}, err
		}
		if err := m.logAccount(ctx, found.user.ID, core.AccountMemberAdded,
			map[string]any{"role": string(role), "invited": true}); err != nil {
			return invited{}, err
		}
	}
	return invited{actorID: member.ID, existing: existing, role: role}, nil
}

// createInvitedActor writes the actor an account of another tenant will act
// as here, with no membership until its owner accepts.
func createInvitedActor(ctx context.Context, m *mutation, account *core.User, email, handle string,
) (*core.Actor, error) {
	actor, err := createMemberActor(ctx, m, account, email, handle)
	if err != nil {
		return nil, err
	}
	return actor, m.Audit(auditActorCreate, "actor", actor.ID, nil, map[string]any{
		"handle": actor.Handle, "invited": true,
	})
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
	actor := &core.Actor{ID: userID, UserID: userID, Kind: core.ActorUser, Handle: handle, DisplayName: display}
	if err := m.tx.CreateActor(ctx, actor); err != nil {
		return "", err
	}
	if err := setUserRole(ctx, m.tx, userID, role); err != nil {
		return "", err
	}
	if err := m.logAccount(ctx, userID, core.AccountCreated,
		map[string]any{"role": string(role), "invited": true}); err != nil {
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
	l.linkInvite(ctx, out)
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
		if out.actor, err = tx.GetActor(ctx, inv.UserID); err != nil {
			return err
		}
		if out.user, err = accountOf(ctx, tx, out.actor); err != nil {
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
	if out.role == "" && out.invite.GrantsMembership {
		out.role = out.invite.Role
	}
	if out.user.DisabledAt != nil || out.actor.DisabledAt != nil || out.role == "" {
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
	out := &core.InvitePreview{
		Email: live.user.Email, DisplayName: live.user.DisplayName, TenantName: live.tenant.Name,
		ExistingAccount: live.hash != "", ExpiresAt: live.invite.ExpiresAt,
	}
	if live.invite.GrantsMembership {
		enrolment, err := l.enrolmentOf(ctx, live)
		if err != nil {
			return nil, err
		}
		out.CodeRequired = enrolment.Confirmed()
	}
	return out, nil
}

// enrolmentOf reads the invited account's second factor, an empty one when it
// has none.
func (l *Local) enrolmentOf(ctx context.Context, live liveInvite) (*core.UserTOTP, error) {
	out := &core.UserTOTP{}
	err := l.read(ctx, core.SystemActor(live.tenant.ID), func(tx store.Tx) error {
		found, err := tx.GetUserTOTP(ctx, live.user.ID)
		switch {
		case core.IsKind(err, core.KindNotFound):
			return nil
		case err != nil:
			return err
		}
		out = found
		return nil
	})
	return out, err
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
	if live.invite.GrantsMembership {
		return l.acceptMembership(ctx, live, surface, in)
	}
	hash, err := l.inviteePassword(ctx, live, in.Password)
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
		before, beforeActor := *live.user, *live.actor
		if hash != "" {
			if display := strings.TrimSpace(in.DisplayName); display != "" {
				live.user.DisplayName = display
				live.actor.DisplayName = display
				if err := m.tx.UpdateActor(ctx, live.actor); err != nil {
					return err
				}
			}
			live.user.RequirePasswordChange = false
			if err := m.tx.UpdateUser(ctx, live.user, hash); err != nil {
				return err
			}
			err := m.Audit(auditUserUpdate, "user", live.actor.ID,
				asMember(&before, &beforeActor), asMember(live.user, live.actor))
			if err != nil {
				return err
			}
			if err := m.logAccount(ctx, live.user.ID, core.AccountPasswordSet,
				map[string]any{"invite": live.invite.ID}); err != nil {
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
		return l.mintSession(ctx, scope, sessionActor, live.user, "", passwordOrigin, accept)
	}
	if err := l.write(ctx, sessionActor, accept); err != nil {
		return nil, err
	}
	return &core.LoginResult{Challenge: challenge}, nil
}

// acceptMembership accepts an invite that admits an account of another tenant.
// The password, and a code when the account holds a second factor, are checked
// before anything is written, so the membership exists only once its owner has
// proved everything signing in would ask for. Holding the link proves nothing:
// the administrator who issued it holds it too.
func (l *Local) acceptMembership(ctx context.Context, live liveInvite, surface core.Surface,
	in core.AcceptInviteInput,
) (*core.LoginResult, error) {
	if live.hash == "" {
		return nil, errInvalidCredentials()
	}
	if err := auth.Verify(ctx, live.hash, in.Password); err != nil {
		if auth.IsHashBusy(err) {
			return nil, err
		}
		return nil, errInvalidCredentials()
	}
	scope := core.TenantScope{TenantID: live.tenant.ID}
	cand := candidate{user: live.user, actor: live.actor, role: live.role, tenant: scope.TenantID}
	sessionActor := cand.sessionActor(scope)
	enrolment, err := l.enrolmentOf(ctx, live)
	if err != nil {
		return nil, err
	}
	var attempt *core.UserTOTP
	if enrolment.Confirmed() {
		if strings.TrimSpace(in.Code) == "" {
			return nil, core.Invalid("this account has a second factor; accepting needs a code from it").
				WithDetail(core.DetailField, "code")
		}
		state, err := l.verifyCode(ctx, sessionActor, live.user, enrolment, in.Code)
		if err != nil {
			return nil, err
		}
		attempt = &state
	}

	accept := func(m *mutation) error {
		if !live.invite.Pending(m.now) {
			return auth.ErrNoInvite()
		}
		if err := m.tx.AcceptInvite(ctx, live.invite.ID, m.now); err != nil {
			return auth.ErrNoInvite()
		}
		if err := setUserRole(ctx, m.tx, live.actor.ID, live.role); err != nil {
			return err
		}
		if attempt != nil {
			if err := m.tx.SaveUserTOTPAttempt(ctx, live.user.ID, *attempt); err != nil {
				return err
			}
		}
		after := *live.invite
		after.AcceptedAt = &m.now
		trail := inviteTrail(after)
		trail["membership"] = string(live.role)
		if err := m.logAccount(ctx, live.user.ID, core.AccountMemberAdded,
			map[string]any{"role": string(live.role), "invite": live.invite.ID}); err != nil {
			return err
		}
		return m.Record(auditInviteAccept, eventInviteAccepted, subjectInvite, live.invite.ID, "",
			*live.invite, after, trail)
	}
	if attempt != nil {
		return l.mintSession(ctx, scope, sessionActor, live.user, "", factorOrigin(core.LoginMethodPassword), accept)
	}
	challenge, err := l.secondFactorFor(ctx, scope, surface, sessionActor, live.user, core.LoginMethodPassword)
	if err != nil {
		return nil, err
	}
	if challenge == nil {
		return l.mintSession(ctx, scope, sessionActor, live.user, "", passwordOrigin, accept)
	}
	if err := l.write(ctx, sessionActor, accept); err != nil {
		return nil, err
	}
	return &core.LoginResult{Challenge: challenge}, nil
}

// inviteePassword checks the password an acceptance carries. A new account's
// is hashed for storing; an existing account's must match the one it has and
// is kept, so the answer is "".
func (l *Local) inviteePassword(ctx context.Context, live liveInvite, password string) (string, error) {
	if live.hash != "" {
		if err := auth.Verify(ctx, live.hash, password); err != nil {
			if auth.IsHashBusy(err) {
				return "", err
			}
			return "", errInvalidCredentials()
		}
		return "", nil
	}
	if err := auth.ValidatePassword(password); err != nil {
		return "", core.Invalid("password must be at least %d characters", core.MinPasswordLength).
			WithDetail(core.DetailField, "password")
	}
	return l.hasher.Hash(ctx, password)
}
