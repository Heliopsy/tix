// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestDomainEditMarksAHostSharedFromAShellHoldingTheDatabase(t *testing.T) {
	c := newCLI(t)
	db := filepath.Join(t.TempDir(), "tix.db")
	c.mustRun("--db", db, "tenant", "create", "test", "Test")
	c.mustRun("--db", db, "--tenant", "test", "domain", "add", "test.example.com")

	if got := c.run("--db", db, "domain", "edit", "test.example.com"); got.code != core.ExitUsage {
		t.Errorf("domain edit with nothing to change exited %d, want %d", got.code, core.ExitUsage)
	}
	plan := c.mustRun("--db", db, "domain", "edit", "test.example.com", "--shared", "--dry-run", "-o", "json")
	if !strings.Contains(plan.out, `"shared": true`) {
		t.Errorf("the dry run does not name the change: %s", plan.out)
	}

	out := c.mustRun("--db", db, "domain", "edit", "test.example.com", "--shared", "-o", "json")
	var got core.Domain
	if err := json.Unmarshal([]byte(out.out), &got); err != nil {
		t.Fatalf("decoding %q: %v", out.out, err)
	}
	if !got.Shared || got.Hostname != "test.example.com" {
		t.Fatalf("domain edit --shared = %+v, want the host shared", got)
	}
	listed := c.mustRun("--db", db, "tenant", "domain", "ls", "test", "-o", "json")
	if !strings.Contains(listed.out, `"shared": true`) {
		t.Errorf("tix tenant domain ls test = %s, want the domain shared", listed.out)
	}

	c.mustRun("--db", db, "domain", "edit", "test.example.com", "--shared=false")
	listed = c.mustRun("--db", db, "tenant", "domain", "ls", "test", "-o", "json")
	if !strings.Contains(listed.out, `"shared": false`) {
		t.Errorf("after --shared=false, tix tenant domain ls test = %s", listed.out)
	}
	if got := c.run("--db", db, "domain", "edit", "nosuch.example.com", "--shared"); got.code != core.ExitNotFound {
		t.Errorf("domain edit of an unmapped host exited %d, want %d", got.code, core.ExitNotFound)
	}
}
