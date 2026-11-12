// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// TestMoveDryRunRefusesWhatTheMoveRefuses is the experiment's first finding:
// a dry run that approved states the workflow lacks while the move refused them.
func TestMoveDryRunRefusesWhatTheMoveRefuses(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "dry")

	for _, to := range []string{"review", "in_review", "in-progress", "bogus"} {
		dry := c.run("task", "mv", "default-1", to, "--dry-run")
		real := c.run("task", "mv", "default-1", to)
		if dry.code != core.ExitUsage || real.code != dry.code {
			t.Errorf("%s: dry run exit %d, move exit %d, want both %d", to, dry.code, real.code, core.ExitUsage)
		}
		if !strings.Contains(dry.out+dry.err, "todo, doing, blocked, done, cancelled") {
			t.Errorf("%s: dry run did not list the states:\n%s%s", to, dry.out, dry.err)
		}
	}
	for _, args := range [][]string{
		{"task", "mv", "default-1", "done", "--dry-run"},
		{"task", "mv", "default-1", "doing>bogus", "--hops", "--dry-run"},
	} {
		if got := c.run(args...); got.code == core.ExitOK {
			t.Errorf("%v succeeded, want the refusal the move gives", args)
		}
	}
	if got := c.mustRun("task", "mv", "default-1", "doing", "--dry-run", "-o", "json"); !strings.Contains(got.out, `"planned"`) {
		t.Errorf("a legal dry run was not planned:\n%s", got.out)
	}
	shown := c.mustRun("task", "show", "default-1", "-o", "json")
	var task core.Task
	if err := json.Unmarshal([]byte(shown.out), &task); err != nil {
		t.Fatal(err)
	}
	if task.Status != "todo" || task.Version != 1 {
		t.Errorf("dry runs changed the task: %q v%d", task.Status, task.Version)
	}
}

// TestClaimStartAndFinishIsTwoCalls is the shorter loop: claim and start in
// one call, finish and release in another.
func TestClaimStartAndFinishIsTwoCalls(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "loop")

	got := c.mustRun("claim", "next", "-s", "todo", "--start", "-o", "json")
	var claim core.Claim
	if err := json.Unmarshal([]byte(got.out), &claim); err != nil {
		t.Fatalf("not json: %v\n%s", err, got.out)
	}
	if claim.Task.Status != "doing" || claim.StartedFrom != "todo" {
		t.Fatalf("claim next --start left %q from %q", claim.Task.Status, claim.StartedFrom)
	}

	refused := c.run("task", "mv", "default-1", "done")
	if refused.code != core.ExitConflict || !strings.Contains(refused.err, "--lease-token") {
		t.Errorf("a tokenless move by the holder = %d %q, want 4 naming --lease-token", refused.code, refused.err)
	}

	c.mustRun("task", "mv", "default-1", "done", "--lease-token", claim.LeaseToken)
	shown := c.mustRun("task", "show", "default-1", "-o", "json")
	var task core.Task
	if err := json.Unmarshal([]byte(shown.out), &task); err != nil {
		t.Fatal(err)
	}
	if task.Status != "done" || task.ClaimedByActorID != "" {
		t.Errorf("after the finishing move the task is %q held by %q", task.Status, task.ClaimedByActorID)
	}
	c.mustRun("claim", "release", "default-1", "--token", claim.LeaseToken)

	c.mustRun("task", "add", "named")
	if got := c.run("claim", "task", "default-2", "--start-status", "bogus"); got.code != core.ExitUsage {
		t.Errorf("an unknown start status exited %d, want %d", got.code, core.ExitUsage)
	}
	got = c.mustRun("claim", "task", "default-2", "--start-status", "blocked", "-o", "json")
	if !strings.Contains(got.out, `"started_from": "todo"`) {
		t.Errorf("claim task --start-status did not start the task:\n%s", got.out)
	}
}
