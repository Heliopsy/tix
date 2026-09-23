// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// The product is multi-tenant and --tenant answers "this one command". This is
// the other half: selecting a tenant that later commands keep using, without
// hand-editing the configuration file.
func TestTenantUseSelectsTheTenantLaterCommandsUse(t *testing.T) {
	c := newCLI(t)
	c.mustRun("tenant", "create", "acme", "Acme")
	c.mustRun("tenant", "use", "acme")

	if got := c.mustRun("config", "show", "-o", "json"); !strings.Contains(got.out, `"Tenant": "acme"`) {
		t.Fatalf("tenant use did not change the effective tenant: %s", got.out)
	}
	// A task created now belongs to acme, not to the tenant that was current
	// when tenant use ran.
	c.mustRun("project", "create", "acme", "Acme")
	c.mustRun("task", "add", "inside acme")
	if got := c.mustRun("--tenant", "default", "task", "ls", "-o", "json"); strings.Contains(got.out, "inside acme") {
		t.Fatalf("the task leaked into the tenant that was current before the switch: %s", got.out)
	}
}

// With a context selected the key belongs on the context, because a context is
// what pins the database or server the tenant lives in.
func TestTenantUseWritesOntoTheCurrentContext(t *testing.T) {
	c := newCLI(t)
	c.mustRun("ctx", "add", "work", "--db", filepath.Join(c.home, "work.db"), "--use")
	c.mustRun("tenant", "create", "acme", "Acme")
	c.mustRun("tenant", "use", "acme")

	got := c.mustRun("ctx", "show", "work", "-o", "json")
	if !strings.Contains(got.out, `"tenant": "acme"`) {
		t.Fatalf("the context does not name the selected tenant: %s", got.out)
	}
}

// A tenant that cannot be reached is refused rather than written, because a
// configuration file naming a tenant that does not exist breaks every later
// command with an error that names the wrong thing.
func TestTenantUseRefusesATenantItCannotReach(t *testing.T) {
	c := newCLI(t)
	if got := c.run("tenant", "use", "nosuchtenant"); got.code != core.ExitNotFound {
		t.Fatalf("exit = %d, want %d", got.code, core.ExitNotFound)
	}
	if got := c.mustRun("config", "show", "-o", "json"); strings.Contains(got.out, "nosuchtenant") {
		t.Fatalf("a refused tenant was written anyway: %s", got.out)
	}
}

// --force is for the tenant that does not exist yet, which is otherwise a
// chicken-and-egg: an actor is bound to one tenant, so it cannot create a
// tenant it is not already in.
func TestTenantUseForceSkipsTheCheck(t *testing.T) {
	c := newCLI(t)
	c.mustRun("tenant", "use", "future", "--force")
	if got := c.mustRun("config", "show", "-o", "json"); !strings.Contains(got.out, `"Tenant": "future"`) {
		t.Fatalf("--force did not write the key: %s", got.out)
	}
}

// --dry-run reports without writing, like every other mutating command.
func TestTenantUseDryRunWritesNothing(t *testing.T) {
	c := newCLI(t)
	c.mustRun("tenant", "create", "acme", "Acme")
	c.mustRun("tenant", "use", "acme", "--dry-run")
	if got := c.mustRun("config", "show", "-o", "json"); strings.Contains(got.out, `"Tenant": "acme"`) {
		t.Fatalf("--dry-run wrote the key: %s", got.out)
	}
}

func TestTenantUseRejectsABlankKey(t *testing.T) {
	c := newCLI(t)
	if got := c.run("tenant", "use", "   "); got.code != core.ExitUsage {
		t.Fatalf("exit = %d, want %d", got.code, core.ExitUsage)
	}
}
