// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// Event types and audit actions for what an identity provider changes.
const (
	eventSCIMGroupCreated core.EventType = "scim.group.created"
	eventSCIMGroupUpdated core.EventType = "scim.group.updated"
	eventSCIMGroupDeleted core.EventType = "scim.group.deleted"

	auditSCIMUserCreate  = "scim.user.create"
	auditSCIMUserLink    = "scim.user.link"
	auditSCIMUserUpdate  = "scim.user.update"
	auditSCIMUserDelete  = "scim.user.delete"
	auditSCIMGroupCreate = "scim.group.create"
	auditSCIMGroupUpdate = "scim.group.update"
	auditSCIMGroupDelete = "scim.group.delete"
	auditSCIMRole        = "scim.role"
)

// subjectSCIMGroup is the event subject of a SCIM group.
const subjectSCIMGroup = "scim_group"

var _ core.Provisioner = (*Local)(nil)

// errSCIMOff is the answer of every provisioning call while SCIM is off.
func errSCIMOff() error {
	return core.NotFound("SCIM provisioning is off for this tenant; an admin turns it on with tix tenant scim enable")
}

// provisioning authorizes a provider and returns the context its changes are
// recorded under: the scim channel, naming the token, whatever surface called.
func (l *Local) provisioning(ctx context.Context) (context.Context, *core.Actor, error) {
	actor, err := l.authorize(ctx, authz.ActionSCIMProvision, authz.Resource{})
	if err != nil {
		return nil, nil, err
	}
	via := core.ViaFrom(ctx).Derive(core.ViaSCIM)
	if via.TokenID == "" {
		via.TokenID, via.TokenName = actor.TokenID, actor.TokenName
	}
	return core.WithVia(ctx, via), actor, nil
}

// scimOn refuses a provisioning call while SCIM is off for the tenant.
func scimOn(ctx context.Context, tx store.Tx) error {
	c, err := tx.GetSCIMConfig(ctx)
	if err != nil {
		return err
	}
	if !c.Enabled {
		return errSCIMOff()
	}
	return nil
}

// SCIMReady refuses a caller that may not provision, and answers not found
// while SCIM is off.
func (l *Local) SCIMReady(ctx context.Context) error {
	ctx, actor, err := l.provisioning(ctx)
	if err != nil {
		return err
	}
	return l.read(ctx, actor, func(tx store.Tx) error { return scimOn(ctx, tx) })
}

// scimMember resolves a user of this tenant that SCIM may address: a person
// with a membership here. Anybody else reads as missing.
func scimMember(ctx context.Context, tx store.Tx, id string) (*core.Actor, *core.User, error) {
	missing := core.NotFound("user %q", id)
	if strings.TrimSpace(id) == "" {
		return nil, nil, missing
	}
	a, err := tx.GetActor(ctx, id)
	if core.IsKind(err, core.KindNotFound) {
		return nil, nil, missing
	}
	if err != nil {
		return nil, nil, err
	}
	if a.Kind != core.ActorUser || a.UserID == "" {
		return nil, nil, missing
	}
	if _, err := tx.GetMember(ctx, a.ID); err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, nil, missing
		}
		return nil, nil, err
	}
	u, err := tx.GetUser(ctx, a.UserID)
	if core.IsKind(err, core.KindNotFound) {
		return nil, nil, missing
	}
	return a, u, err
}

// scimUserOf builds the SCIM view of a member. A member a provider never
// pushed reads with their address as the userName.
func scimUserOf(ctx context.Context, tx store.Tx, a *core.Actor, u *core.User) (*core.SCIMUser, error) {
	out := &core.SCIMUser{
		ID: a.ID, UserName: u.Email, Email: u.Email, DisplayName: a.DisplayName,
		Active: a.DisabledAt == nil && u.DisabledAt == nil, Created: u.CreatedAt, Modified: u.UpdatedAt,
	}
	row, err := tx.GetSCIMUser(ctx, a.ID)
	switch {
	case err == nil:
		out.UserName, out.ExternalID = row.UserName, row.ExternalID
		out.GivenName, out.FamilyName = row.GivenName, row.FamilyName
		if row.Email != "" {
			out.Email = row.Email
		}
		out.Created, out.Modified = row.CreatedAt, row.UpdatedAt
	case !core.IsKind(err, core.KindNotFound):
		return nil, err
	}
	groups, err := tx.SCIMGroupsOf(ctx, a.ID)
	if err != nil {
		return nil, err
	}
	for _, g := range groups {
		out.Groups = append(out.Groups, core.SCIMRef{ID: g.ID, Display: g.DisplayName})
	}
	return out, nil
}

