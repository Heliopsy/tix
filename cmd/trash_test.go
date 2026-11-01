// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestTenantEditTurnsTheTrashOffAndTaskRmSaysItIsPermanent(t *testing.T) {
	c := newCLI(t)
	if got := c.mustRun("task", "trash", "-o", "json"); !strings.Contains(got.out, `"enabled": true`) {
		t.Fatalf("a new tenant's trash = %q, want enabled", got.out)
	}
	if got := c.run("tenant", "edit", "default", "--trash", "maybe"); got.code != core.ExitUsage {
		t.Fatalf("--trash maybe exited %d, want %d", got.code, core.ExitUsage)
	}
	c.mustRun("tenant", "edit", "default", "--trash", "off")
	if got := c.mustRun("task", "trash", "-o", "json"); !strings.Contains(got.out, `"enabled": false`) {
		t.Fatalf("trash after --trash off = %q, want disabled", got.out)
	}

	c.mustRun("task", "add", "permanent")
	rm := c.mustRun("task", "rm", "default-1")
	if !strings.Contains(rm.err, "deleted permanently") {
		t.Errorf("task rm with the trash off said nothing on stderr: %q", rm.err)
	}
	if got := c.run("task", "restore", "default-1"); got.code == core.ExitOK {
		t.Errorf("restoring a task deleted with the trash off succeeded")
	}
	if got := c.run("task", "ls", "--trashed"); got.code != core.ExitPrecondtion {
		t.Errorf("task ls --trashed with the trash off exited %d, want %d\n%s", got.code, core.ExitPrecondtion, got.err)
	}
}
