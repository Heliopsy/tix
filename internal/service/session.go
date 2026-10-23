// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// Event types for the session lifecycle.
const (
	eventSessionCreated core.EventType = "session.created"
	eventSessionEnded   core.EventType = "session.ended"
)

// Audit actions recorded for sessions.
const (
	auditUserLogin  = "user.login"
	auditUserLogout = "user.logout"
)

// SessionTTL is how long a password login stays valid.
const SessionTTL = 12 * time.Hour

// errInvalidCredentials is the single answer every failed login gives, so that
// a wrong password, an unknown email and a disabled account are one outcome.
func errInvalidCredentials() error { return core.Unauthenticated("invalid credentials") }

// dummyHash is verified against when no account matches, so that a login for an
// address nobody holds costs the same as a login with the wrong password. It is
// per service because the hasher's cost is configurable.
func (l *Local) dummyHash() string {
	l.dummyOnce.Do(func() {
		h, err := l.hasher.Hash(strings.Repeat("tix-absent-", 4))
		if err == nil {
			l.dummy = h
		}
	})
	return l.dummy
}

// sessionCtxKey carries the session token a caller presented.
type sessionCtxKey struct{}

// WithSessionToken returns a context carrying the session token of the caller.
func WithSessionToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, sessionCtxKey{}, token)
}

// SessionTokenFrom returns the session token carried by ctx, if any.
func SessionTokenFrom(ctx context.Context) (string, bool) {
	t, ok := ctx.Value(sessionCtxKey{}).(string)
	return t, ok && t != ""
}

// candidate is what a login attempt resolved, all of it optional so that a
// failure carries no information about which part was missing.
type candidate struct {
	user   *core.User
	hash   string
	actor  *core.Actor
	role   core.Role
	tenant string
}

// findCandidate resolves the account an email names within one tenant. A user
// without an actor in this tenant is no candidate, which is what keeps login
// tenant-scoped even though user rows are global.
func (l *Local) findCandidate(ctx context.Context, scope core.TenantScope, email string) (candidate, error) {
	out := candidate{tenant: scope.TenantID}
	if email == "" {
		return out, nil
	}
	var (
		user *core.User
		hash string
	)
	if err := l.store.Unscoped(ctx, func(u store.UnscopedTx) error {
		found, h, err := u.GetUserByEmail(ctx, email)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		user, hash = found, h
		return nil
	}); err != nil {
		return out, err
	}
	if user == nil {
		return out, nil
	}

	system := core.SystemActor(scope.TenantID)
	if err := l.read(ctx, system, func(tx store.Tx) error {
		a, err := tx.GetActor(ctx, user.ID)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		role, err := userRole(ctx, tx, a.ID)
		if err != nil {
			return err
		}
		out.actor, out.role = a, role
		return nil
	}); err != nil {
		return out, err
	}
	if out.actor == nil {
		return out, nil
	}
	out.user, out.hash = user, hash
	return out, nil
}

// usable reports whether the candidate may be logged in with password.
func (c candidate) usable() bool {
	return c.user != nil && c.actor != nil && c.user.DisabledAt == nil && c.hash != ""
}

// Login is one step of an interactive sign-in.
//
// The second-factor requirement is read here, at the moment a session is
// minted, against the tenant the session is for. It is deliberately not
// re-read on each subsequent request: a minted session has already satisfied
// the policy, and re-checking would mean a flag flipped mid-session logs
// everybody out, which is a different feature.
func (l *Local) Login(ctx context.Context, in core.LoginInput) (*core.LoginResult, error) {
	scope, err := core.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	surface := in.Surface
	if surface == "" {
		// An unspecified surface is the command line. The alternative, a
		// surface that no requirement is written against, would let a caller
		// omit the field and walk past the policy; defaulting to one that can
		// be required fails in the safe direction.
		surface = core.SurfaceCLI
	}
	if !surface.Valid() {
		return nil, core.Invalid("%q is not a sign-in surface", in.Surface)
	}
	if in.Ticket != "" {
		return l.completeLogin(ctx, scope, surface, in)
	}
	return l.beginLogin(ctx, scope, surface, in)
}