// scimMatches reports whether a user satisfies every condition.
func scimUserMatches(u *core.SCIMUser, conds []core.SCIMCondition) bool {
	for _, c := range conds {
		var ok bool
		switch c.Attr {
		case core.SCIMAttrID:
			ok = u.ID == c.Value
		case core.SCIMAttrUserName:
			ok = strings.EqualFold(u.UserName, c.Value)
		case core.SCIMAttrExternalID:
			ok = u.ExternalID == c.Value
		case core.SCIMAttrEmail:
			ok = strings.EqualFold(u.Email, c.Value)
		case core.SCIMAttrDisplayName:
			ok = u.DisplayName == c.Value
		}
		if !ok {
			return false
		}
	}
	return true
}

// page slices a sorted listing by a one-based start index and a count.
func scimPage[T any](items []T, q core.SCIMQuery) []T {
	start := max(q.StartIndex, 1) - 1
	if start >= len(items) || q.Count <= 0 {
		return []T{}
	}
	end := min(start+q.Count, len(items))
	return items[start:end]
}

// accountByEmail returns the account with an address, or nil.
func (l *Local) accountByEmail(ctx context.Context, email string) (*core.User, error) {
	found, err := l.inviteeByEmail(ctx, core.NormalEmail(email))
	return found.user, err
}

// SCIMListUsers lists the tenant's members that satisfy every condition,
// ordered by identifier, with the number of matches before paging. The page
// is cut here from the ordered identifiers rather than by the database, which
// keeps every query keyset-shaped.
func (l *Local) SCIMListUsers(ctx context.Context, q core.SCIMQuery) ([]core.SCIMUser, int, error) {
	ctx, actor, err := l.provisioning(ctx)
	if err != nil {
		return nil, 0, err
	}
	var byEmail *core.User
	for _, c := range q.Conditions {
		if c.Attr == core.SCIMAttrUserName || c.Attr == core.SCIMAttrEmail {
			if byEmail, err = l.accountByEmail(ctx, c.Value); err != nil {
				return nil, 0, err
			}
			break
		}
	}
	var out []core.SCIMUser
	total := 0
	err = l.read(ctx, actor, func(tx store.Tx) error {
		if err := scimOn(ctx, tx); err != nil {
			return err
		}
		ids, filtered, err := scimUserCandidates(ctx, tx, q.Conditions, byEmail)
		if err != nil {
			return err
		}
		if !filtered {
			total = len(ids)
			ids = scimPage(ids, q)
		}
		users := []core.SCIMUser{}
		for _, id := range ids {
			a, u, err := scimMember(ctx, tx, id)
			if core.IsKind(err, core.KindNotFound) {
				continue
			}
			if err != nil {
				return err
			}
			su, err := scimUserOf(ctx, tx, a, u)
			if err != nil {
				return err
			}
			if !filtered || scimUserMatches(su, q.Conditions) {
				users = append(users, *su)
			}
		}
		if filtered {
			total = len(users)
			users = scimPage(users, q)
		}
		out = users
		return nil
	})
	return out, total, err
}

