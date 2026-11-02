// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// nonceKey returns the key sign-in challenges are signed under, derived once.
func (l *Local) nonceKey() ([]byte, error) {
	l.nonceOnce.Do(func() {
		l.nonceKeyBytes, l.nonceKeyErr = auth.SSHNonceKey(l.totpKey)
	})
	return l.nonceKeyBytes, l.nonceKeyErr
}

// SSHLoginChallenge issues a signed nonce for this tenant and audience.
func (l *Local) SSHLoginChallenge(ctx context.Context) (*core.SSHChallenge, error) {
	scope, err := core.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	key, err := l.nonceKey()
	if err != nil {
		return nil, err
	}
	audience := core.AudienceFrom(ctx)
	nonce, minted, err := auth.MintSSHNonce(key, auth.SSHNonce{
		TenantID: scope.TenantID, Audience: audience, IssuedAt: l.clock.Now(),
	})
	if err != nil {
		return nil, err
	}
	return &core.SSHChallenge{
		Nonce:     nonce,
		Namespace: core.SSHLoginNamespace(audience),
		ExpiresAt: minted.ExpiresAt,
	}, nil
}

// SSHLogin is the first step of a sign-in that proves an enrolled key.
//
// The checks run in an order that discloses nothing to a stranger: the nonce
// and the signature come before the enrolment is read, so only the holder of
// the private key learns whether it is enrolled or revoked.
func (l *Local) SSHLogin(ctx context.Context, in core.SSHLoginInput) (*core.LoginResult, error) {
	scope, err := core.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	nonce, err := l.checkNonce(ctx, scope, in.Nonce)
	if err != nil {
		return nil, err
	}
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(in.PublicKey))
	if err != nil {
		return nil, core.Invalid("that is not an ssh public key; expected the contents of a .pub file")
	}
	namespace := core.SSHLoginNamespace(nonce.Audience)
	if err := auth.VerifySSH(pub, in.Signature, namespace, []byte(strings.TrimSpace(in.Nonce))); err != nil {
		return nil, err
	}
	if err := l.checkNamedTenant(ctx, scope, in.Tenant); err != nil {
		return nil, err
	}
	key, cand, err := l.enrolledPerson(ctx, scope, ssh.FingerprintSHA256(pub))
	if err != nil {
		return nil, err
	}
	sessionActor := cand.sessionActor(scope)

	challenge, err := l.secondFactorFor(ctx, scope, core.SurfaceCLI, sessionActor, cand.user, core.LoginMethodSSHKey)
	if err != nil {
		return nil, err
	}
	if challenge != nil {
		if err := l.claimNonce(ctx, sessionActor, key, nonce.IssuedAt); err != nil {
			return nil, err
		}
		return &core.LoginResult{Challenge: challenge}, nil
	}
	out, err := l.mintSessionIf(ctx, scope, sessionActor, core.LoginMethodSSHKey, func(m *mutation) (bool, error) {
		return claim(ctx, m.tx, key, nonce.IssuedAt, m.now)
	})
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, errNonceUsed()
	}
	return out, nil
}

func errNonceUsed() error {
	return core.Unauthenticated("this login challenge was already used with this key; start again")
}

// checkNonce verifies the challenge was minted here, for this tenant and host.
func (l *Local) checkNonce(ctx context.Context, scope core.TenantScope, raw string) (auth.SSHNonce, error) {
	key, err := l.nonceKey()
	if err != nil {
		return auth.SSHNonce{}, err
	}
	nonce, err := auth.ParseSSHNonce(key, raw, l.clock.Now())
	if err != nil {
		return auth.SSHNonce{}, err
	}
	if nonce.TenantID != scope.TenantID || nonce.Audience != core.AudienceFrom(ctx) {
		return auth.SSHNonce{}, core.Unauthenticated("the login challenge is not valid for this server; start again")
	}
	return nonce, nil
}

// checkNamedTenant refuses a sign-in that names a tenant other than the one
// the request resolved, rather than signing in somewhere the caller did not
// mean.
func (l *Local) checkNamedTenant(ctx context.Context, scope core.TenantScope, named string) error {
	named = strings.TrimSpace(named)
	if named == "" {
		return nil
	}
	var tenant *core.Tenant
	if err := l.store.View(ctx, scope, func(tx store.Tx) error {
		var err error
		tenant, err = tx.GetTenant(ctx)
		return err
	}); err != nil {
		return err
	}
	if !strings.EqualFold(tenant.Key, named) && tenant.ID != named {
		return core.Invalid("this server signs in to tenant %q, not %q; use that tenant's address",
			tenant.Key, named)
	}
	return nil
}

// enrolledPerson reads this tenant's enrolment of a fingerprint and the person
// it speaks for. Only this tenant is read: the one cross-tenant lookup belongs
// to the SSH listener, which has no tenant until it has narrowed to one.
func (l *Local) enrolledPerson(ctx context.Context, scope core.TenantScope, fingerprint string,
) (*core.SSHKey, candidate, error) {
	var (
		key  *core.SSHKey
		cand = candidate{tenant: scope.TenantID}
	)
	err := l.store.View(ctx, scope, func(tx store.Tx) error {
		found, err := tx.GetSSHKeyByFingerprint(ctx, fingerprint)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		key = found
		actor, err := tx.GetActor(ctx, found.ActorID)
		if err != nil {
			return err
		}
		if actor.Kind != core.ActorUser {
			cand.actor = actor
			return nil
		}
		user, err := tx.GetUser(ctx, actor.ID)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		role, err := userRole(ctx, tx, actor.ID)
		if err != nil {
			return err
		}
		cand.actor, cand.user, cand.role = actor, user, role
		return nil
	})
	if err != nil {
		return nil, cand, err
	}
	switch {
	case key == nil:
		return nil, cand, core.Unauthenticated(
			"this key (%s) is not enrolled in this tenant: ask an operator to enrol it with `tix user key add`",
			fingerprint).WithDetail("reason", core.ReasonSSHKeyNotEnrolled)
	case !key.Active():
		return nil, cand, core.Unauthenticated("this key (%s) was revoked; enrol a new key", fingerprint).
			WithDetail("reason", core.ReasonSSHKeyRevoked)
	case cand.actor != nil && cand.user == nil:
		return nil, cand, core.Unauthenticated(
			"this key belongs to an agent; an agent authenticates with an API token, see tix token create")
	case cand.user == nil || cand.user.DisabledAt != nil:
		return nil, cand, errInvalidCredentials()
	}
	return key, cand, nil
}

// claimNonce spends the challenge for this key before a second factor is asked
// for, so one signature cannot be replayed into a second ticket.
func (l *Local) claimNonce(ctx context.Context, actor *core.Actor, key *core.SSHKey, issued time.Time) error {
	claimed := false
	if err := l.write(ctx, actor, func(m *mutation) error {
		var err error
		claimed, err = claim(ctx, m.tx, key, issued, m.now)
		return err
	}); err != nil {
		return err
	}
	if !claimed {
		return errNonceUsed()
	}
	return nil
}

// claim records the use at the later of now and the issue time, so a nonce
// from a replica whose clock runs ahead still ends at or below the stored use.
func claim(ctx context.Context, tx store.Tx, key *core.SSHKey, issued, now time.Time) (bool, error) {
	at := now
	if issued.After(at) {
		at = issued
	}
	return tx.ClaimSSHKeyUse(ctx, key.ID, issued, at)
}
