// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// A shell holding the database signs in through no session, so it lists none,
// and revoking an identifier that is not its own is not found.
func TestAccountSessionsListAndRevoke(t *testing.T) {
	c := newCLI(t)
	db := filepath.Join(t.TempDir(), "tix.db")
	c.mustRun("--db", db, "user", "create", "ada@s01.test", "--password", "correct-horse-battery")

	if got := c.mustRun("--db", db, "account", "sessions", "-o", "json"); strings.TrimSpace(got.out) != "[]" {
		t.Errorf("tix account sessions -o json = %q, want []", got.out)
	}
	if got := c.mustRun("--db", db, "account", "sessions"); !strings.Contains(got.out, "USER AGENT") {
		t.Errorf("the table = %q", got.out)
	}
	if got := c.run("--db", db, "account", "sessions", "revoke", "01J8Z3K4M5N6P7Q8R9S0T1V2W3"); got.code != core.ExitNotFound {
		t.Errorf("revoking a session that is not yours exited %d, want %d", got.code, core.ExitNotFound)
	}
	if got := c.run("--db", db, "account", "sessions", "revoke"); got.code != core.ExitUsage {
		t.Errorf("revoke without an identifier exited %d, want %d", got.code, core.ExitUsage)
	}
}