// scimUserCandidates returns the identifiers a listing considers, sorted, and
// whether they still have to be checked against the conditions. A condition
// on an identifier, a userName, an externalId or an address narrows them to a
// point lookup; with none, every member of the tenant is a candidate.
func scimUserCandidates(ctx context.Context, tx store.Tx, conds []core.SCIMCondition, byEmail *core.User,
) ([]string, bool, error) {
	if len(conds) == 0 {
		members, err := tx.ListMembers(ctx)
		if err != nil {
			return nil, false, err
		}
		ids := make([]string, 0, len(members))
		for _, m := range members {
			ids = append(ids, m.ActorID)
		}
		slices.Sort(ids)
		var users []string
		for _, id := range ids {
			if a, err := tx.GetActor(ctx, id); err == nil && a.Kind == core.ActorUser && a.UserID != "" {
				users = append(users, id)
			} else if err != nil && !core.IsKind(err, core.KindNotFound) {
				return nil, false, err
			}
		}
		return users, false, nil
	}
	var ids []string
	add := func(rows []store.SCIMUserRow, err error) error {
		for _, r := range rows {
			ids = append(ids, r.ActorID)
		}
		return err
	}
	c := conds[0]
	var err error
	switch c.Attr {
	case core.SCIMAttrID:
		ids = []string{c.Value}
	case core.SCIMAttrUserName:
		err = add(tx.SCIMUsersByName(ctx, c.Value))
	case core.SCIMAttrExternalID:
		err = add(tx.SCIMUsersByExternalID(ctx, c.Value))
	case core.SCIMAttrEmail:
		err = add(tx.SCIMUsersByEmail(ctx, c.Value))
	default:
		return nil, false, core.Invalid("users cannot be filtered by %s", c.Attr)
	}
	if err != nil {
		return nil, false, err
	}
	if byEmail != nil && (c.Attr == core.SCIMAttrUserName || c.Attr == core.SCIMAttrEmail) {
		a, err := tx.GetActorByUser(ctx, byEmail.ID)
		switch {
		case err == nil:
			ids = append(ids, a.ID)
		case !core.IsKind(err, core.KindNotFound):
			return nil, false, err
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids), true, nil
}

// SCIMGetUser returns one member as SCIM sees them.
func (l *Local) SCIMGetUser(ctx context.Context, id string) (*core.SCIMUser, error) {
	ctx, actor, err := l.provisioning(ctx)
	if err != nil {
		return nil, err
	}
	var out *core.SCIMUser
	err = l.read(ctx, actor, func(tx store.Tx) error {
		if err := scimOn(ctx, tx); err != nil {
			return err
		}
		a, u, err := scimMember(ctx, tx, id)
		if err != nil {
			return err
		}
		out, err = scimUserOf(ctx, tx, a, u)
		return err
	})
	return out, err
}

// scimEmail is the address a user is created with: the one the provider sent,
// or the userName when that is an address.
func scimEmail(in core.SCIMUser) (string, error) {
	email := strings.TrimSpace(in.Email)
	if email == "" && strings.Contains(in.UserName, "@") {
		email = in.UserName
	}
	if email == "" {
		return "", core.Invalid("a user needs an email address, in emails or as the userName").
			WithDetail(core.DetailField, "emails")
	}
	if err := core.ValidateEmail(email); err != nil {
		return "", err
	}
	return core.NormalEmail(email), nil
}

// validSCIMUser checks what every write of a user needs.
func validSCIMUser(u core.SCIMUser) error {
	switch {
	case strings.TrimSpace(u.UserName) == "":
		return core.Invalid("userName is required").WithDetail(core.DetailField, "userName")
	case len(u.UserName) > core.MaxSCIMName || len(u.ExternalID) > core.MaxSCIMName:
		return core.Invalid("userName and externalId are at most %d bytes", core.MaxSCIMName).
			WithDetail(core.DetailField, "userName")
	}
	return nil
}

// errSCIMTaken is the refusal of a name another resource already holds.
func errSCIMTaken(format string, args ...any) error {
	return core.Conflict(format, args...).WithDetail("reason", "uniqueness")
}

// userNameFree refuses a userName another member of the tenant holds.
func userNameFree(ctx context.Context, tx store.Tx, userName, self string) error {
	rows, err := tx.SCIMUsersByName(ctx, userName)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if r.ActorID != self {
			return errSCIMTaken("userName %q is already provisioned in this tenant", userName)
		}
	}
	return nil
}

