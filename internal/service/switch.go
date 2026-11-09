// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// ReasonSecondFactorRequired is the detail a refused switch carries when the
// target tenant requires a second factor the account has not set up.
const ReasonSecondFactorRequired = "second_factor_required"

// SwitchTenant moves the caller's browser session to another tenant they are a
// member of, served on the same host. It mints a session for the person's
// actor in that tenant and ends the one presented. When the target requires a
// second factor on the surface and the current session was minted without
// one, it answers with a code challenge and mints nothing until the code is
// sent again with the request. A tenant the person is not a member of reads
// as not found, like one that does not exist. The switch is recorded in the
// target as a sign-in with method switch, and nothing is recorded in the
// tenant left.
func (l *Local) SwitchTenant(ctx context.Context, in core.SwitchTenantInput) (*core.LoginResult, error) {
	actor, err := core.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	token, ok := SessionTokenFrom(ctx)
	if !ok || actor.SessionID == "" || actor.TokenID != "" {
		return nil, core.Invalid("only a browser session can switch tenant; a token stays in its own tenant")
	}
	surface := in.Surface
	if surface == "" {
		surface = core.SurfaceWeb
	}
	if !surface.Valid() {
		return nil, core.Invalid("%q is not a sign-in surface", in.Surface)
	}
	key := strings.ToLower(strings.TrimSpace(in.Tenant))
	if key == "" {
		return nil, core.Invalid("tenant is required").WithDetail(core.DetailField, "tenant")
	}
	host, _ := core.ServedHostFrom(ctx)

	target, err := l.switchTarget(ctx, actor, host, key)
	if err != nil {
		return nil, err
	}
	scope := core.TenantScope{TenantID: target.tenant}
	sessionActor := target.sessionActor(scope)
	hash := auth.HashToken(token)

	origin, required, enrolment, err := l.switchFactor(ctx, actor, sessionActor, target.user, surface, hash)
	if err != nil {
		return nil, err
	}
	var attempt *core.UserTOTP
	if required && !origin.SecondFactor {
		if !enrolment.Confirmed() {
			return nil, core.Forbidden("tenant %q requires a second factor; set one up under Settings, then switch", key).
				WithDetail("reason", ReasonSecondFactorRequired)
		}
		if strings.TrimSpace(in.Code) == "" {
			return &core.LoginResult{Challenge: &core.TOTPChallenge{Kind: core.ChallengeCode}}, nil
		}
		state, verifyErr := l.verifyCode(ctx, sessionActor, target.user, enrolment, in.Code)
		if verifyErr != nil {
			if core.IsTOTPLocked(verifyErr) {
				return nil, verifyErr
			}
			return l.switchWithRecoveryCode(ctx, scope, actor, sessionActor, target.user, hash, in.Code, verifyErr)
		}
		attempt = &state
		origin.SecondFactor = true
	}

	minted, err := l.mintSession(switchContext(ctx), scope, sessionActor, target.user, "",
		core.SessionOrigin{Method: core.LoginMethodSwitch, SecondFactor: origin.SecondFactor},
		func(m *mutation) error {
			if attempt == nil {
				return nil
			}
			return m.tx.SaveUserTOTPAttempt(ctx, target.user.ID, *attempt)
		})
	if err != nil {
		return nil, err
	}
	if err := l.endSwitchedSession(ctx, actor, hash); err != nil {
		return nil, err
	}
	return minted, nil
}

// switchContext is the request's context with the presented session's
// reference taken off its access channel, so the target's audit entry cannot
// be tied back to a session of the tenant left.
func switchContext(ctx context.Context) context.Context {
	via := core.ViaFrom(ctx)
	via.Session = ""
	return core.WithVia(ctx, via)
}

