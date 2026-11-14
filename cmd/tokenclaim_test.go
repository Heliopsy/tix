// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// issuedClaimToken is the part of `token create -o json` this file reads.
type issuedClaimToken struct {
	Token        string `json:"token"`
	RequireClaim bool   `json:"require_claim"`
}

func TestTokenCreateRequireClaimIsShownAndSurvivesRotation(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "seed")
	var issued issuedClaimToken
	out := c.mustRun("token", "create", "worker", "--scope", "task:read", "--require-claim", "-o", "json").out
	if err := json.Unmarshal([]byte(out), &issued); err != nil || !issued.RequireClaim {
		t.Fatalf("token create --require-claim printed %s (%v)", out, err)
	}
	table := c.mustRun("token", "ls").out
	if !strings.Contains(table, " CLAIM ") || !strings.Contains(table, " required ") {
		t.Errorf("token ls does not show the claim requirement:\n%s", table)
	}
	rotated := c.mustRun("token", "rotate", "worker", "-o", "json").out
	var again issuedClaimToken
	if err := json.Unmarshal([]byte(rotated), &again); err != nil || !again.RequireClaim {
		t.Errorf("the rotated token dropped the claim requirement: %s", rotated)
	}
	plain := c.mustRun("token", "create", "script", "--scope", "task:read", "-o", "json").out
	if strings.Contains(plain, `"require_claim": true`) {
		t.Errorf("a token minted without the flag requires a claim: %s", plain)
	}
}

// TestAClaimRequiringTokenOverTheServer drives the whole path a remote agent
// takes: the refusal and its exit code, the dry run, claim --start in one
// step, and the finishing move that releases the lease.
func TestAClaimRequiringTokenOverTheServer(t *testing.T) {
	host := newCLI(t)
	host.mustRun("task", "add", "agent work")
	var issued issuedClaimToken
	out := host.mustRun("token", "create", "agent", "--scope", "*", "--require-claim", "-o", "json").out
	if err := json.Unmarshal([]byte(out), &issued); err != nil || issued.Token == "" {
		t.Fatalf("token create: %v (%s)", err, out)
	}

	addr := freePort(t)
	done := make(chan result, 1)
	go func() { done <- host.run("serve", "--listen", addr, "--no-registry") }()
	waitFor(t, "http://"+addr+"/healthz")
	defer stopServe(t, done)

	c := newCLI(t)
	remote := []string{"--server", "http://" + addr, "--token", issued.Token}
	run := func(args ...string) result { return c.runBare(nil, append(append([]string{}, remote...), args...)...) }

	hint := "run: tix claim task default-1 --start"
	for _, args := range [][]string{
		{"task", "mv", "default-1", "doing"},
		{"task", "mv", "default-1", "doing", "--dry-run"},
		{"task", "edit", "default-1", "--priority", "high"},
	} {
		got := run(args...)
		if got.code != core.ExitPermission || !strings.Contains(got.err, hint) {
			t.Errorf("tix %s = %d %q, want exit 5 naming %q", strings.Join(args, " "), got.code, got.err, hint)
		}
	}
	if got := run("comment", "add", "default-1", "picking this up"); got.code != core.ExitOK {
		t.Errorf("a comment without a claim exited %d: %s", got.code, got.err)
	}

	claimed := run("claim", "task", "default-1", "--start", "-o", "json")
	if claimed.code != core.ExitOK {
		t.Fatalf("claim task --start exited %d: %s", claimed.code, claimed.err)
	}
	var claim core.Claim
	if err := json.Unmarshal([]byte(claimed.out), &claim); err != nil || claim.Task.Status != "doing" {
		t.Fatalf("claim task --start printed %s (%v)", claimed.out, err)
	}
	if got := run("task", "mv", "default-1", "done", "--lease-token", claim.LeaseToken); got.code != core.ExitOK {
		t.Fatalf("the finishing move exited %d: %s", got.code, got.err)
	}
	var task core.Task
	if err := json.Unmarshal([]byte(run("task", "show", "default-1", "-o", "json").out), &task); err != nil {
		t.Fatal(err)
	}
	if task.Status != "done" || task.ClaimedByActorID != "" {
		t.Errorf("after the finishing move the task is %q held by %q", task.Status, task.ClaimedByActorID)
	}
}
