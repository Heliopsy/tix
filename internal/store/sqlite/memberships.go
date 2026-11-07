// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"

	"github.com/heliopsy/tix/internal/core"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// ListMemberships returns every tenant an account holds a membership in.
func (t *tx) ListMemberships(ctx context.Context, userID string) ([]core.AccountMembership, error) {
	q, args := sqlb.AccountMembershipsQuery(dialect, userID)
	rows, err := t.ex.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, mapErr(err, "listing the memberships of user %q", userID)
	}
	defer func() { _ = rows.Close() }()
	out := []core.AccountMembership{}
	for rows.Next() {
		var m core.AccountMembership
		if err := rows.Scan(&m.TenantID, &m.TenantKey, &m.TenantName, &m.ActorID, &m.Handle, &m.Role); err != nil {
			return nil, mapErr(err, "scanning a membership")
		}
		out = append(out, m)
	}
	return out, mapRowsErr(rows, "listing memberships")
}
