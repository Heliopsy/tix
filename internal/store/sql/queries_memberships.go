// SPDX-License-Identifier: AGPL-3.0-or-later

package sql

// AccountMembershipsQuery renders the listing of every tenant an account
// holds a membership in, which is deliberately not scoped: an account is
// global, and deciding whether a change to it reaches beyond the caller's
// tenant needs every tenant it belongs to. It reads the account's actors and
// their memberships, never anything they own, and skips deleted tenants.
func AccountMembershipsQuery(d Dialect, userID string) (string, []any) {
	b := &Builder{dialect: d}
	q := "SELECT a.tenant_id, t.key, t.name, a.id, a.handle, m.role" +
		" FROM actors a" +
		" JOIN tenant_members m ON m.tenant_id = a.tenant_id AND m.actor_id = a.id" +
		" JOIN tenants t ON t.id = a.tenant_id" +
		" WHERE a.user_id = ? AND t.deleted_at IS NULL" +
		" ORDER BY t.key"
	return b.rebind(q), []any{userID}
}
