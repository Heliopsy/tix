// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// ReasonAccountInOtherTenants is the detail a refused account change carries
// when the person also belongs to another tenant.
const ReasonAccountInOtherTenants = "account_in_other_tenants"

// globalStandingKey marks a context inTenant built for a global admin, whose
// actor stands in a tenant it is not a member of and so cannot be looked up
// there to confirm the standing again.
type globalStandingKey struct{}

// withGlobalStanding returns a context marked as a global admin's standing,
// carrying the global admin's account so a change it makes can name them.
func withGlobalStanding(ctx context.Context, by actingAccount) context.Context {
	by.global = true
	return context.WithValue(ctx, globalStandingKey{}, by)
}

// globalStanding reports whether inTenant marked ctx.
func globalStanding(ctx context.Context) bool {
	_, ok := ctx.Value(globalStandingKey{}).(actingAccount)
	return ok
}

// membershipsOf lists every tenant an account holds a membership in.
func (l *Local) membershipsOf(ctx context.Context, userID string) ([]core.AccountMembership, error) {
	var out []core.AccountMembership
	err := l.store.ViewUnscoped(ctx, func(u store.UnscopedTx) error {
		var err error
		out, err = u.ListMemberships(ctx, userID)
		return err
	})
	return out, err
}

// memberElsewhere reports whether an account holds a membership in any tenant
// but tenantID.
func (l *Local) memberElsewhere(ctx context.Context, userID, tenantID string) (bool, error) {
	found, err := l.membershipsOf(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, m := range found {
		if m.TenantID != tenantID {
			return true, nil
		}
	}
	return false, nil
}

// accountReach is what the caller may do to the account behind a user of its
// tenant: change the account itself, or only the person's standing here.
type accountReach struct {
	// elsewhere is whether the person belongs to another tenant too.
	elsewhere bool
	// account is whether the caller may change the account: the person is in
	// this tenant only, or the caller is a global admin.
	account bool
}

// reachOver decides how far a tenant administrator's change to one of its
// users may go. Password, second factor and the account-wide disabled state
// reach every tenant the person belongs to, so for somebody who also belongs
// to another tenant only a global admin may change them; anything else would
// let an administrator of one tenant take over a person who works in another.
//
// It is read before the write that acts on it, because the listing crosses
// tenants and the write is bound to one. A membership granted between the two
// is granted by a global admin or by the person, and neither is the threat.
func (l *Local) reachOver(ctx context.Context, account *core.User, tenantID string) (accountReach, error) {
	elsewhere, err := l.memberElsewhere(ctx, account.ID, tenantID)
	if err != nil || !elsewhere {
		return accountReach{account: err == nil}, err
	}
	out := accountReach{elsewhere: true}
	if globalStanding(ctx) {
		out.account = true
		return out, nil
	}
	if out.account, err = l.globalAllowed(ctx); err != nil {
		return accountReach{}, err
	}
	return out, nil
}

// errAccountInOtherTenants is the refusal of an account change a tenant
// administrator may not make. It names no other tenant.
func errAccountInOtherTenants() error {
	return core.Forbidden("this person also belongs to another tenant, so only they or a global admin "+
		"can change their password, second factor or account").
		WithDetail("reason", ReasonAccountInOtherTenants)
}

// accountForUser reads the account behind a user of the caller's tenant.
func (l *Local) accountForUser(ctx context.Context, actor *core.Actor, id string) (*core.User, error) {
	var out *core.User
	err := l.read(ctx, actor, func(tx store.Tx) error {
		u, _, err := ownUser(ctx, tx, id)
		out = u
		return err
	})
	return out, err
}

// StandingWithdrawn reports whether an actor's credentials must stop working
// now: the actor is disabled in its tenant, or its account is disabled. It is
// read on every request, because a disabled account has sessions and tokens in
// every tenant it belongs to and a transaction can revoke them in one only.
func StandingWithdrawn(ctx context.Context, tx store.Tx, actorID string) (bool, error) {
	a, err := tx.GetActor(ctx, actorID)
	if core.IsKind(err, core.KindNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if a.DisabledAt != nil {
		return true, nil
	}
	if a.UserID == "" {
		return false, nil
	}
	u, err := tx.GetUser(ctx, a.UserID)
	if core.IsKind(err, core.KindNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return u.DisabledAt != nil, nil
}

// describeAccount fills in what WhoAmI tells a caller about themselves. Every
// credential learns its tenant's key, name and resolved theme, which every
// member sees on every page anyway. A person also learns their account, and
// for an interactive credential (a session, an SSH sign-in, a shell holding
// the database) or a token carrying account:read every tenant the account is
// a member of. A token without that scope says nothing about its owner's
// other tenants, so an agent's token in one tenant cannot list where else its
// owner works.
//
// A token's actor reads as an agent whoever it was minted for, so a token is
// looked up by the actor it acts as: a person's token finds their account, an
// agent's finds none.
func (l *Local) describeAccount(ctx context.Context, out *core.Actor) error {
	var account *core.User
	if err := l.read(ctx, out, func(tx store.Tx) error {
		if t, err := tx.GetTenant(ctx); err == nil {
			out.TenantKey, out.TenantName = t.Key, t.Name
			theme := l.ResolveTheme(t)
			out.Theme = &theme
		}
		if out.Kind != core.ActorUser && out.TokenID == "" {
			return nil
		}
		u, err := accountOfActor(ctx, tx, out.ID)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		account = u
		return err
	}); err != nil || account == nil {
		return err
	}
	out.AccountID, out.Email = account.ID, account.Email
	standing, err := l.factorStandingOf(ctx, out)
	if err != nil {
		return err
	}
	out.SecondFactorDue = standing.due()
	out.SecondFactorEnrollmentRequired = standing.gated(l.clock.Now())
	if out.TokenID != "" && !out.HasScope(core.ScopeAccountRead) {
		return nil
	}
	found, err := l.membershipsOf(ctx, account.ID)
	if err != nil {
		return err
	}
	out.Memberships = found
	return nil
}
