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