// freeHandle returns base, or base with the first free number after it, so a
// provisioned person never fails on a handle nobody typed.
func freeHandle(ctx context.Context, tx store.Tx, base string) (string, error) {
	base = userHandle("", base)
	if base == "" {
		base = "user"
	}
	for n := 1; n < 1000; n++ {
		handle := base
		if n > 1 {
			handle = fmt.Sprintf("%s%d", base, n)
		}
		_, err := tx.GetActorByHandle(ctx, handle)
		if core.IsKind(err, core.KindNotFound) {
			return handle, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", core.Conflict("no free handle starts with %q", base)
}

// scimDisplayName is the name a user gets: the one sent, else the name parts.
func scimDisplayName(in core.SCIMUser) string {
	if name := strings.TrimSpace(in.DisplayName); name != "" {
		return name
	}
	return strings.TrimSpace(strings.TrimSpace(in.GivenName) + " " + strings.TrimSpace(in.FamilyName))
}

// SCIMCreateUser provisions a person. An account with the user's address
// that already has an actor in this tenant is linked, and given a membership
// when it has none. An address held by an account of another tenant only is
// refused, as an invite is: an administrator of one tenant never grants an
// account a membership its owner did not accept. Otherwise an account is
// created with no password, to sign in with SSO or after an invite or reset.
func (l *Local) SCIMCreateUser(ctx context.Context, in core.SCIMUser) (*core.SCIMUser, error) {
	ctx, actor, err := l.provisioning(ctx)
	if err != nil {
		return nil, err
	}
	if err := validSCIMUser(in); err != nil {
		return nil, err
	}
	email, err := scimEmail(in)
	if err != nil {
		return nil, err
	}
	in.Email = email
	existing, err := l.accountByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	var out *core.SCIMUser
	err = l.write(ctx, actor, func(m *mutation) error {
		if err := scimOn(ctx, m.tx); err != nil {
			return err
		}
		if err := userNameFree(ctx, m.tx, in.UserName, ""); err != nil {
			return err
		}
		var subject *core.Actor
		var account *core.User
		action := auditSCIMUserCreate
		if existing != nil {
			if subject, err = l.scimLink(ctx, m, existing, in); err != nil {
				return err
			}
			account, action = existing, auditSCIMUserLink
		} else if subject, account, err = l.scimNewAccount(ctx, m, in); err != nil {
			return err
		}
		if err := m.tx.PutSCIMUser(ctx, store.SCIMUserRow{
			ActorID: subject.ID, UserName: strings.TrimSpace(in.UserName), ExternalID: in.ExternalID, Email: email,
			GivenName: in.GivenName, FamilyName: in.FamilyName, CreatedAt: m.now, UpdatedAt: m.now,
		}); err != nil {
			return err
		}
		if out, err = scimUserOf(ctx, m.tx, subject, account); err != nil {
			return err
		}
		if err := m.tx.TouchSCIM(ctx, m.now); err != nil {
			return err
		}
		typ := eventUserCreated
		if action == auditSCIMUserLink {
			typ = eventUserUpdated
		}
		return m.Record(action, typ, "user", subject.ID, "", nil, out,
			map[string]any{"email": email, "user_name": out.UserName, "external_id": out.ExternalID,
				"handle": subject.Handle, "active": out.Active})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// scimLink readies an existing account as a provisioned member here.
func (l *Local) scimLink(ctx context.Context, m *mutation, account *core.User, in core.SCIMUser) (*core.Actor, error) {
	subject, err := m.tx.GetActorByUser(ctx, account.ID)
	if core.IsKind(err, core.KindNotFound) {
		return nil, errSCIMTaken("%s belongs to an account of another tenant; invite them to this tenant "+
			"with tix user invite before provisioning them", account.Email)
	}
	if err != nil {
		return nil, err
	}
	switch _, err := m.tx.GetSCIMUser(ctx, subject.ID); {
	case err == nil:
		return nil, errSCIMTaken("%s is already provisioned in this tenant", account.Email)
	case !core.IsKind(err, core.KindNotFound):
		return nil, err
	}
	role, err := userRole(ctx, m.tx, subject.ID)
	if err != nil {
		return nil, err
	}
	if role == "" {
		if err := setUserRole(ctx, m.tx, subject.ID, defaultUserRole); err != nil {
			return nil, err
		}
		if err := m.logAccount(ctx, account.ID, core.AccountMemberAdded,
			map[string]any{"role": string(defaultUserRole)}); err != nil {
			return nil, err
		}
	}
	if err := scimSetActive(ctx, m, account, subject, in.Active); err != nil {
		return nil, err
	}
	if name := scimDisplayName(in); name != "" && name != subject.DisplayName {
		subject.DisplayName = name
		if err := m.tx.UpdateActor(ctx, subject); err != nil {
			return nil, err
		}
	}
	return subject, nil
}

// scimNewAccount creates an account with no password, its actor here and a
// viewer membership.
func (l *Local) scimNewAccount(ctx context.Context, m *mutation, in core.SCIMUser) (*core.Actor, *core.User, error) {
	handle, err := freeHandle(ctx, m.tx, in.Email)
	if err != nil {
		return nil, nil, err
	}
	id := l.ids.New()
	name := scimDisplayName(in)
	u := &core.User{ID: id, Email: in.Email, DisplayName: name}
	if err := m.tx.CreateUser(ctx, u, ""); err != nil {
		return nil, nil, err
	}
	a := &core.Actor{ID: id, UserID: id, Kind: core.ActorUser, Handle: handle, DisplayName: name}
	if err := m.tx.CreateActor(ctx, a); err != nil {
		return nil, nil, err
	}
	if err := setUserRole(ctx, m.tx, id, defaultUserRole); err != nil {
		return nil, nil, err
	}
	if err := m.logAccount(ctx, id, core.AccountCreated, map[string]any{"role": string(defaultUserRole)}); err != nil {
		return nil, nil, err
	}
	if err := scimSetActive(ctx, m, u, a, in.Active); err != nil {
		return nil, nil, err
	}
	return a, u, nil
}

// scimSetActive applies a user's active flag to their standing in this
// tenant. Deactivating ends their sessions here; tokens and SSH keys are kept,
// and stop working for as long as the mark stands, because every credential
// check asks whether the actor is disabled.
func scimSetActive(ctx context.Context, m *mutation, account *core.User, subject *core.Actor, active bool) error {
	switch {
	case !active && subject.DisabledAt == nil:
		now := m.now
		subject.DisabledAt = &now
		if _, err := m.tx.DeleteActorSessions(ctx, subject.ID); err != nil {
			return err
		}
		if err := m.logAccount(ctx, account.ID, core.AccountDisabled, map[string]any{"scope": "tenant"}); err != nil {
			return err
		}
	case active && subject.DisabledAt != nil:
		subject.DisabledAt = nil
		if err := m.logAccount(ctx, account.ID, core.AccountEnabled, map[string]any{"scope": "tenant"}); err != nil {
			return err
		}
	default:
		return nil
	}
	return m.tx.UpdateActor(ctx, subject)
}

// SCIMUpdateUser applies a provider's change to a member in one transaction.
// The address of the account is not changed: it is global, and a provider of
// one tenant does not own it; the address sent is kept with the SCIM record.
func (l *Local) SCIMUpdateUser(ctx context.Context, id string, apply func(*core.SCIMUser) error) (*core.SCIMUser, error) {
	ctx, actor, err := l.provisioning(ctx)
	if err != nil {
		return nil, err
	}
	var out *core.SCIMUser
	err = l.write(ctx, actor, func(m *mutation) error {
		if err := scimOn(ctx, m.tx); err != nil {
			return err
		}
		subject, account, err := scimMember(ctx, m.tx, id)
		if err != nil {
			return err
		}
		before, err := scimUserOf(ctx, m.tx, subject, account)
		if err != nil {
			return err
		}
		next := *before
		next.Groups = slices.Clone(before.Groups)
		if err := apply(&next); err != nil {
			return err
		}
		if err := validSCIMUser(next); err != nil {
			return err
		}
		if err := userNameFree(ctx, m.tx, next.UserName, subject.ID); err != nil {
			return err
		}
		if err := l.scimApplyUser(ctx, m, subject, account, before, next); err != nil {
			return err
		}
		if out, err = scimUserOf(ctx, m.tx, subject, account); err != nil {
			return err
		}
		if err := m.tx.TouchSCIM(ctx, m.now); err != nil {
			return err
		}
		return m.Record(auditSCIMUserUpdate, eventUserUpdated, "user", subject.ID, "", before, out,
			map[string]any{"email": account.Email, "user_name": out.UserName, "active": out.Active})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// scimApplyUser writes the difference between a member and what a provider
// wants them to be.
func (l *Local) scimApplyUser(ctx context.Context, m *mutation, subject *core.Actor, account *core.User,
	before *core.SCIMUser, next core.SCIMUser,
) error {
	created := before.Created
	if _, err := m.tx.GetSCIMUser(ctx, subject.ID); core.IsKind(err, core.KindNotFound) {
		created = m.now
	} else if err != nil {
		return err
	}
	email := strings.TrimSpace(next.Email)
	if email == "" {
		email = account.Email
	}
	if err := m.tx.PutSCIMUser(ctx, store.SCIMUserRow{
		ActorID: subject.ID, UserName: strings.TrimSpace(next.UserName), ExternalID: next.ExternalID,
		Email: email, GivenName: next.GivenName, FamilyName: next.FamilyName, CreatedAt: created, UpdatedAt: m.now,
	}); err != nil {
		return err
	}
	if next.DisplayName != before.DisplayName {
		subject.DisplayName = strings.TrimSpace(next.DisplayName)
		if err := m.tx.UpdateActor(ctx, subject); err != nil {
			return err
		}
	}
	if next.Active != before.Active {
		return scimSetActive(ctx, m, account, subject, next.Active)
	}
	return nil
}

// SCIMDeleteUser removes a member from the tenant on a provider's behalf: the
// membership goes, their sessions, tokens and SSH keys here are revoked, and
// they leave every SCIM group. The account and the actor stay, so history
// still names them, even when this was their only tenant.
func (l *Local) SCIMDeleteUser(ctx context.Context, id string) error {
	ctx, actor, err := l.provisioning(ctx)
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		if err := scimOn(ctx, m.tx); err != nil {
			return err
		}
		subject, account, err := scimMember(ctx, m.tx, id)
		if err != nil {
			return err
		}
		before, err := scimUserOf(ctx, m.tx, subject, account)
		if err != nil {
			return err
		}
		if err := m.tx.RemoveMember(ctx, subject.ID); err != nil {
			return err
		}
		ended, err := revokeStanding(ctx, m, subject.ID)
		if err != nil {
			return err
		}
		if err := m.tx.DropSCIMMember(ctx, subject.ID); err != nil {
			return err
		}
		if err := m.tx.DeleteSCIMUser(ctx, subject.ID); err != nil {
			return err
		}
		if err := m.logAccount(ctx, account.ID, core.AccountMemberRemoved, nil); err != nil {
			return err
		}
		if err := m.tx.TouchSCIM(ctx, m.now); err != nil {
			return err
		}
		return m.Record(auditSCIMUserDelete, eventUserDeleted, "user", subject.ID, "", before, nil,
			map[string]any{"email": account.Email, "handle": subject.Handle,
				"sessions_ended": ended.sessions, "tokens_revoked": ended.tokens, "keys_revoked": ended.keys})
	})
}

// scimGroupOf builds the SCIM view of a group, with its members when asked.
func scimGroupOf(ctx context.Context, tx store.Tx, g store.SCIMGroupRow, members bool) (*core.SCIMGroup, error) {
	out := &core.SCIMGroup{ID: g.ID, ExternalID: g.ExternalID, DisplayName: g.DisplayName,
		Created: g.CreatedAt, Modified: g.UpdatedAt}
	if !members {
		return out, nil
	}
	ids, err := tx.SCIMGroupMembers(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		ref := core.SCIMRef{ID: id}
		if a, err := tx.GetActor(ctx, id); err == nil {
			ref.Display = a.DisplayName
			if ref.Display == "" {
				ref.Display = a.Handle
			}
		} else if !core.IsKind(err, core.KindNotFound) {
			return nil, err
		}
		out.Members = append(out.Members, ref)
	}
	return out, nil
}

// SCIMListGroups lists the tenant's SCIM groups that satisfy every condition,
// ordered by identifier, with the number of matches before paging.
func (l *Local) SCIMListGroups(ctx context.Context, q core.SCIMQuery) ([]core.SCIMGroup, int, error) {
	ctx, actor, err := l.provisioning(ctx)
	if err != nil {
		return nil, 0, err
	}
	var out []core.SCIMGroup
	total := 0
	err = l.read(ctx, actor, func(tx store.Tx) error {
		if err := scimOn(ctx, tx); err != nil {
			return err
		}
		rows, err := scimGroupCandidates(ctx, tx, q.Conditions)
		if err != nil {
			return err
		}
		var kept []store.SCIMGroupRow
		for _, g := range rows {
			ok, err := scimGroupMatches(ctx, tx, g, q.Conditions)
			if err != nil {
				return err
			}
			if ok {
				kept = append(kept, g)
			}
		}
		total = len(kept)
		groups := []core.SCIMGroup{}
		for _, g := range scimPage(kept, q) {
			built, err := scimGroupOf(ctx, tx, g, !q.WithoutMembers)
			if err != nil {
				return err
			}
			groups = append(groups, *built)
		}
		out = groups
		return nil
	})
	return out, total, err
}

// scimGroupCandidates narrows a group listing by its first condition.
func scimGroupCandidates(ctx context.Context, tx store.Tx, conds []core.SCIMCondition) ([]store.SCIMGroupRow, error) {
	if len(conds) == 0 {
		return tx.ListSCIMGroups(ctx)
	}
	c := conds[0]
	switch c.Attr {
	case core.SCIMAttrID:
		g, err := tx.GetSCIMGroup(ctx, c.Value)
		if core.IsKind(err, core.KindNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return []store.SCIMGroupRow{*g}, nil
	case core.SCIMAttrDisplayName:
		return tx.SCIMGroupsByName(ctx, c.Value)
	case core.SCIMAttrExternalID:
		return tx.SCIMGroupsByExternalID(ctx, c.Value)
	case core.SCIMAttrMember:
		return tx.SCIMGroupsOf(ctx, c.Value)
	}
	return nil, core.Invalid("groups cannot be filtered by %s", c.Attr)
}

// scimGroupMatches reports whether a group satisfies every condition.
func scimGroupMatches(ctx context.Context, tx store.Tx, g store.SCIMGroupRow, conds []core.SCIMCondition) (bool, error) {
	for _, c := range conds {
		switch c.Attr {
		case core.SCIMAttrID:
			if g.ID != c.Value {
				return false, nil
			}
		case core.SCIMAttrDisplayName:
			if !strings.EqualFold(g.DisplayName, c.Value) {
				return false, nil
			}
		case core.SCIMAttrExternalID:
			if g.ExternalID != c.Value {
				return false, nil
			}
		case core.SCIMAttrMember:
			ids, err := tx.SCIMGroupMembers(ctx, g.ID)
			if err != nil || !slices.Contains(ids, c.Value) {
				return false, err
			}
		default:
			return false, core.Invalid("groups cannot be filtered by %s", c.Attr)
		}
	}
	return true, nil
}

// SCIMGetGroup returns one SCIM group, with its members when asked.
func (l *Local) SCIMGetGroup(ctx context.Context, id string, members bool) (*core.SCIMGroup, error) {
	ctx, actor, err := l.provisioning(ctx)
	if err != nil {
		return nil, err
	}
	var out *core.SCIMGroup
	err = l.read(ctx, actor, func(tx store.Tx) error {
		if err := scimOn(ctx, tx); err != nil {
			return err
		}
		g, err := tx.GetSCIMGroup(ctx, id)
		if err != nil {
			return err
		}
		out, err = scimGroupOf(ctx, tx, *g, members)
		return err
	})
	return out, err
}

// validSCIMGroup checks a group's name, that no other group holds it, and
// that every member is a person of this tenant. It returns the member ids,
// without repeats.
func validSCIMGroup(ctx context.Context, tx store.Tx, g core.SCIMGroup, self string) ([]string, error) {
	name := strings.TrimSpace(g.DisplayName)
	switch {
	case name == "":
		return nil, core.Invalid("displayName is required").WithDetail(core.DetailField, "displayName")
	case len(name) > core.MaxSCIMName || len(g.ExternalID) > core.MaxSCIMName:
		return nil, core.Invalid("displayName and externalId are at most %d bytes", core.MaxSCIMName).
			WithDetail(core.DetailField, "displayName")
	}
	taken, err := tx.SCIMGroupsByName(ctx, name)
	if err != nil {
		return nil, err
	}
	for _, other := range taken {
		if other.ID != self {
			return nil, errSCIMTaken("a group named %q already exists", name)
		}
	}
	var ids []string
	for _, ref := range g.Members {
		if slices.Contains(ids, ref.ID) {
			continue
		}
		if _, _, err := scimMember(ctx, tx, ref.ID); err != nil {
			if core.IsKind(err, core.KindNotFound) {
				return nil, core.Invalid("member %q is not a user of this tenant", ref.ID).
					WithDetail(core.DetailField, "members")
			}
			return nil, err
		}
		ids = append(ids, ref.ID)
	}
	return ids, nil
}

// SCIMCreateGroup stores a provider's group and gives its members the role
// the group's name is mapped to.
func (l *Local) SCIMCreateGroup(ctx context.Context, in core.SCIMGroup) (*core.SCIMGroup, error) {
	ctx, actor, err := l.provisioning(ctx)
	if err != nil {
		return nil, err
	}
	var out *core.SCIMGroup
	err = l.write(ctx, actor, func(m *mutation) error {
		if err := scimOn(ctx, m.tx); err != nil {
			return err
		}
		ids, err := validSCIMGroup(ctx, m.tx, in, "")
		if err != nil {
			return err
		}
		mapped, err := mappedBefore(ctx, m.tx, ids)
		if err != nil {
			return err
		}
		row := store.SCIMGroupRow{ID: l.ids.New(), DisplayName: strings.TrimSpace(in.DisplayName),
			ExternalID: in.ExternalID, CreatedAt: m.now, UpdatedAt: m.now}
		if err := m.tx.CreateSCIMGroup(ctx, row); err != nil {
			return err
		}
		if err := m.tx.SetSCIMGroupMembers(ctx, row.ID, ids); err != nil {
			return err
		}
		if err := applyMappedRoles(ctx, m, ids, mapped); err != nil {
			return err
		}
		if out, err = scimGroupOf(ctx, m.tx, row, true); err != nil {
			return err
		}
		if err := m.tx.TouchSCIM(ctx, m.now); err != nil {
			return err
		}
		return m.Record(auditSCIMGroupCreate, eventSCIMGroupCreated, subjectSCIMGroup, row.ID, "", nil, out,
			map[string]any{"display_name": row.DisplayName, "members": len(ids)})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SCIMUpdateGroup applies a provider's change to a group in one transaction,
// and settles the roles of everybody whose groups it changed.
func (l *Local) SCIMUpdateGroup(ctx context.Context, id string, apply func(*core.SCIMGroup) error) (*core.SCIMGroup, error) {
	ctx, actor, err := l.provisioning(ctx)
	if err != nil {
		return nil, err
	}
	var out *core.SCIMGroup
	err = l.write(ctx, actor, func(m *mutation) error {
		if err := scimOn(ctx, m.tx); err != nil {
			return err
		}
		row, err := m.tx.GetSCIMGroup(ctx, id)
		if err != nil {
			return err
		}
		before, err := scimGroupOf(ctx, m.tx, *row, true)
		if err != nil {
			return err
		}
		next := *before
		next.Members = slices.Clone(before.Members)
		if err := apply(&next); err != nil {
			return err
		}
		ids, err := validSCIMGroup(ctx, m.tx, next, row.ID)
		if err != nil {
			return err
		}
		touched := refIDs(before.Members)
		for _, id := range ids {
			if !slices.Contains(touched, id) {
				touched = append(touched, id)
			}
		}
		mapped, err := mappedBefore(ctx, m.tx, touched)
		if err != nil {
			return err
		}
		row.DisplayName, row.ExternalID, row.UpdatedAt = strings.TrimSpace(next.DisplayName), next.ExternalID, m.now
		if err := m.tx.UpdateSCIMGroup(ctx, *row); err != nil {
			return err
		}
		if err := m.tx.SetSCIMGroupMembers(ctx, row.ID, ids); err != nil {
			return err
		}
		if err := applyMappedRoles(ctx, m, touched, mapped); err != nil {
			return err
		}
		if out, err = scimGroupOf(ctx, m.tx, *row, true); err != nil {
			return err
		}
		if err := m.tx.TouchSCIM(ctx, m.now); err != nil {
			return err
		}
		return m.Record(auditSCIMGroupUpdate, eventSCIMGroupUpdated, subjectSCIMGroup, row.ID, "", before, out,
			map[string]any{"display_name": row.DisplayName, "members": len(ids)})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SCIMDeleteGroup removes a provider's group; its members who were left in no
// mapped group drop to viewer.
func (l *Local) SCIMDeleteGroup(ctx context.Context, id string) error {
	ctx, actor, err := l.provisioning(ctx)
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		if err := scimOn(ctx, m.tx); err != nil {
			return err
		}
		row, err := m.tx.GetSCIMGroup(ctx, id)
		if err != nil {
			return err
		}
		before, err := scimGroupOf(ctx, m.tx, *row, true)
		if err != nil {
			return err
		}
		ids := refIDs(before.Members)
		mapped, err := mappedBefore(ctx, m.tx, ids)
		if err != nil {
			return err
		}
		if err := m.tx.DeleteSCIMGroup(ctx, id); err != nil {
			return err
		}
		if err := applyMappedRoles(ctx, m, ids, mapped); err != nil {
			return err
		}
		if err := m.tx.TouchSCIM(ctx, m.now); err != nil {
			return err
		}
		return m.Record(auditSCIMGroupDelete, eventSCIMGroupDeleted, subjectSCIMGroup, id, "", before, nil,
			map[string]any{"display_name": row.DisplayName, "members": len(ids)})
	})
}

// refIDs lists the identifiers of references.
func refIDs(refs []core.SCIMRef) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.ID)
	}
	return out
}

// mappedRoles returns the roles mapped to the groups an actor belongs to.
func mappedRoles(ctx context.Context, tx store.Tx, actorID string, mappings map[string]core.Role) ([]core.Role, error) {
	groups, err := tx.SCIMGroupsOf(ctx, actorID)
	if err != nil {
		return nil, err
	}
	var out []core.Role
	for _, g := range groups {
		if role, ok := mappings[core.SCIMKey(g.DisplayName)]; ok {
			out = append(out, role)
		}
	}
	return out, nil
}

// roleMappings reads the group to role mappings keyed by group name.
func roleMappings(ctx context.Context, tx store.Tx) (map[string]core.Role, error) {
	found, err := tx.SCIMGroupRoles(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]core.Role, len(found))
	for _, r := range found {
		out[core.SCIMKey(r.Group)] = r.Role
	}
	return out, nil
}

// mappedBefore records, before a change, which actors are in a mapped group.
func mappedBefore(ctx context.Context, tx store.Tx, actorIDs []string) (map[string]bool, error) {
	mappings, err := roleMappings(ctx, tx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(actorIDs))
	for _, id := range actorIDs {
		roles, err := mappedRoles(ctx, tx, id, mappings)
		if err != nil {
			return nil, err
		}
		out[id] = len(roles) > 0
	}
	return out, nil
}

// highestRole is the most privileged of roles.
func highestRole(roles []core.Role) core.Role {
	best := -1
	for _, r := range roles {
		best = max(best, slices.Index(core.Roles, r))
	}
	if best < 0 {
		return ""
	}
	return core.Roles[best]
}

// applyMappedRoles settles the role of each provisioned member after their
// groups or the mappings changed: the highest role mapped to their groups, or
// viewer for somebody who was in a mapped group and is in none now. People a
// provider never pushed are left alone.
func applyMappedRoles(ctx context.Context, m *mutation, actorIDs []string, before map[string]bool) error {
	mappings, err := roleMappings(ctx, m.tx)
	if err != nil {
		return err
	}
	for _, id := range actorIDs {
		switch _, err := m.tx.GetSCIMUser(ctx, id); {
		case core.IsKind(err, core.KindNotFound):
			continue
		case err != nil:
			return err
		}
		current, err := userRole(ctx, m.tx, id)
		if err != nil || current == "" {
			return err
		}
		roles, err := mappedRoles(ctx, m.tx, id, mappings)
		if err != nil {
			return err
		}
		want := highestRole(roles)
		if want == "" && before[id] {
			want = defaultUserRole
		}
		if want == "" || want == current {
			continue
		}
		if err := scimSetRole(ctx, m, id, current, want); err != nil {
			return err
		}
	}
	return nil
}

// scimSetRole changes a member's role and records why.
func scimSetRole(ctx context.Context, m *mutation, actorID string, was, role core.Role) error {
	if err := setUserRole(ctx, m.tx, actorID, role); err != nil {
		return err
	}
	if account, err := accountOfActor(ctx, m.tx, actorID); err == nil {
		if err := m.logAccount(ctx, account.ID, core.AccountMemberRole,
			map[string]any{"role": string(role), "was": string(was)}); err != nil {
			return err
		}
	} else if !core.IsKind(err, core.KindNotFound) {
		return err
	}
	return m.Record(auditSCIMRole, eventUserUpdated, "user", actorID, "",
		map[string]any{"role": string(was)}, map[string]any{"role": string(role)},
		map[string]any{"role": string(role), "was": string(was)})
}
