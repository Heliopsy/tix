// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// Event types for the second factor.
const (
	eventTOTPEnrolled      core.EventType = "user.totp.enrolled"
	eventTOTPConfirmed     core.EventType = "user.totp.confirmed"
	eventTOTPRemoved       core.EventType = "user.totp.removed"
	eventTOTPCleared       core.EventType = "user.totp.cleared"
	eventTOTPCodesIssued   core.EventType = "user.totp.codes_issued"
	eventTOTPRecoverySpent core.EventType = "user.totp.recovery_spent"
)

// Audit actions recorded for the second factor.
const (
	auditTOTPEnrol         = "user.totp.enrol"
	auditTOTPConfirm       = "user.totp.confirm"
	auditTOTPRemove        = "user.totp.remove"
	auditTOTPClear         = "user.totp.clear"
	auditTOTPCodesIssued   = "user.totp.codes_issued"
	auditTOTPRecoverySpent = "user.totp.recovery_spent"
)

// subjectTOTP is what an audit entry and an event call the thing that changed.
const subjectTOTP = "totp"

// defaultTOTPIssuer labels an authenticator entry when the tenant is unnamed.
const defaultTOTPIssuer = "tix"

// errNoTOTPKey refuses an enrolment that has nowhere safe to put its secret.
//
// There is deliberately no fallback to a plaintext column. A fallback is the
// kind of thing chosen once under time pressure and then run in production for
// two years, and the failure is silent: the feature appears to work and the
// property it exists for is gone.
func errNoTOTPKey() error {
	return core.Precondition(
		"no second factor key is configured; set auth.totp_key (TIX_AUTH_TOTP_KEY) to a 32 byte base64 value")
}

// sealer returns the configured sealer, or the reason there is none.
func (l *Local) sealer() (*auth.Sealer, error) {
	if l.totpSealer == nil {
		return nil, errNoTOTPKey()
	}
	return l.totpSealer, nil
}

// totpSelf resolves the caller to the global user their second factor belongs
// to.
//
// The test is the users row behind the actor, not the actor's kind. A caller
// presenting an API token reads as an agent whatever actor minted it, so a
// kind check would refuse a person managing their own factor through their
// own credential, while a purely automated actor has no users row and is
// refused here anyway. It concedes nothing a token did not already have: a
// token is exempt from being challenged at all, which is a wider allowance
// than being able to change the enrolment it bypasses.
func (l *Local) totpSelf(ctx context.Context) (*core.Actor, *core.User, error) {
	actor, err := core.RequireActor(ctx)
	if err != nil {
		return nil, nil, err
	}
	var user *core.User
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		var err error
		user, err = tx.GetUser(ctx, actor.ID)
		return err
	}); err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, nil, core.Invalid("a second factor belongs to a person, and this caller is not one")
		}
		return nil, nil, err
	}
	return actor, user, nil
}

