// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// stdinPassword is long enough to pass the length rule, so a command that
// took "-" literally fails on length instead of reading it.
const stdinPassword = "piped in from a file"

// signsIn reports whether email signs in with password on c's database.
func signsIn(c *cli, email, password string) bool {
	got := c.runIn(password+"\n", "login", "--email", email, "--password", "-", "-o", "json")
	var session core.Session
	return got.code == core.ExitOK && json.Unmarshal([]byte(got.out), &session) == nil && session.Token != ""
}

func TestUserCreatePasswordDashReadsStandardInput(t *testing.T) {
	c := newCLI(t)
	got := c.runIn(stdinPassword+"\n", "user", "create", "ada@example.com", "--handle", "ada", "--password", "-")
	if got.code != core.ExitOK {
		t.Fatalf("user create --password - exited %d: %s", got.code, got.err)
	}
	if !signsIn(c, "ada@example.com", stdinPassword) {
		t.Fatal("the password read from standard input, less its newline, does not sign in")
	}
	if empty := c.runIn("\n", "user", "create", "bob@example.com", "--password", "-"); empty.code != core.ExitUsage ||
		!strings.Contains(empty.err, "standard input") {
		t.Fatalf("an empty standard input exited %d, err %q; want %d naming standard input", empty.code, empty.err, core.ExitUsage)
	}
}

func TestUserEditPasswordDashReadsStandardInput(t *testing.T) {
	c := newCLI(t)
	var user core.User
	out := c.mustRun("user", "create", "ada@example.com", "--handle", "ada", "--password", firstPassword, "-o", "json").out
	if err := json.Unmarshal([]byte(out), &user); err != nil {
		t.Fatalf("user create is not json: %v", err)
	}
	if got := c.runIn(stdinPassword+"\r\n", "user", "edit", user.ID, "--password", "-"); got.code != core.ExitOK {
		t.Fatalf("user edit --password - exited %d: %s", got.code, got.err)
	}
	if !signsIn(c, "ada@example.com", stdinPassword) || signsIn(c, "ada@example.com", firstPassword) {
		t.Fatal("the edit did not set the password read from standard input")
	}
	if empty := c.runIn("", "user", "edit", user.ID, "--password", "-"); empty.code != core.ExitUsage {
		t.Fatalf("an empty standard input exited %d, want %d", empty.code, core.ExitUsage)
	}
}

func TestLoginPasswordDashReadsStandardInput(t *testing.T) {
	c := newCLI(t)
	c.mustRun("user", "create", "ada@example.com", "--handle", "ada", "--password", stdinPassword)
	if !signsIn(c, "ada@example.com", stdinPassword) {
		t.Fatal("login --password - did not read the password from standard input")
	}
	if empty := c.runIn("", "login", "--email", "ada@example.com", "--password", "-"); empty.code != core.ExitUsage {
		t.Fatalf("an empty standard input exited %d, want %d", empty.code, core.ExitUsage)
	}
}

func TestTenantUserPasswordDashReadsStandardInput(t *testing.T) {
	c := newCLI(t)
	db := filepath.Join(t.TempDir(), "tix.db")
	c.mustRun("--db", db, "tenant", "create", "acme", "Acme")

	created := c.runIn(stdinPassword+"\n", "--db", db, "tenant", "user", "create", "acme", "carol@acme.test",
		"--handle", "carol", "--password", "-")
	if created.code != core.ExitOK {
		t.Fatalf("tenant user create --password - exited %d: %s", created.code, created.err)
	}
	listed := c.mustRun("--db", db, "tenant", "user", "ls", "acme", "-o", "json")
	var users []core.TenantUser
	if err := json.Unmarshal([]byte(listed.out), &users); err != nil || len(users) != 1 {
		t.Fatalf("tenant user ls acme = %s (%v), want carol", listed.out, err)
	}
	if got := c.runIn(stdinPassword+" again\n", "--db", db, "tenant", "user", "edit", "acme", users[0].ID,
		"--password", "-"); got.code != core.ExitOK {
		t.Fatalf("tenant user edit --password - exited %d: %s", got.code, got.err)
	}
	for _, args := range [][]string{
		{"tenant", "user", "create", "acme", "dave@acme.test", "--password", "-"},
		{"tenant", "user", "edit", "acme", users[0].ID, "--password", "-"},
	} {
		if got := c.runIn("\n", append([]string{"--db", db}, args...)...); got.code != core.ExitUsage {
			t.Errorf("tix %s with nothing on standard input exited %d, want %d", strings.Join(args, " "), got.code, core.ExitUsage)
		}
	}
}
