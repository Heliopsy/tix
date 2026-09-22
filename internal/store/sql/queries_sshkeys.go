package sql

import "strings"

// SSHKeysByFingerprintQuery renders the lookup of a fingerprint across every
// tenant, which is the one statement here that is deliberately not scoped: an
// SSH client proves a key before any tenant is known, so there is no scope to
// build with yet. It returns live enrolments only, and it selects tenant_id so
// the caller can narrow to one tenant before anything else happens.
//
// cols names the columns in the order the caller's scanner reads them.
func SSHKeysByFingerprintQuery(d Dialect, cols []string, fingerprint string) (string, []any) {
	b := &Builder{dialect: d}
	q := "SELECT " + strings.Join(cols, ", ") + " FROM ssh_keys" +
		" WHERE fingerprint = ? AND revoked_at IS NULL" +
		" ORDER BY tenant_id, id"
	return b.rebind(q), []any{fingerprint}
}