// beginLogin is the password step.
func (l *Local) beginLogin(ctx context.Context, scope core.TenantScope, surface core.Surface,
	in core.LoginInput,
) (*core.LoginResult, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))

	cand, err := l.findCandidate(ctx, scope, email)
	if err != nil {
		return nil, err
	}
	stored := l.dummyHash()
	if cand.hash != "" {
		stored = cand.hash
	}
	verified := auth.Verify(stored, in.Password) == nil
	if !verified || !cand.usable() {
		return nil, errInvalidCredentials()
	}

	rehashed := ""
	if auth.NeedsRehash(cand.hash, l.hasher.Params()) {
		if rehashed, err = l.hasher.Hash(in.Password); err != nil {
			return nil, err
		}
	}
	sessionActor := cand.sessionActor(scope)

	challenge, err := l.secondFactorFor(ctx, scope, surface, sessionActor, cand.user)
	if err != nil {
		return nil, err
	}
	if challenge != nil {
		// The password rehash waits for the sign-in to finish. Writing it
		// here would be a write on a path that may never produce a session.
		return &core.LoginResult{Challenge: challenge}, nil
	}
	return l.mintSession(ctx, scope, sessionActor, cand.user, rehashed, nil)
}

// secondFactorFor decides what, if anything, stands between this person and a
// session. It answers nil when the sign-in may complete now.
//
// A code is asked for when the tenant requires one on this surface, and also
// when the person holds a confirmed enrolment of their own. The second half
// is what makes enrolment worth doing before an administrator turns the flag
// on: without it a user who enrolled voluntarily would be asked for nothing,
// and the factor they set up would protect no sign-in at all.
func (l *Local) secondFactorFor(ctx context.Context, scope core.TenantScope, surface core.Surface,
	actor *core.Actor, user *core.User,
) (*core.TOTPChallenge, error) {
	var (
		required bool
		enrolled bool
	)
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		tenant, err := tx.GetTenant(ctx)
		if err != nil {
			return err
		}
		required = tenant.RequiresTOTP(surface)
		enrolment, err := tx.GetUserTOTP(ctx, user.ID)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		enrolled = enrolment.Confirmed()
		return nil
	}); err != nil {
		return nil, err
	}
	if !required && !enrolled {
		return nil, nil
	}

	ticket, err := auth.MintTicket(l.totpKey, auth.Ticket{
		UserID:    user.ID,
		TenantID:  scope.TenantID,
		Surface:   surface,
		ExpiresAt: l.clock.Now().Add(auth.TicketTTL),
	})
	if err != nil {
		return nil, err
	}
	if enrolled {
		return &core.TOTPChallenge{Kind: core.ChallengeCode, Ticket: ticket}, nil
	}
	// An unenrolled person on a requiring surface is taken through enrolment
	// rather than refused. Refusing would make enabling the requirement lock
	// out every user, the administrator who enabled it first among them.
	enrolment, err := l.beginEnrolment(ctx, actor, user)
	if err != nil {
		return nil, err
	}
	return &core.TOTPChallenge{Kind: core.ChallengeEnrol, Ticket: ticket, Enrolment: enrolment}, nil
}

