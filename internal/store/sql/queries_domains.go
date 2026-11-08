// SPDX-License-Identifier: AGPL-3.0-or-later

package sql

import "strings"

// DomainByHostnameQuery renders the hostname lookup that maps a request's host
// to the tenant serving it. Like the SSH fingerprint lookup it runs before a
// tenant is known, so there is no scope to build with yet: a host arrives on a
// connection and the tenant is what the statement is trying to find.
//
// tenant_domains is otherwise fully scoped, so this is the only statement in
// the tree that reads it across tenants, and it reads nothing but the tenant
// row the hostname already names.
//
// cols names the columns of tenants in the order the caller's scanner reads them.
func DomainByHostnameQuery(d Dialect, cols []string, hostname string) (string, []any) {
	b := &Builder{dialect: d}
	qualified := make([]string, len(cols))
	for i, c := range cols {
		qualified[i] = "tenants." + c
	}
	q := "SELECT " + strings.Join(qualified, ", ") +
		" FROM tenant_domains" +
		" JOIN tenants ON tenants.id = tenant_domains.tenant_id" +
		" WHERE tenant_domains.hostname = ? AND tenants.deleted_at IS NULL" +
		" LIMIT 1"
	return b.rebind(q), []any{hostname}
}

// HostByHostnameQuery renders the lookup DomainByHostnameQuery does, with the
// domain's shared flag after the tenant's columns, so the transport learns in
// one read whether the host serves its tenant alone.
func HostByHostnameQuery(d Dialect, cols []string, hostname string) (string, []any) {
	b := &Builder{dialect: d}
	qualified := make([]string, len(cols))
	for i, c := range cols {
		qualified[i] = "tenants." + c
	}
	q := "SELECT " + strings.Join(qualified, ", ") + ", tenant_domains.shared" +
		" FROM tenant_domains" +
		" JOIN tenants ON tenants.id = tenant_domains.tenant_id" +
		" WHERE tenant_domains.hostname = ? AND tenants.deleted_at IS NULL" +
		" LIMIT 1"
	return b.rebind(q), []any{hostname}
}

// TenantAddressQuery renders the lookup of a tenant's own address: its first
// domain not marked shared, verified ones first. A shared host reads it before
// serving a tenant a credential or header names, so a tenant with an address
// of its own keeps its sessions there.
func TenantAddressQuery(d Dialect, tenantID string) (string, []any) {
	b := &Builder{dialect: d}
	q := "SELECT " + tenantAddressColumn("?")
	return b.rebind(q), []any{tenantID, false}
}

// CredentialTenantQuery renders the lookup of the tenant a session or API
// token belongs to, by the hash of its secret. A host serving several tenants
// cannot name one before the credential is read, so this is unscoped; it
// reads one tenant identifier and nothing the credential grants, and the
// credential is verified again inside the tenant it names. Both hashes are
// unique across the installation, so at most one row matches.
func CredentialTenantQuery(d Dialect, tokenHash string) (string, []any) {
	b := &Builder{dialect: d}
	q := "SELECT c.tenant_id FROM (" +
		"SELECT s.tenant_id FROM sessions s WHERE s.token_hash = ?" +
		" UNION ALL SELECT k.tenant_id FROM api_tokens k WHERE k.token_hash = ?" +
		") c JOIN tenants t ON t.id = c.tenant_id WHERE t.deleted_at IS NULL LIMIT 1"
	return b.rebind(q), []any{tokenHash, tokenHash}
}
