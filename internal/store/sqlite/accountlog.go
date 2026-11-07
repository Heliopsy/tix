// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/heliopsy/tix/internal/core"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

var accountLogColumns = []string{
	"seq", "user_id", "action", "tenant_key", "by_user_id", "by_email", "by_global_admin",
	"via_kind", "detail", "occurred_at",
}

func scanAccountLog(s scanner) (core.AccountLogEntry, error) {
	var (
		e        core.AccountLogEntry
		global   int64
		detail   sql.NullString
		occurred sql.NullString
	)
	if err := s.Scan(&e.Seq, &e.UserID, &e.Action, &e.TenantKey, &e.ByUserID, &e.ByEmail, &global,
		&e.Via, &detail, &occurred); err != nil {
		return core.AccountLogEntry{}, mapErr(err, "scanning an account log entry")
	}
	e.ByGlobalAdmin = sqlb.Bool(global)
	if detail.Valid && detail.String != "" {
		if err := json.Unmarshal([]byte(detail.String), &e.Detail); err != nil {
			return core.AccountLogEntry{}, core.Internal("decoding an account log entry").Wrap(err)
		}
	}
	var err error
	if e.OccurredAt, err = sqlb.ScanTime(occurred); err != nil {
		return core.AccountLogEntry{}, err
	}
	return e, err
}

// AppendAccountLog records one change to an account in the caller's
// transaction, beside the change itself.
func (t *tx) AppendAccountLog(ctx context.Context, e *core.AccountLogEntry) error {
	detail := ""
	if len(e.Detail) > 0 {
		raw, err := json.Marshal(e.Detail)
		if err != nil {
			return core.Internal("encoding an account log entry").Wrap(err)
		}
		detail = string(raw)
	}
	if e.Via == "" {
		e.Via = core.ViaUnknown
	}
	ins := t.insert("account_log").
		Set("user_id", e.UserID).
		Set("action", string(e.Action)).
		Set("tenant_key", e.TenantKey).
		Set("by_user_id", e.ByUserID).
		Set("by_email", e.ByEmail).
		Set("by_global_admin", sqlb.BoolInt(e.ByGlobalAdmin)).
		Set("via_kind", string(e.Via)).
		Set("detail", sqlb.NullText(detail)).
		Set("occurred_at", sqlb.TimeText(e.OccurredAt))
	_, err := t.execInsert(ctx, ins, "appending to the log of account %q", e.UserID)
	return err
}

// ListAccountLog returns one account's log, keyset paginated on seq.
func (t *tx) ListAccountLog(ctx context.Context, userID string, page core.Page) ([]core.AccountLogEntry, error) {
	spec, err := resolvePage(page, "seq", map[string]string{"seq": "seq"})
	if err != nil {
		return nil, err
	}
	b := t.builder("account_log").Select(accountLogColumns...).Where("user_id = ?", userID)
	rows, err := t.query(ctx, spec.apply(b, "seq"), "listing the log of account %q", userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []core.AccountLogEntry{}
	for rows.Next() {
		e, err := scanAccountLog(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, mapRowsErr(rows, "listing an account log")
}
