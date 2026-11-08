// SPDX-License-Identifier: AGPL-3.0-or-later

package sql

// AccountMembershipsQuery renders the listing of every tenant an account
// holds a membership in, which is deliberately not scoped: an account is
// global, and deciding whether a change to it reaches beyond the caller's
// tenant needs every tenant it belongs to. It reads the account's actors and
// their memberships, never anything they own, and skips deleted tenants. Each
// row carries its tenant's own address, the first domain not marked shared,
// verified ones first, or the empty string.
func AccountMembershipsQuery(d Dialect, userID string) (string, []any) {
	b := &Builder{dialect: d}
	q := "SELECT a.tenant_id, t.key, t.name, a.id, a.handle, m.role, " + tenantAddressColumn("a.tenant_id") +
		" FROM actors a" +
		" JOIN tenant_members m ON m.tenant_id = a.tenant_id AND m.actor_id = a.id" +
		" JOIN tenants t ON t.id = a.tenant_id" +
		" WHERE a.user_id = ? AND t.deleted_at IS NULL" +
		" ORDER BY t.key"
	return b.rebind(q), []any{false, userID}
}

// tenantAddressColumn is the subquery naming a tenant's own address. It takes
// one argument, false, for the shared flag.
func tenantAddressColumn(tenantID string) string {
	return "COALESCE((SELECT d.hostname FROM tenant_domains d" +
		" WHERE d.tenant_id = " + tenantID + " AND d.shared = ?" +
		" ORDER BY (d.verified_at IS NULL), d.hostname LIMIT 1), '')"
}