// completeLogin is the code step. The ticket proves the password was accepted
// a moment ago and carries no authority of its own.
func (l *Local) completeLogin(ctx context.Context, scope core.TenantScope, surface core.Surface,
	in core.LoginInput,
) (*core.LoginResult, error) {
	ticket, err := auth.ParseTicket(l.totpKey, in.Ticket, l.clock.Now())
	if err != nil {
		return nil, err
	}
	if ticket.TenantID != scope.TenantID || ticket.Surface != surface {
		return nil, core.Unauthenticated("this sign-in has expired; start again")
	}

	var (
		user      *core.User
		cand      candidate
		enrolment *core.UserTOTP
	)
	system := core.SystemActor(scope.TenantID)
	if err := l.read(ctx, system, func(tx store.Tx) error {
		var err error
		if user, err = tx.GetUser(ctx, ticket.UserID); err != nil {
			return err
		}
		actor, err := tx.GetActor(ctx, ticket.UserID)
		if err != nil {
			return err
		}
		role, err := userRole(ctx, tx, actor.ID)
		if err != nil {
			return err
		}
		cand = candidate{user: user, actor: actor, role: role, tenant: scope.TenantID}
		enrolment, err = tx.GetUserTOTP(ctx, ticket.UserID)
		return err
	}); err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, errInvalidCredentials()
		}
		return nil, err
	}
	if user.DisabledAt != nil {
		return nil, errInvalidCredentials()
	}
	sessionActor := cand.sessionActor(scope)

	pending := !enrolment.Confirmed()
	attempt, verifyErr := l.verifyCode(ctx, sessionActor, user, enrolment, in.Code)
	if verifyErr != nil {
		if core.IsTOTPLocked(verifyErr) || pending {
			// A pending enrolment has no recovery codes to fall back on, and
			// a locked enrolment accepts nothing at all, so neither gets the
			// extra round trip a recovery attempt costs.
			return nil, verifyErr
		}
		return l.loginWithRecoveryCode(ctx, scope, sessionActor, user, in.Code, verifyErr)
	}

	if !pending {
		return l.mintSession(ctx, scope, sessionActor, user, "", func(m *mutation) error {
			return m.tx.SaveUserTOTPAttempt(ctx, user.ID, attempt)
		})
	}

	// The sign-in completed by enrolling, so it also issues the recovery
	// codes. They are returned here and nowhere else: nothing can produce
	// them again, and a caller that discards them has discarded them.
	plain, hashes, err := auth.NewRecoveryCodes()
	if err != nil {
		return nil, err
	}
	rows := make([]core.RecoveryCode, 0, len(hashes))
	for _, h := range hashes {
		rows = append(rows, core.RecoveryCode{UserID: user.ID, CodeHash: h})
	}
	out, err := l.mintSession(ctx, scope, sessionActor, user, "", func(m *mutation) error {
		if err := m.tx.SaveUserTOTPAttempt(ctx, user.ID, attempt); err != nil {
			return err
		}
		if err := m.tx.ConfirmUserTOTP(ctx, user.ID, m.now); err != nil {
			return err
		}
		if err := m.Record(auditTOTPConfirm, eventTOTPConfirmed, subjectTOTP, user.ID, "",
			map[string]any{"user_id": user.ID, "confirmed": false},
			map[string]any{"user_id": user.ID, "confirmed": true},
			map[string]any{"user_id": user.ID}); err != nil {
			return err
		}
		if err := m.tx.ReplaceRecoveryCodes(ctx, user.ID, rows); err != nil {
			return err
		}
		return m.Record(auditTOTPCodesIssued, eventTOTPCodesIssued, subjectTOTP, user.ID, "", nil,
			map[string]any{"user_id": user.ID, "codes": len(rows)},
			map[string]any{"user_id": user.ID, "codes": len(rows)})
	})
	if err != nil {
		return nil, err
	}
	out.RecoveryCodes = plain
	return out, nil
}