// switchTarget finds the caller's membership in the tenant a key names and
// the person's actor there. Every way of not being a live member there reads
// as the same not found.
func (l *Local) switchTarget(ctx context.Context, actor *core.Actor, host core.ServedHost, key string) (candidate, error) {
	missing := core.NotFound("tenant %q", key)
	var account *core.User
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		u, err := accountOfActor(ctx, tx, actor.ID)
		account = u
		return err
	}); err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return candidate{}, missing
		}
		return candidate{}, err
	}
	if account.DisabledAt != nil {
		return candidate{}, missing
	}
	found, err := l.membershipsOf(ctx, account.ID)
	if err != nil {
		return candidate{}, err
	}
	var to *core.AccountMembership
	for i := range found {
		if found[i].TenantKey == key {
			to = &found[i]
		}
	}
	switch {
	case to == nil:
		return candidate{}, missing
	case to.TenantID == actor.TenantID:
		return candidate{}, core.Invalid("this session is already in tenant %q", key)
	case !host.Serves(to.TenantID, to.Host):
		if to.Host != "" {
			return candidate{}, core.Invalid("tenant %q is served at its own address, %s; sign in there", key, to.Host)
		}
		return candidate{}, core.Invalid("this address serves one tenant; tenant %q is reached at an address "+
			"that serves several", key)
	}

	cand, err := l.memberIn(ctx, account, to.TenantID)
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return candidate{}, missing
		}
		return candidate{}, err
	}
	if !cand.member() {
		return candidate{}, missing
	}
	return cand, nil
}

// switchFactor reads how the presented session was minted, whether the target
// requires a second factor on this surface, and the account's enrolment.
func (l *Local) switchFactor(ctx context.Context, actor, target *core.Actor, user *core.User,
	surface core.Surface, hash string,
) (core.SessionOrigin, bool, *core.UserTOTP, error) {
	var origin core.SessionOrigin
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		var err error
		origin, err = tx.GetSessionOrigin(ctx, hash)
		return err
	}); err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return origin, false, nil, core.Unauthenticated("no session to switch from")
		}
		return origin, false, nil, err
	}
	var (
		required  bool
		enrolment *core.UserTOTP
	)
	err := l.read(ctx, target, func(tx store.Tx) error {
		t, err := tx.GetTenant(ctx)
		if err != nil {
			return err
		}
		e, err := tx.GetUserTOTP(ctx, user.ID)
		if err != nil && !core.IsKind(err, core.KindNotFound) {
			return err
		}
		enrolment = e
		// Under the policy an enrolled person proves the factor to come in,
		// and an unenrolled one is refused once the grace is over; inside it
		// they come in as they would sign in, unasked.
		required = t.RequiresTOTP(surface) ||
			t.MFARequired() && (t.MFAEnforced(l.clock.Now()) || enrolment.Confirmed())
		return nil
	})
	return origin, required, enrolment, err
}

// switchWithRecoveryCode accepts a recovery code where the switch asked for a
// code, as sign-in does, and ends the presented session once the new one is
// minted.
func (l *Local) switchWithRecoveryCode(ctx context.Context, scope core.TenantScope, from, to *core.Actor,
	user *core.User, hash, submitted string, codeErr error,
) (*core.LoginResult, error) {
	result, err := l.loginWithRecoveryCode(switchContext(ctx), scope, to, user, core.LoginMethodSwitch, submitted, codeErr)
	if err != nil {
		return nil, err
	}
	if err := l.endSwitchedSession(ctx, from, hash); err != nil {
		return nil, err
	}
	return result, nil
}

// endSwitchedSession ends the session a switch left. It records nothing in
// the tenant left, so its administrators do not learn where the person went.
// The two tenants are two transactions, so it runs once the new session is
// written; when it fails the new session's token is never handed out, and the
// switch fails with the old session still the one in use.
func (l *Local) endSwitchedSession(ctx context.Context, from *core.Actor, hash string) error {
	return l.store.Update(ctx, core.TenantScope{TenantID: from.TenantID}, func(tx store.Tx) error {
		return tx.DeleteSession(ctx, hash)
	})
}
