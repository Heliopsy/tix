// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func cleanupOf(t *testing.T, c *cli, args ...string) core.HousekeepingView {
	t.Helper()
	out := c.mustRun(append([]string{"cleanup", "show", "-o", "json"}, args...)...).out
	var v core.HousekeepingView
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("cleanup show is not json: %v\n%s", err, out)
	}
	return v
}

func TestCleanupSetsTheTenantAndAProject(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "seed")
	if v := cleanupOf(t, c); v.Effective.Days(core.RuleDone) != 0 || v.Paused {
		t.Fatalf("a fresh tenant = %+v, want everything off", v)
	}

	c.mustRun("cleanup", "set", "--done", "30", "--trash", "90d")
	table := c.mustRun("cleanup", "show").out
	if !strings.Contains(table, "30d") || !strings.Contains(table, "90d") {
		t.Fatalf("table = %q, want both rules", table)
	}

	c.mustRun("cleanup", "set", "--project", "default", "--done", "off")
	v := cleanupOf(t, c, "--project", "default")
	if v.Effective.Days(core.RuleDone) != 0 || v.Effective.Days(core.RuleTrash) != 90 || v.Override.Trash != nil {
		t.Fatalf("project view = %+v, want done off and trash inherited", v)
	}
	if got := c.mustRun("cleanup", "show", "--project", "default").out; !strings.Contains(got, "inherit") {
		t.Errorf("project table = %q, want the inherited rule named", got)
	}

	preview := c.mustRun("cleanup", "preview", "-o", "json").out
	var res core.HousekeepingResult
	if !strings.Contains(c.mustRun("cleanup", "preview").out, "CANCELLED") {
		t.Error("the preview table has no column per rule")
	}
	if err := json.Unmarshal([]byte(preview), &res); err != nil || !res.DryRun || res.Total() != 0 {
		t.Fatalf("preview = %s (%v), want a dry run with nothing due", preview, err)
	}
	c.mustRun("cleanup", "run")
}

func TestCleanupRefusals(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "seed")
	if got := c.run("cleanup", "set"); got.code != core.ExitUsage {
		t.Errorf("set with no rule exited %d, want usage", got.code)
	}
	if got := c.run("cleanup", "set", "--done", "inherit"); got.code != core.KindInvalid.ExitCode() {
		t.Errorf("the tenant inheriting exited %d, want invalid", got.code)
	}
	c.mustRun("tenant", "edit", "default", "--trash", "off")
	got := c.run("cleanup", "set", "--done", "30")
	if got.code != core.KindPrecondition.ExitCode() || !strings.Contains(got.err, "trash") {
		t.Errorf("done with the trash off exited %d (%s), want the trash refusal", got.code, got.err)
	}
	if got := c.mustRun("cleanup", "preview"); !strings.Contains(got.err, "paused") {
		t.Errorf("preview with the trash off said %q on stderr, want paused", got.err)
	}
}
