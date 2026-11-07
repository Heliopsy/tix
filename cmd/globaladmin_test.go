// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestTenantCreateWithAdminsPrintsTheirPasswordsOnce(t *testing.T) {
	c := newCLI(t)
	db := filepath.Join(t.TempDir(), "tix.db")

	got := c.mustRun("--db", db, "tenant", "create", "test", "--name", "Test",
		"--admin", "villy@z09.test", "--admin", "icek@z09.test")
	if !strings.Contains(got.err, "shown once") {
		t.Errorf("stderr %q does not warn that the passwords are shown once", got.err)
	}
	for _, email := range []string{"villy@z09.test", "icek@z09.test"} {
		if !strings.Contains(got.out, email) {
			t.Errorf("the credentials table does not name %s:\n%s", email, got.out)
		}
	}

	asJSON := c.mustRun("--db", db, "tenant", "create", "json", "JSON", "--admin", "ada@z09.test", "-o", "json")
	var created core.CreatedTenant
	if err := json.Unmarshal([]byte(asJSON.out), &created); err != nil {
		t.Fatalf("decoding %q: %v", asJSON.out, err)
	}
	if created.Key != "json" || len(created.InitialCredentials) != 1 || created.InitialCredentials[0].Password == "" {
		t.Fatalf("json = %+v, want the tenant and one generated password", created)
	}

	users := c.mustRun("--db", db, "tenant", "user", "ls", "json", "-o", "json")
	if !strings.Contains(users.out, `"role": "admin"`) || !strings.Contains(users.out, "ada@z09.test") {
		t.Errorf("tix tenant user ls json = %s, want ada as admin", users.out)
	}
}

func TestTenantCreateTakesTheNameOnceAndValidatesAdmins(t *testing.T) {
	c := newCLI(t)
	db := filepath.Join(t.TempDir(), "tix.db")
	for _, args := range [][]string{
		{"tenant", "create", "test"},
		{"tenant", "create", "test", "Test", "--name", "Test"},
		{"tenant", "create", "test", "Test", "--admin", "not-an-email"},
		{"tenant", "create", "test", "Test", "--admin", "a@z.test", "--admin", "A@Z.test"},
	} {
		if got := c.run(append([]string{"--db", db}, args...)...); got.code != core.ExitUsage {
			t.Errorf("tix %s exited %d, want %d\nstderr: %s", strings.Join(args, " "), got.code, core.ExitUsage, got.err)
		}
	}
}

func TestGlobalAdminCommandsFromAShellHoldingTheDatabase(t *testing.T) {
	c := newCLI(t)
	db := filepath.Join(t.TempDir(), "tix.db")
	c.mustRun("--db", db, "user", "create", "root@acme.test", "--role", "admin", "--password", "correct-horse-battery")

	c.mustRun("--db", db, "global-admin", "grant", "root@acme.test")
	listed := c.mustRun("--db", db, "global-admin", "ls", "-o", "json")
	if !strings.Contains(listed.out, "root@acme.test") || !strings.Contains(listed.out, `"global_admin": true`) {
		t.Errorf("tix global-admin ls = %s, want root", listed.out)
	}
	if got := c.run("--db", db, "global-admin", "revoke", "root@acme.test"); got.code != core.ExitPrecondtion {
		t.Errorf("revoking the last global admin exited %d, want %d\nstderr: %s", got.code, core.ExitPrecondtion, got.err)
	}
	if got := c.run("--db", db, "global-admin", "grant", "nobody@acme.test"); got.code != core.ExitNotFound {
		t.Errorf("granting an unknown user exited %d, want %d", got.code, core.ExitNotFound)
	}
}