// TOTPStatus reports the caller's enrolment and what the tenant requires.
func (l *Local) TOTPStatus(ctx context.Context) (*core.TOTPStatus, error) {
	actor, user, err := l.totpSelf(ctx)
	if err != nil {
		return nil, err
	}
	out := &core.TOTPStatus{}
	err = l.read(ctx, actor, func(tx store.Tx) error {
		tenant, err := tx.GetTenant(ctx)
		if err != nil {
			return err
		}
		out.RequiredWeb = tenant.RequireTOTPWeb
		out.RequiredCLI = tenant.RequireTOTPCLI
		out.RequiredSSH = tenant.RequireTOTPSSH

		enrolment, err := tx.GetUserTOTP(ctx, user.ID)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		out.Enrolled = enrolment.Confirmed()
		out.Pending = !enrolment.Confirmed()
		out.ConfirmedAt = enrolment.ConfirmedAt
		if enrolment.LockedUntil != nil && enrolment.LockedUntil.After(l.clock.Now()) {
			out.VerificationLocked = true
			out.VerificationLockEnd = enrolment.LockedUntil
		}
		codes, err := tx.ListRecoveryCodes(ctx, user.ID)
		if err != nil {
			return err
		}
		for _, c := range codes {
			if c.UsedAt == nil {
				out.RecoveryCodesLeft++
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// BeginTOTPEnrolment generates a secret and stores it unconfirmed.
func (l *Local) BeginTOTPEnrolment(ctx context.Context) (*core.TOTPEnrolment, error) {
	actor, user, err := l.totpSelf(ctx)
	if err != nil {
		return nil, err
	}
	out, err := l.beginEnrolment(ctx, actor, user)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// beginEnrolment is the body of BeginTOTPEnrolment, reached again from a
// sign-in that has to take an unenrolled person through enrolment rather than
// refuse them.
func (l *Local) beginEnrolment(ctx context.Context, actor *core.Actor, user *core.User) (*core.TOTPEnrolment, error) {
	sealer, err := l.sealer()
	if err != nil {
		return nil, err
	}
	secret, err := auth.NewTOTPSecret()
	if err != nil {
		return nil, err
	}
	cipher, nonce, err := sealer.Seal(user.ID, secret)
	if err != nil {
		return nil, err
	}

	issuer := defaultTOTPIssuer
	enrolment := &core.UserTOTP{UserID: user.ID, SecretCipher: cipher, SecretNonce: nonce}
	if err := l.write(ctx, actor, func(m *mutation) error {
		tenant, err := m.tx.GetTenant(ctx)
		if err != nil {
			return err
		}
		if name := strings.TrimSpace(tenant.Name); name != "" {
			issuer = name
		}
		// Replacing whatever was there is the point: re-enrolment has to stop
		// the previous secret being accepted, and an abandoned enrolment must
		// not block a second attempt.
		if err := m.tx.PutUserTOTP(ctx, enrolment); err != nil {
			return err
		}
		return m.Record(auditTOTPEnrol, eventTOTPEnrolled, subjectTOTP, user.ID, "", nil,
			map[string]any{"user_id": user.ID, "confirmed": false},
			map[string]any{"user_id": user.ID})
	}); err != nil {
		return nil, err
	}
	return &core.TOTPEnrolment{
		Secret:  secret,
		URI:     auth.TOTPURI(issuer, user.Email, secret),
		Issuer:  issuer,
		Account: user.Email,
	}, nil
}

// ConfirmTOTPEnrolment proves possession and issues the recovery codes.
func (l *Local) ConfirmTOTPEnrolment(ctx context.Context, code string) (*core.TOTPRecoveryCodes, error) {
	actor, user, err := l.totpSelf(ctx)
	if err != nil {
		return nil, err
	}
	var enrolment *core.UserTOTP
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		var err error
		enrolment, err = tx.GetUserTOTP(ctx, user.ID)
		return err
	}); err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.Precondition("no enrolment is in progress; begin one first")
		}
		return nil, err
	}
	if enrolment.Confirmed() {
		return nil, core.Precondition("this second factor is already confirmed")
	}

	attempt, err := l.verifyCode(ctx, actor, user, enrolment, code)
	if err != nil {
		return nil, err
	}

	codes, err := l.issueRecoveryCodes(ctx, actor, user, func(m *mutation) error {
		if err := m.tx.SaveUserTOTPAttempt(ctx, user.ID, attempt); err != nil {
			return err
		}
		if err := m.tx.ConfirmUserTOTP(ctx, user.ID, m.now); err != nil {
			return err
		}
		return m.Record(auditTOTPConfirm, eventTOTPConfirmed, subjectTOTP, user.ID, "",
			map[string]any{"user_id": user.ID, "confirmed": false},
			map[string]any{"user_id": user.ID, "confirmed": true},
			map[string]any{"user_id": user.ID})
	})
	if err != nil {
		return nil, err
	}
	return codes, nil
}

// RemoveTOTP drops the caller's own enrolment and every code beside it.
func (l *Local) RemoveTOTP(ctx context.Context) error {
	actor, user, err := l.totpSelf(ctx)
	if err != nil {
		return err
	}
	return l.clearEnrolment(ctx, actor, user.ID, auditTOTPRemove, eventTOTPRemoved)
}

// ClearUserTOTP returns another user to an unenrolled state.
func (l *Local) ClearUserTOTP(ctx context.Context, userID string) error {
	actor, err := l.authorize(ctx, authz.ActionTOTPAdmin, authz.Resource{})
	if err != nil {
		return err
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return core.Invalid("a user identifier is required")
	}
	// An actor of this tenant carries the user's identifier, so resolving one
	// is what keeps an administrator of one tenant from clearing the factor
	// of somebody who belongs only to another.
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		_, err := tx.GetActor(ctx, userID)
		return err
	}); err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return core.NotFound("user %q", userID)
		}
		return err
	}
	return l.clearEnrolment(ctx, actor, userID, auditTOTPClear, eventTOTPCleared)
}

// clearEnrolment removes one user's enrolment and recovery codes together.
func (l *Local) clearEnrolment(ctx context.Context, actor *core.Actor, userID, action string, event core.EventType) error {
	return l.write(ctx, actor, func(m *mutation) error {
		if err := m.tx.DeleteUserTOTP(ctx, userID); err != nil {
			if core.IsKind(err, core.KindNotFound) {
				return core.NotFound("no second factor enrolled for user %q", userID)
			}
			return err
		}
		// The recovery codes reference users rather than the enrolment row, so
		// nothing cascades them: a clear that forgot this would leave live
		// substitutes for a factor that no longer exists.
		if _, err := m.tx.DeleteRecoveryCodes(ctx, userID); err != nil {
			return err
		}
		return m.Record(action, event, subjectTOTP, userID, "",
			map[string]any{"user_id": userID, "enrolled": true},
			nil,
			map[string]any{"user_id": userID})
	})
}

