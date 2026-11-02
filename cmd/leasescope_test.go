// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestLeaseLengthFromTheCommandLine(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "long build")

	c.mustRun("project", "edit", "default", "--lease-default", "2h")
	if got := c.mustRun("task", "show", "default-1").err; !strings.Contains(got, "claims last 2h, from the project") {
		t.Fatalf("task show said %q, want the project's 2h named", got)
	}

	c.mustRun("task", "edit", "default-1", "--lease", "4h")
	var task core.Task
	if err := json.Unmarshal([]byte(c.mustRun("task", "show", "default-1", "-o", "json").out), &task); err != nil {
		t.Fatal(err)
	}
	if task.Lease.String() != "4h0m0s" {
		t.Fatalf("task lease = %s, want 4h", task.Lease)
	}
	if got := c.mustRun("claim", "task", "default-1").out; !strings.Contains(got, "4h (task)") {
		t.Fatalf("claim output = %q, want the granted length and its level", got)
	}

	c.mustRun("task", "edit", "default-1", "--lease", "")
	if got := c.mustRun("task", "show", "default-1").err; !strings.Contains(got, "from the project") {
		t.Fatalf("after clearing, task show said %q, want the project's figure back", got)
	}

	c.mustRun("project", "edit", "default", "--lease-default", "")
	if got := c.mustRun("task", "show", "default-1").err; !strings.Contains(got, "claims last 30m, the built-in default") {
		t.Fatalf("after inheriting, task show said %q, want the built-in 30m", got)
	}
	if got := c.run("task", "edit", "default-1", "--lease", "soon"); got.code != core.ExitUsage && got.code != 2 {
		t.Fatalf("an unreadable lease exited %d, want 2", got.code)
	}
}