// loginWithRecoveryCode is the fallback when a submitted value is not a TOTP
// code. A recovery code is accepted anywhere a code is, and the one that was
// spent is named in the audit trail.
func (l *Local) loginWithRecoveryCode(ctx context.Context, scope core.TenantScope, actor *core.Actor,
	user *core.User, submitted string, codeErr error,
) (*core.LoginResult, error) {
	result, err := l.mintSessionIf(ctx, scope, actor, func(m *mutation) (bool, error) {
		id, spent, err := l.spendRecoveryCode(ctx, m, user.ID, submitted, m.now)
		if err != nil {
			return false, err
		}
		if !spent {
			return false, nil
		}
		// A code that worked clears the failure the TOTP attempt just
		// recorded: the limit exists to stop guessing, and this was not one.
		if err := m.tx.SaveUserTOTPAttempt(ctx, user.ID, core.UserTOTP{}); err != nil {
			return false, err
		}
		return true, m.Record(auditTOTPRecoverySpent, eventTOTPRecoverySpent, subjectTOTP, user.ID, "",
			map[string]any{"user_id": user.ID, "recovery_code_id": id, "used": false},
			map[string]any{"user_id": user.ID, "recovery_code_id": id, "used": true},
			map[string]any{"user_id": user.ID, "recovery_code_id": id})
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, codeErr
	}
	return result, nil
}

// sessionActor builds the actor a minted session runs as.
func (c candidate) sessionActor(scope core.TenantScope) *core.Actor {
	return &core.Actor{
		ID:          c.actor.ID,
		TenantID:    scope.TenantID,
		Kind:        core.ActorUser,
		Handle:      c.actor.Handle,
		DisplayName: c.actor.DisplayName,
		Role:        c.role,
	}
}

// mintSession writes the session, the optional password rehash and whatever
// else the sign-in has to record, in one transaction.
func (l *Local) mintSession(ctx context.Context, scope core.TenantScope, actor *core.Actor,
	user *core.User, rehashed string, also func(*mutation) error,
) (*core.LoginResult, error) {
	out, err := l.mintSessionIf(ctx, scope, actor, func(m *mutation) (bool, error) {
		if rehashed != "" {
			if err := m.tx.UpdateUser(ctx, user, rehashed); err != nil {
				return false, err
			}
		}
		if also == nil {
			return true, nil
		}
		return true, also(m)
	})
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, errInvalidCredentials()
	}
	return out, nil
}

// mintSessionIf mints a session only when also reports the sign-in may
// complete, so a check that has to run inside the transaction can refuse
// without leaving a session behind. It answers nil, nil when it refused.
func (l *Local) mintSessionIf(ctx context.Context, scope core.TenantScope, actor *core.Actor,
	also func(*mutation) (bool, error),
) (*core.LoginResult, error) {
	minted, err := auth.MintSession(l.clock, auth.SessionInput{
		ActorID:  actor.ID,
		TenantID: scope.TenantID,
		Handle:   actor.Handle,
		Role:     actor.Role,
		TTL:      SessionTTL,
	})
	if err != nil {
		return nil, err
	}
	ok := false
	if err := l.write(ctx, actor, func(m *mutation) error {
		if ok, err = also(m); err != nil || !ok {
			return err
		}
		if err := m.tx.CreateSession(ctx, actor.ID, minted.Stored.TokenHash, minted.Session.ExpiresAt); err != nil {
			return err
		}
		return m.Record(auditUserLogin, eventSessionCreated, "session", actor.ID, "", nil,
			map[string]any{"actor_id": actor.ID, "expires_at": minted.Session.ExpiresAt},
			map[string]any{"actor_id": actor.ID, "handle": actor.Handle})
	}); err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return &core.LoginResult{Session: &minted.Session}, nil
}

// Logout ends the session the caller presented. It needs no authorization
// beyond being authenticated, since an actor may always end its own session.
func (l *Local) Logout(ctx context.Context) error {
	actor, err := core.RequireActor(ctx)
	if err != nil {
		return err
	}
	token, ok := SessionTokenFrom(ctx)
	if !ok {
		return core.Unauthenticated("no session to end")
	}
	hash := auth.HashToken(token)

	return l.write(ctx, actor, func(m *mutation) error {
		if err := m.tx.DeleteSession(ctx, hash); err != nil {
			if core.IsKind(err, core.KindNotFound) {
				return core.Unauthenticated("no session to end")
			}
			return err
		}
		return m.Record(auditUserLogout, eventSessionEnded, "session", actor.ID, "",
			map[string]any{"actor_id": actor.ID}, nil, map[string]any{"actor_id": actor.ID})
	})
}