func TestTenantUserCommandsManageAnotherTenant(t *testing.T) {
	c := newCLI(t)
	db := filepath.Join(t.TempDir(), "tix.db")
	c.mustRun("--db", db, "tenant", "create", "acme", "Acme")

	c.mustRun("--db", db, "tenant", "user", "create", "acme", "carol@acme.test", "--role", "viewer",
		"--display-name", "Carol", "--handle", "carol", "--password", "correct-horse-battery")
	if got := c.run("--db", db, "tenant", "user", "create", "acme", "carol@acme.test"); got.code != core.ExitConflict {
		t.Errorf("creating carol twice exited %d, want %d", got.code, core.ExitConflict)
	}
	listed := c.mustRun("--db", db, "tenant", "user", "ls", "acme", "-o", "json")
	var users []core.TenantUser
	if err := json.Unmarshal([]byte(listed.out), &users); err != nil || len(users) != 1 {
		t.Fatalf("tix tenant user ls acme = %s (%v), want carol", listed.out, err)
	}
	id := users[0].ID
	assertTenantUserTable(t, c.mustRun("--db", db, "tenant", "user", "ls", "acme").out, id)

	c.mustRun("--db", db, "tenant", "user", "edit", "acme", id, "--role", "admin",
		"--display-name", "Carol A", "--password", "another-long-password", "--disabled=false")
	after := c.mustRun("--db", db, "tenant", "user", "ls", "acme", "-o", "json")
	if !strings.Contains(after.out, `"role": "admin"`) || !strings.Contains(after.out, "Carol A") {
		t.Errorf("after the edit carol is %s, want admin named Carol A", after.out)
	}

	for _, args := range [][]string{
		{"tenant", "user", "create", "acme", "dave@acme.test", "--dry-run"},
		{"tenant", "user", "edit", "acme", id, "--role", "viewer", "--dry-run"},
		{"tenant", "user", "rm", "acme", id, "--dry-run"},
	} {
		c.mustRun(append([]string{"--db", db}, args...)...)
	}
	c.mustRun("--db", db, "tenant", "user", "rm", "acme", id)
	if got := c.run("--db", db, "tenant", "user", "rm", "acme", id); got.code != core.ExitNotFound {
		t.Errorf("removing carol twice exited %d, want %d", got.code, core.ExitNotFound)
	}
	if got := c.run("--db", db, "tenant", "user", "edit", "acme", id, "--role", "admin"); got.code != core.ExitNotFound {
		t.Errorf("editing a removed user exited %d, want %d", got.code, core.ExitNotFound)
	}
	if got := c.run("--db", db, "tenant", "user", "ls", "nosuch"); got.code != core.ExitNotFound {
		t.Errorf("listing an unknown tenant exited %d, want %d", got.code, core.ExitNotFound)
	}
	if got := c.run("--db", db, "tenant", "user", "create", "acme", ""); got.code != core.ExitUsage {
		t.Errorf("creating a user with no address exited %d, want %d", got.code, core.ExitUsage)
	}
}

func TestGlobalAdminGrantAndRevokeRoundTrip(t *testing.T) {
	c := newCLI(t)
	db := filepath.Join(t.TempDir(), "tix.db")
	for _, email := range []string{"one@acme.test", "two@acme.test"} {
		c.mustRun("--db", db, "user", "create", email, "--role", "admin")
		c.mustRun("--db", db, "global-admin", "grant", email)
	}
	c.mustRun("--db", db, "global-admin", "grant", "one@acme.test", "--dry-run")
	c.mustRun("--db", db, "global-admin", "revoke", "two@acme.test", "--dry-run")
	c.mustRun("--db", db, "global-admin", "revoke", "two@acme.test")
	listed := c.mustRun("--db", db, "global-admin", "ls")
	if !strings.Contains(listed.out, "one@acme.test") || strings.Contains(listed.out, "two@acme.test") {
		t.Errorf("after revoking two the listing is %s", listed.out)
	}
	for _, args := range [][]string{
		{"global-admin", "grant", "not-an-email"},
		{"global-admin", "revoke", "not-an-email"},
	} {
		if got := c.run(append([]string{"--db", db}, args...)...); got.code != core.ExitUsage {
			t.Errorf("tix %s exited %d, want %d", strings.Join(args, " "), got.code, core.ExitUsage)
		}
	}
	tenants := c.mustRun("--db", db, "tenant", "ls")
	if !strings.Contains(tenants.out, "default") {
		t.Errorf("tix tenant ls from the shell = %s", tenants.out)
	}
}

// A shell holding the database stands as a global admin, so the tenant
// commands reach any tenant from there, not only the one in force.
func TestTheShellAdministersAnyTenant(t *testing.T) {
	c := newCLI(t)
	db := filepath.Join(t.TempDir(), "tix.db")
	c.mustRun("--db", db, "tenant", "create", "acme", "Acme")

	if got := c.mustRun("--db", db, "tenant", "show", "acme"); !strings.Contains(got.out, "Acme") {
		t.Errorf("tix tenant show acme = %s", got.out)
	}
	c.mustRun("--db", db, "tenant", "edit", "acme", "--name", "Acme Ltd")
	if got := c.mustRun("--db", db, "tenant", "ls"); !strings.Contains(got.out, "Acme Ltd") || !strings.Contains(got.out, "default") {
		t.Errorf("tix tenant ls from the shell = %s, want every tenant", got.out)
	}
	c.mustRun("--db", db, "tenant", "rm", "acme", "--dry-run")
	c.mustRun("--db", db, "tenant", "rm", "acme")
	for _, args := range [][]string{
		{"tenant", "rm", "acme"},
		{"tenant", "rm", "nosuch", "--dry-run"},
		{"tenant", "show", "nosuch"},
		{"tenant", "edit", "nosuch", "--name", "X"},
		{"member", "rm", "01J000000000000000000NOPE"},
		{"domain", "rm", "nosuch.example"},
	} {
		if got := c.run(append([]string{"--db", db}, args...)...); got.code != core.ExitNotFound {
			t.Errorf("tix %s exited %d, want %d\nstderr: %s", strings.Join(args, " "), got.code, core.ExitNotFound, got.err)
		}
	}
}