// RegenerateRecoveryCodes replaces the caller's codes and returns them once.
func (l *Local) RegenerateRecoveryCodes(ctx context.Context) (*core.TOTPRecoveryCodes, error) {
	actor, user, err := l.totpSelf(ctx)
	if err != nil {
		return nil, err
	}
	var enrolment *core.UserTOTP
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		var err error
		enrolment, err = tx.GetUserTOTP(ctx, user.ID)
		return err
	}); err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.Precondition("no second factor is enrolled, so there are no codes to replace")
		}
		return nil, err
	}
	if !enrolment.Confirmed() {
		return nil, core.Precondition("this second factor is not confirmed yet")
	}
	return l.issueRecoveryCodes(ctx, actor, user, nil)
}

// issueRecoveryCodes mints a set, writes it in one transaction along with
// whatever else the caller wants recorded, and returns the codes to show once.
func (l *Local) issueRecoveryCodes(ctx context.Context, actor *core.Actor, user *core.User,
	also func(*mutation) error,
) (*core.TOTPRecoveryCodes, error) {
	plain, hashes, err := auth.NewRecoveryCodes()
	if err != nil {
		return nil, err
	}
	rows := make([]core.RecoveryCode, 0, len(hashes))
	for _, h := range hashes {
		rows = append(rows, core.RecoveryCode{UserID: user.ID, CodeHash: h})
	}
	if err := l.write(ctx, actor, func(m *mutation) error {
		if also != nil {
			if err := also(m); err != nil {
				return err
			}
		}
		if err := m.tx.ReplaceRecoveryCodes(ctx, user.ID, rows); err != nil {
			return err
		}
		return m.Record(auditTOTPCodesIssued, eventTOTPCodesIssued, subjectTOTP, user.ID, "", nil,
			map[string]any{"user_id": user.ID, "codes": len(rows)},
			map[string]any{"user_id": user.ID, "codes": len(rows)})
	}); err != nil {
		return nil, err
	}
	return &core.TOTPRecoveryCodes{Codes: plain}, nil
}

// verifyCode checks a TOTP code against an enrolment and returns the attempt
// state to write back on success.
//
// A rejection is persisted here and the error returned to the caller, because
// the write that records a failure cannot ride on the transaction that the
// failure is about to roll back. Without that the rate limit would count
// nothing: every failed attempt would undo its own tally.
func (l *Local) verifyCode(ctx context.Context, actor *core.Actor, user *core.User,
	enrolment *core.UserTOTP, code string,
) (core.UserTOTP, error) {
	sealer, err := l.sealer()
	if err != nil {
		return core.UserTOTP{}, err
	}
	secret, err := sealer.Open(user.ID, enrolment.SecretCipher, enrolment.SecretNonce)
	if err != nil {
		return core.UserTOTP{}, err
	}
	attempt := auth.Attempt{
		LastStep:    enrolment.LastStep,
		Failures:    enrolment.Failures,
		LockedUntil: enrolment.LockedUntil,
	}
	verifyErr := attempt.Verify(secret, code, l.clock.Now())
	state := core.UserTOTP{
		LastStep:    attempt.LastStep,
		Failures:    attempt.Failures,
		LockedUntil: attempt.LockedUntil,
	}
	if verifyErr != nil {
		l.recordFailedAttempt(ctx, actor, user.ID, state)
		return core.UserTOTP{}, verifyErr
	}
	return state, nil
}

// recordFailedAttempt persists the tally a rejection left. It is best effort:
// a refusal that could not be counted is still a refusal, and turning a wrong
// code into a storage error would tell the caller something about the account
// that a wrong code must not.
func (l *Local) recordFailedAttempt(ctx context.Context, actor *core.Actor, userID string, state core.UserTOTP) {
	_ = l.store.Update(ctx, core.TenantScope{TenantID: actor.TenantID}, func(tx store.Tx) error {
		return tx.SaveUserTOTPAttempt(ctx, userID, state)
	})
}

// spendRecoveryCode marks a submitted recovery code used, reporting which one
// so the audit trail can name it. It answers false when nothing matched.
func (l *Local) spendRecoveryCode(ctx context.Context, m *mutation, userID, submitted string, now time.Time) (string, bool, error) {
	if auth.NormalizeRecoveryCode(submitted) == "" {
		return "", false, nil
	}
	codes, err := m.tx.ListRecoveryCodes(ctx, userID)
	if err != nil {
		return "", false, err
	}
	unused := make([]core.RecoveryCode, 0, len(codes))
	hashes := make([]string, 0, len(codes))
	for _, c := range codes {
		if c.UsedAt != nil {
			continue
		}
		unused = append(unused, c)
		hashes = append(hashes, c.CodeHash)
	}
	index, found := auth.MatchRecoveryCode(hashes, submitted)
	if !found {
		return "", false, nil
	}
	spent, err := m.tx.SpendRecoveryCode(ctx, unused[index].ID, now)
	if err != nil {
		return "", false, err
	}
	return unused[index].ID, spent, nil
}
