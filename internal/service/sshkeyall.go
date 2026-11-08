// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// enrolEverywhere enrols the caller's own key in every tenant their account is
// a member of. Each tenant gets its own transaction and its own audit entry,
// written by the person's actor there, and a tenant already holding the key,
// or one where the person may not enrol keys, is passed over. The answer is
// the enrolment in the caller's tenant, listing the tenants this call wrote to.
//
// The tenants are written one after another, so a failure part way leaves the
// earlier enrolments in place; running it again passes over them.
func (l *Local) enrolEverywhere(ctx context.Context, actor *core.Actor, parsed parsedKey,
	label string,
) (*core.SSHKey, error) {
	if actor.TokenID != "" && !actor.HasScope(core.ScopeAccountRead) {
		return nil, core.Forbidden("enrolling a key in every tenant needs a session or a token carrying %s",
			core.ScopeAccountRead)
	}
	account, err := l.accountBehind(ctx, actor)
	if err != nil {
		return nil, err
	}
	found, err := l.membershipsOf(ctx, account.ID)
	if err != nil {
		return nil, err
	}
	var (
		home, first *core.SSHKey
		enrolledIn  []string
	)
	for _, m := range found {
		writer, wctx, ok, err := l.enrolmentWriter(ctx, actor, account, m)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		key, err := l.enrolIn(wctx, writer, "", parsed, label)
		if core.IsKind(err, core.KindConflict) {
			continue
		}
		if err != nil {
			return nil, err
		}
		enrolledIn = append(enrolledIn, m.TenantKey)
		if first == nil {
			first = key
		}
		if m.TenantID == actor.TenantID {
			home = key
		}
	}
	if len(enrolledIn) == 0 {
		return nil, core.Conflict("that key is already enrolled in every tenant you belong to")
	}
	if home == nil {
		home = l.enrolledHere(ctx, actor, parsed.fingerprint, first)
	}
	out := *home
	out.EnrolledIn = enrolledIn
	return &out, nil
}

// accountBehind reads the account behind the calling actor.
func (l *Local) accountBehind(ctx context.Context, actor *core.Actor) (*core.User, error) {
	var account *core.User
	err := l.read(ctx, actor, func(tx store.Tx) error {
		u, err := accountOfActor(ctx, tx, actor.ID)
		account = u
		return err
	})
	if core.IsKind(err, core.KindNotFound) {
		return nil, core.Invalid("this actor has no account, so there are no other tenants to enrol the key in")
	}
	return account, err
}

// enrolmentWriter is the actor and context an enrolment in one membership's
// tenant is written with: the caller in its own tenant, and the person's actor
// elsewhere, with nothing on the access channel that names a credential of the
// caller's tenant. ok is false for a membership the person cannot use there.
func (l *Local) enrolmentWriter(ctx context.Context, actor *core.Actor, account *core.User,
	m core.AccountMembership,
) (*core.Actor, context.Context, bool, error) {
	if m.TenantID == actor.TenantID {
		return actor, ctx, true, nil
	}
	cand, err := l.memberIn(ctx, account, m.TenantID)
	if core.IsKind(err, core.KindNotFound) {
		return nil, nil, false, nil
	}
	if err != nil || !cand.member() {
		return nil, nil, false, err
	}
	writer := cand.sessionActor(core.TenantScope{TenantID: m.TenantID})
	if !l.mayEnrolKeys(writer) {
		return nil, nil, false, nil
	}
	via := core.ViaFrom(ctx)
	via.TokenID, via.TokenName, via.Session = "", "", ""
	return writer, core.WithVia(core.WithActor(ctx, writer), via), true, nil
}

// mayEnrolKeys reports whether an actor may enrol keys in its own tenant.
func (l *Local) mayEnrolKeys(a *core.Actor) bool {
	return l.policy.Can(a, authz.ActionTokenAdmin, authz.Resource{TenantID: a.TenantID}) == nil
}

// memberIn reads an account's actor in a tenant and the role it holds there.
func (l *Local) memberIn(ctx context.Context, account *core.User, tenantID string) (candidate, error) {
	cand := candidate{user: account, tenant: tenantID}
	err := l.read(ctx, core.SystemActor(tenantID), func(tx store.Tx) error {
		a, err := tx.GetActorByUser(ctx, account.ID)
		if err != nil {
			return err
		}
		role, err := userRole(ctx, tx, a.ID)
		cand.actor, cand.role = a, role
		return err
	})
	return cand, err
}

// enrolledHere finds the caller's existing enrolment of a fingerprint, for an
// all-tenants run that found the key already in the caller's tenant, falling
// back to the first enrolment the run made.
func (l *Local) enrolledHere(ctx context.Context, actor *core.Actor, fingerprint string,
	fallback *core.SSHKey,
) *core.SSHKey {
	var found *core.SSHKey
	_ = l.read(ctx, actor, func(tx store.Tx) error {
		keys, err := tx.ListSSHKeys(ctx, actor.ID)
		for i := range keys {
			if keys[i].Fingerprint == fingerprint && keys[i].Active() {
				found = &keys[i]
			}
		}
		return err
	})
	if found == nil {
		return fallback
	}
	return found
}