// assertTenantUserTable checks tix tenant user ls draws one column per field,
// carol's row reading as her address, name and role, and never the struct.
func assertTenantUserTable(t *testing.T, out, id string) {
	t.Helper()
	var rows [][]string
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "│") {
			continue
		}
		var cells []string
		for _, c := range strings.Split(strings.Trim(line, "│"), "│") {
			cells = append(cells, strings.TrimSpace(c))
		}
		rows = append(rows, cells)
	}
	if len(rows) != 2 {
		t.Fatalf("tix tenant user ls acme =\n%s\nwant a header and one row", out)
	}
	if got := strings.Join(rows[0], ","); got != "ID,EMAIL,NAME,ROLE,GLOBAL ADMIN,CREATED,DISABLED" {
		t.Errorf("header = %q", got)
	}
	row := rows[1]
	if len(row) != 7 || row[0] != id || row[1] != "carol@acme.test" || row[2] != "Carol" || row[3] != "viewer" || row[4] != "no" || row[6] != "" {
		t.Errorf("carol's row = %q, want id, address, name, role viewer, global admin no, not disabled", row)
	}
	if strings.ContainsAny(out, "{}") {
		t.Errorf("the table prints a struct:\n%s", out)
	}
}

func TestTenantUserAddPutsAnExistingAccountInAnotherTenant(t *testing.T) {
	c := newCLI(t)
	db := filepath.Join(t.TempDir(), "tix.db")
	c.mustRun("--db", db, "user", "create", "villy@ho.test", "--role", "member", "--password", "correct-horse-battery")
	c.mustRun("--db", db, "tenant", "create", "test", "Test")

	c.mustRun("--db", db, "tenant", "user", "add", "test", "villy@ho.test", "--dry-run")
	c.mustRun("--db", db, "tenant", "user", "add", "test", "villy@ho.test", "--role", "admin")
	listed := c.mustRun("--db", db, "tenant", "user", "ls", "test", "-o", "json")
	var users []core.TenantUser
	if err := json.Unmarshal([]byte(listed.out), &users); err != nil || len(users) != 1 ||
		users[0].Email != "villy@ho.test" || users[0].Role != core.RoleAdmin {
		t.Fatalf("tix tenant user ls test = %s (%v), want villy as admin", listed.out, err)
	}
	if got := c.run("--db", db, "tenant", "user", "add", "test", "villy@ho.test"); got.code != core.ExitConflict {
		t.Errorf("adding villy twice exited %d, want %d", got.code, core.ExitConflict)
	}
	if got := c.run("--db", db, "tenant", "user", "add", "test", "nobody@ho.test"); got.code != core.ExitNotFound {
		t.Errorf("adding an address with no account exited %d, want %d", got.code, core.ExitNotFound)
	}
}

func TestAccountLogCommands(t *testing.T) {
	c := newCLI(t)
	db := filepath.Join(t.TempDir(), "tix.db")
	c.mustRun("--db", db, "user", "create", "villy@ho.test", "--role", "member", "--password", "correct-horse-battery")
	c.mustRun("--db", db, "global-admin", "grant", "villy@ho.test")

	listed := c.mustRun("--db", db, "global-admin", "account-log", "villy@ho.test", "-o", "json")
	var entries []core.AccountLogEntry
	if err := json.Unmarshal([]byte(listed.out), &entries); err != nil || len(entries) != 2 {
		t.Fatalf("tix global-admin account-log = %s (%v), want two entries", listed.out, err)
	}
	if entries[0].Action != core.AccountGlobalAdminGranted || entries[1].Action != core.AccountCreated {
		t.Errorf("entries = %v, %v; want the grant first, newest first", entries[0].Action, entries[1].Action)
	}
	table := c.mustRun("--db", db, "global-admin", "account-log", "villy@ho.test")
	if !strings.Contains(table.out, "ACTION") || !strings.Contains(table.out, "account.created") {
		t.Errorf("the table = %s, want columns with the actions", table.out)
	}
	if got := c.run("--db", db, "global-admin", "account-log", "nobody@ho.test"); got.code != core.ExitNotFound {
		t.Errorf("an unknown address exited %d, want %d", got.code, core.ExitNotFound)
	}
	if got := c.run("--db", db, "account", "log"); got.code != core.ExitUsage {
		t.Errorf("tix account log from a shell, which is nobody's account, exited %d, want %d", got.code, core.ExitUsage)
	}
}
