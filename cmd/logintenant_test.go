// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

const adaPassword = "correct horse battery"

// twoTenantCLI makes ada an admin of the default tenant and of test, and
// serves the API on a host no domain claims, which serves either tenant.
func twoTenantCLI(t *testing.T) (*cli, string) {
	t.Helper()
	c := newCLI(t)
	c.mustRun("user", "create", "ada@example.com", "--password", adaPassword, "--handle", "ada", "--role", "admin")
	c.mustRun("tenant", "create", "test", "Test")
	c.mustRun("tenant", "user", "add", "test", "ada@example.com", "--role", "admin")
	return c, c.loginServer()
}

// loginTenant runs tix login --tenant against url and returns what it wrote.
func (c *cli) loginTenant(url, tenant, context string) loginContext {
	c.t.Helper()
	got := c.run("--server", url, "login", "--tenant", tenant, "--context", context,
		"--email", "ada@example.com", "--password", adaPassword, "-o", "json")
	if got.code != core.ExitOK {
		c.t.Fatalf("login --tenant %s exited %d\nstdout: %s\nstderr: %s", tenant, got.code, got.out, got.err)
	}
	var out loginContext
	if err := json.Unmarshal([]byte(got.out), &out); err != nil {
		c.t.Fatalf("login --tenant printed no context: %v\n%s", err, got.out)
	}
	return out
}

// contextOf reads one context as tix ctx show reports it.
func (c *cli) contextOf(name string) contextRow {
	c.t.Helper()
	var row contextRow
	got := c.mustRun("ctx", "show", name, "-o", "json")
	if err := json.Unmarshal([]byte(got.out), &row); err != nil {
		c.t.Fatalf("ctx show is not json: %v\n%s", err, got.out)
	}
	return row
}

func TestLoginWithATenantStoresAContextPerTenant(t *testing.T) {
	c, url := twoTenantCLI(t)

	test := c.loginTenant(url, "test", "test")
	if test.Tenant != "test" || test.User != "ada" || !test.Current {
		t.Fatalf("login --tenant test = %+v, want ada's context for test, current as the first one", test)
	}
	c.loginTenant(url, "default", "home")
	for name, tenant := range map[string]string{"test": "test", "home": "default"} {
		row := c.contextOf(name)
		if row.Tenant != tenant || !row.Token || row.Server != url {
			t.Errorf("context %s = %+v, want a token for %s on %s", name, row, tenant, url)
		}
	}

	var who identity
	if err := json.Unmarshal([]byte(c.mustRun("--ctx", "test", "whoami", "-o", "json").out), &who); err != nil {
		t.Fatal(err)
	}
	if who.Tenant != "test" || len(who.Memberships) != 2 {
		t.Fatalf("whoami under test = %+v, want tenant test and both memberships", who)
	}
	if !strings.Contains(c.mustRun("--ctx", "test", "whoami").out, "default, admin as ada (credential: home)") {
		t.Errorf("whoami does not list the default tenant with the context holding its token")
	}

	var rows []membershipRow
	if err := json.Unmarshal([]byte(c.mustRun("--ctx", "home", "tenant", "ls", "-o", "json").out), &rows); err != nil {
		t.Fatal(err)
	}
	creds := map[string]string{}
	for _, r := range rows {
		creds[r.Key] = r.Credential
		if r.Role != core.RoleAdmin {
			t.Errorf("membership %s role = %q, want admin", r.Key, r.Role)
		}
	}
	if len(rows) != 2 || creds["test"] != "test" || creds["default"] != "home" {
		t.Fatalf("tenant ls = %+v, want both tenants with the context holding each token", rows)
	}
}

func TestTenantUseSwitchesToTheContextHoldingTheTenantsToken(t *testing.T) {
	c, url := twoTenantCLI(t)
	c.mustRun("tenant", "create", "third", "Third")
	c.mustRun("tenant", "create", "fourth", "Fourth")
	c.mustRun("tenant", "user", "add", "fourth", "ada@example.com", "--role", "member")
	c.loginTenant(url, "test", "test")
	c.loginTenant(url, "default", "home")

	c.mustRun("tenant", "use", "default")
	if !c.contextOf("home").Current {
		t.Fatal("tenant use default did not make the context holding its token current")
	}

	if got := c.run("tenant", "use", "third"); got.code != core.ExitNotFound {
		t.Fatalf("tenant use of a tenant ada is not in exited %d, want %d: %s", got.code, core.ExitNotFound, got.err)
	}
	before := c.mustRun("ctx", "ls", "-o", "json").out
	got := c.run("tenant", "use", "fourth")
	if got.code != core.ExitPrecondtion || !strings.Contains(got.err, "tix login --tenant fourth") {
		t.Fatalf("tenant use with no credential exited %d: %s; want %d naming the login", got.code, got.err, core.ExitPrecondtion)
	}
	if after := c.mustRun("ctx", "ls", "-o", "json").out; after != before {
		t.Errorf("a refused tenant use wrote the configuration:\nbefore %s\nafter  %s", before, after)
	}
}

// A host bound to one tenant refuses a sign-in naming another, and nothing is
// written.
func TestLoginWithATenantIsRefusedOnAHostBoundToAnother(t *testing.T) {
	c, url := twoTenantCLI(t)
	c.mustRun("domain", "add", "localhost")
	bound := strings.Replace(url, "127.0.0.1", "localhost", 1)

	got := c.run("--server", bound, "login", "--tenant", "test", "--context", "test",
		"--email", "ada@example.com", "--password", adaPassword)
	if got.code != core.ExitPermission || !strings.Contains(got.err, core.HeaderTenant) {
		t.Fatalf("login --tenant test on a host bound to default exited %d: %s; want %d naming the header",
			got.code, got.err, core.ExitPermission)
	}
	if out := c.mustRun("ctx", "ls", "-o", "json").out; strings.Contains(out, `"test"`) {
		t.Errorf("a refused login wrote a context: %s", out)
	}
}
