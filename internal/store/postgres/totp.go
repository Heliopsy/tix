// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
)

var totpColumns = []string{
	"user_id", "secret_cipher", "secret_nonce", "confirmed_at",
	"last_step", "failures", "locked_until", "created_at", "updated_at",
}

func scanUserTOTP(s scanner) (core.UserTOTP, error) {
	var (
		e         core.UserTOTP
		confirmed sql.NullTime
		locked    sql.NullTime
		created   sql.NullTime
		updated   sql.NullTime
	)
	if err := s.Scan(&e.UserID, &e.SecretCipher, &e.SecretNonce, &confirmed,
		&e.LastStep, &e.Failures, &locked, &created, &updated); err != nil {
		return core.UserTOTP{}, mapErr(err, "scanning totp enrolment")
	}
	e.ConfirmedAt = scanNullTime(confirmed)
	e.LockedUntil = scanNullTime(locked)
	e.CreatedAt = scanTime(created)
	e.UpdatedAt = scanTime(updated)
	return e, nil
}

// GetUserTOTP returns a user's enrolment, confirmed or not.
func (t *tx) GetUserTOTP(ctx context.Context, userID string) (*core.UserTOTP, error) {
	b := t.builder("user_totp").Select(totpColumns...).Where("user_id = ?", userID).Limit(1)
	q, args := b.SelectQuery()
	e, err := scanUserTOTP(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("no second factor enrolled for user %q", userID)
		}
		return nil, err
	}
	return &e, nil
}

// PutUserTOTP writes an enrolment, replacing whatever the user held.
func (t *tx) PutUserTOTP(ctx context.Context, e *core.UserTOTP) error {
	if e.UserID == "" {
		return core.Invalid("a totp enrolment needs the user it belongs to")
	}
	now := t.now()
	if e.CreatedAt.IsZero() {
		e.CreatedAt = now
	}
	e.UpdatedAt = now
	if err := t.DeleteUserTOTP(ctx, e.UserID); err != nil && !core.IsKind(err, core.KindNotFound) {
		return err
	}
	ins := t.insert("user_totp").
		Set("user_id", e.UserID).
		Set("secret_cipher", e.SecretCipher).
		Set("secret_nonce", e.SecretNonce).
		Set("confirmed_at", nullTimeArg(e.ConfirmedAt)).
		Set("last_step", e.LastStep).
		Set("failures", e.Failures).
		Set("locked_until", nullTimeArg(e.LockedUntil)).
		Set("created_at", timeArg(e.CreatedAt)).
		Set("updated_at", timeArg(e.UpdatedAt))
	_, err := t.execInsert(ctx, ins, "enrolling a second factor for user %q", e.UserID)
	return err
}

// SaveUserTOTPAttempt writes back what a verification attempt left behind.
func (t *tx) SaveUserTOTPAttempt(ctx context.Context, userID string, a core.UserTOTP) error {
	b := t.builder("user_totp").
		Where("user_id = ?", userID).
		Set("last_step", a.LastStep).
		Set("failures", a.Failures).
		Set("locked_until", nullTimeArg(a.LockedUntil)).
		Set("updated_at", timeArg(t.now()))
	n, err := t.execUpdate(ctx, b, "recording a totp attempt for user %q", userID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("no second factor enrolled for user %q", userID)
	}
	return nil
}

// ConfirmUserTOTP marks an enrolment proved.
func (t *tx) ConfirmUserTOTP(ctx context.Context, userID string, at time.Time) error {
	b := t.builder("user_totp").
		Where("user_id = ?", userID).
		Set("confirmed_at", timeArg(at)).
		Set("updated_at", timeArg(at))
	n, err := t.execUpdate(ctx, b, "confirming the second factor of user %q", userID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("no second factor enrolled for user %q", userID)
	}
	return nil
}

// DeleteUserTOTP removes an enrolment.
func (t *tx) DeleteUserTOTP(ctx context.Context, userID string) error {
	b := t.builder("user_totp").Where("user_id = ?", userID)
	n, err := t.execDelete(ctx, b, "removing the second factor of user %q", userID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("no second factor enrolled for user %q", userID)
	}
	return nil
}

// CountConfirmedTOTP counts confirmed enrolments across the installation.
func (t *tx) CountConfirmedTOTP(ctx context.Context) (int, error) {
	b := t.builder("user_totp").Where("confirmed_at IS NOT NULL")
	return t.count(ctx, b, "counting confirmed second factors")
}

var recoveryColumns = []string{"id", "user_id", "code_hash", "created_at", "used_at"}

func scanRecoveryCode(s scanner) (core.RecoveryCode, error) {
	var (
		c       core.RecoveryCode
		created sql.NullTime
		used    sql.NullTime
	)
	if err := s.Scan(&c.ID, &c.UserID, &c.CodeHash, &created, &used); err != nil {
		return core.RecoveryCode{}, mapErr(err, "scanning recovery code")
	}
	c.CreatedAt = scanTime(created)
	c.UsedAt = scanNullTime(used)
	return c, nil
}

// ReplaceRecoveryCodes drops every code the user holds and writes the new set.
func (t *tx) ReplaceRecoveryCodes(ctx context.Context, userID string, codes []core.RecoveryCode) error {
	if userID == "" {
		return core.Invalid("recovery codes need the user they belong to")
	}
	if _, err := t.DeleteRecoveryCodes(ctx, userID); err != nil {
		return err
	}
	now := t.now()
	for i := range codes {
		c := &codes[i]
		c.UserID = userID
		if c.ID == "" {
			c.ID = id.New()
		}
		if c.CreatedAt.IsZero() {
			c.CreatedAt = now
		}
		ins := t.insert("user_recovery_codes").
			Set("id", c.ID).
			Set("user_id", c.UserID).
			Set("code_hash", c.CodeHash).
			Set("created_at", timeArg(c.CreatedAt)).
			Set("used_at", nullTimeArg(c.UsedAt))
		if _, err := t.execInsert(ctx, ins, "issuing recovery codes for user %q", userID); err != nil {
			return err
		}
	}
	return nil
}

// ListRecoveryCodes returns a user's codes, spent ones included, oldest first.
func (t *tx) ListRecoveryCodes(ctx context.Context, userID string) ([]core.RecoveryCode, error) {
	b := t.builder("user_recovery_codes").
		Select(recoveryColumns...).
		Where("user_id = ?", userID).
		OrderBy("created_at", core.Ascending).
		OrderBy("id", core.Ascending)
	rows, err := t.query(ctx, b, "listing recovery codes")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.RecoveryCode{}
	for rows.Next() {
		v, err := scanRecoveryCode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, mapRowsErr(rows, "listing recovery codes")
}

// SpendRecoveryCode marks one code used, and reports whether it was unused.
//
// The predicate re-asserts used_at IS NULL inside the statement rather than
// trusting the read that chose this code: without it two attempts arriving on
// one code would both be told they spent it.
func (t *tx) SpendRecoveryCode(ctx context.Context, codeID string, at time.Time) (bool, error) {
	b := t.builder("user_recovery_codes").
		Where("id = ?", codeID).
		Where("used_at IS NULL").
		Set("used_at", timeArg(at))
	n, err := t.execUpdate(ctx, b, "spending recovery code %q", codeID)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// DeleteRecoveryCodes removes every code a user holds.
func (t *tx) DeleteRecoveryCodes(ctx context.Context, userID string) (int64, error) {
	b := t.builder("user_recovery_codes").Where("user_id = ?", userID)
	return t.execDelete(ctx, b, "removing the recovery codes of user %q", userID)
}
