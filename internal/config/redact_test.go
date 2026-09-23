// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"strings"
	"testing"
)

// TestRedactDSNHidesEveryCredentialForm pins two gaps: a prefixed parameter
// such as sslpassword, which a word boundary alone never matched, and a value
// terminator that did not stop at & and so swallowed the parameters after it.
func TestRedactDSNHidesEveryCredentialForm(t *testing.T) {
	for _, tc := range []struct{ dsn, secret, keep string }{
		{"postgres://tix:s3cr3t@db:5432/tix", "s3cr3t", "tix"},
		{"postgres://db:5432/tix?password=s3cr3t", "s3cr3t", ""},
		{"postgres://db:5432/tix?sslpassword=s3cr3t", "s3cr3t", ""},
		{"postgres://db:5432/tix?password=s3cr3t&sslmode=disable", "s3cr3t", "sslmode=disable"},
		{"host=db user=tix password=s3cr3t dbname=tix", "s3cr3t", "dbname=tix"},
	} {
		got := RedactDSN(tc.dsn)
		if strings.Contains(got, tc.secret) {
			t.Errorf("RedactDSN(%q) = %q, the secret survived", tc.dsn, got)
		}
		if tc.keep != "" && !strings.Contains(got, tc.keep) {
			t.Errorf("RedactDSN(%q) = %q, lost %q", tc.dsn, got, tc.keep)
		}
	}
}
