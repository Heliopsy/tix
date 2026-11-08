// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"

	"github.com/heliopsy/tix/internal/core"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// ListMemberships returns every tenant an account holds a membership in.
func (t *tx) ListMemberships(ctx context.Context, userID string) ([]core.AccountMembership, error) {
	if err := t.accountLookup(ctx, true); err != nil {
		return nil, err
	}
	defer func() { _ = t.accountLookup(ctx, false) }()

	q, args := sqlb.AccountMembershipsQuery(dialect, userID)
	rows, err := t.ex.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, mapErr(err, "listing the memberships of user %q", userID)
	}
	defer func() { _ = rows.Close() }()
	out := []core.AccountMembership{}
	for rows.Next() {
		var m core.AccountMembership
		if err := rows.Scan(&m.TenantID, &m.TenantKey, &m.TenantName, &m.ActorID, &m.Handle, &m.Role, &m.Host); err != nil {
			return nil, mapErr(err, "scanning a membership")
		}
		out = append(out, m)
	}
	return out, mapRowsErr(rows, "listing memberships")
}

// accountLookup raises or lowers the flag accountLookupPolicy reads. It is
// transaction-local, so it cannot outlive the listing that raised it, and this
// is deliberately the only place in the tree that writes it.
func (t *tx) accountLookup(ctx context.Context, on bool) error {
	value := ""
	if on {
		value = accountLookupOn
	}
	if _, err := t.ex.ExecContext(ctx,
		`SELECT set_config($1, $2, true)`, accountLookupSetting, value); err != nil {
		return mapErr(err, "admitting the membership listing")
	}
	return nil
}
