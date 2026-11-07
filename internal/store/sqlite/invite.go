// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/heliopsy/tix/internal/core"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

var inviteColumns = []string{
	"id", "tenant_id", "user_id", "email", "role", "invited_by", "existing_account", "grants_membership",
	"created_at", "expires_at", "accepted_at", "revoked_at",
}

func scanInvite(s scanner, extra ...any) (*core.Invite, error) {
	var (
		inv      core.Invite
		existing int64
		grants   int64
		created  sql.NullString
		expires  sql.NullString
		accepted sql.NullString
		revoked  sql.NullString
	)
	dest := append([]any{&inv.ID, &inv.TenantID, &inv.UserID, &inv.Email, &inv.Role, &inv.InvitedBy,
		&existing, &grants, &created, &expires, &accepted, &revoked}, extra...)
	if err := s.Scan(dest...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, core.NotFound("invite not found")
		}
		return nil, mapErr(err, "scanning invite")
	}
	inv.ExistingAccount = sqlb.Bool(existing)
	inv.GrantsMembership = sqlb.Bool(grants)
	var err error
	if inv.CreatedAt, err = sqlb.ScanTime(created); err != nil {
		return nil, err
	}
	if inv.ExpiresAt, err = sqlb.ScanTime(expires); err != nil {
		return nil, err
	}
	if inv.AcceptedAt, err = sqlb.ScanNullTime(accepted); err != nil {
		return nil, err
	}
	if inv.RevokedAt, err = sqlb.ScanNullTime(revoked); err != nil {
		return nil, err
	}
	return &inv, nil
}

// CreateInvite stores an invite and the hash of its secret.
func (t *tx) CreateInvite(ctx context.Context, inv *core.Invite, tokenHash string) error {
	inv.TenantID = t.scope.TenantID
	ins := t.insert("user_invites").
		Set("id", inv.ID).
		Set("user_id", inv.UserID).
		Set("email", inv.Email).
		Set("role", string(inv.Role)).
		Set("invited_by", inv.InvitedBy).
		Set("existing_account", sqlb.BoolInt(inv.ExistingAccount)).
		Set("grants_membership", sqlb.BoolInt(inv.GrantsMembership)).
		Set("token_hash", tokenHash).
		Set("created_at", sqlb.TimeText(inv.CreatedAt)).
		Set("expires_at", sqlb.TimeText(inv.ExpiresAt))
	_, err := t.execInsert(ctx, ins, "creating invite for %q", inv.Email)
	return err
}

// GetInvite returns an invite by identifier.
func (t *tx) GetInvite(ctx context.Context, id string) (*core.Invite, error) {
	q, args := t.builder("user_invites").Select(inviteColumns...).Where("id = ?", id).Limit(1).SelectQuery()
	return scanInvite(t.ex.QueryRowContext(ctx, q, args...))
}

// InviteByHash returns the invite behind a secret's hash and the stored hash.
func (t *tx) InviteByHash(ctx context.Context, tokenHash string) (*core.Invite, string, error) {
	cols := append(append([]string{}, inviteColumns...), "token_hash")
	q, args := t.builder("user_invites").Select(cols...).Where("token_hash = ?", tokenHash).Limit(1).SelectQuery()
	var stored string
	inv, err := scanInvite(t.ex.QueryRowContext(ctx, q, args...), &stored)
	if err != nil {
		return nil, "", err
	}
	return inv, stored, nil
}

// PendingInviteFor returns a user's unaccepted, unrevoked invite.
func (t *tx) PendingInviteFor(ctx context.Context, userID string) (*core.Invite, error) {
	q, args := t.builder("user_invites").Select(inviteColumns...).Where("user_id = ?", userID).
		Where("accepted_at IS NULL").Where("revoked_at IS NULL").Limit(1).SelectQuery()
	return scanInvite(t.ex.QueryRowContext(ctx, q, args...))
}

// ListPendingInvites returns the unaccepted, unrevoked invites, oldest first.
func (t *tx) ListPendingInvites(ctx context.Context) ([]core.Invite, error) {
	b := t.builder("user_invites").Select(inviteColumns...).
		Where("accepted_at IS NULL").Where("revoked_at IS NULL").
		OrderBy("created_at", core.Ascending).OrderBy("id", core.Ascending).Limit(core.MaxPageLimit)
	rows, err := t.query(ctx, b, "listing invites")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []core.Invite{}
	for rows.Next() {
		inv, err := scanInvite(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *inv)
	}
	return out, mapRowsErr(rows, "listing invites")
}

// AcceptInvite marks a pending invite used from the given instant.
func (t *tx) AcceptInvite(ctx context.Context, id string, at time.Time) error {
	return t.closeInvite(ctx, id, "accepted_at", at, "accepting invite %q")
}

// RevokeInvite marks a pending invite unusable from the given instant.
func (t *tx) RevokeInvite(ctx context.Context, id string, at time.Time) error {
	return t.closeInvite(ctx, id, "revoked_at", at, "revoking invite %q")
}

func (t *tx) closeInvite(ctx context.Context, id, column string, at time.Time, what string) error {
	b := t.builder("user_invites").Where("id = ?", id).
		Where("accepted_at IS NULL").Where("revoked_at IS NULL").Set(column, sqlb.TimeText(at))
	n, err := t.execUpdate(ctx, b, what, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("invite not found")
	}
	return nil
}
