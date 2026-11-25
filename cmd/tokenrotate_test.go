// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// The new value goes to stdout, where a script or an SSH session reads it, and
// the warning about it to stderr, so `-o json` stays parseable.
func TestTokenRotatePrintsTheNewValueOnceToStdout(t *testing.T) {
	c := newCLI(t)
	var first, second core.IssuedToken
	if err := json.Unmarshal([]byte(c.mustRun("token", "create", "rotor", "--scope", "task:read", "-o", "json").out), &first); err != nil {
		t.Fatalf("token create: %v", err)
	}
	got := c.mustRun("token", "rotate", "rotor", "--grace", "10m", "-o", "json")
	if err := json.Unmarshal([]byte(got.out), &second); err != nil {
		t.Fatalf("token rotate -o json printed %q: %v", got.out, err)
	}
	if second.Token == "" || second.Token == first.Token || second.ID == first.ID || second.Name != "rotor" {
		t.Errorf("token rotate printed %+v, want a new token named rotor with its value", second)
	}
	if strings.Contains(got.err, second.Token) || !strings.Contains(got.err, "shown once") {
		t.Errorf("stderr is %q, want the shown-once warning and never the value", got.err)
	}

	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"token", "rotate", "rotor", "--grace", "25h"}, core.ExitUsage},
		{[]string{"token", "rotate", "rotor", "--grace", "soon"}, core.ExitUsage},
		{[]string{"token", "rotate", "nosuch"}, core.ExitNotFound},
		{[]string{"token", "rotate", first.ID}, core.ExitConflict},
	} {
		if res := c.run(tc.args...); res.code != tc.code {
			t.Errorf("%v exited %d, want %d\nstderr: %s", tc.args, res.code, tc.code, res.err)
		}
	}
	if res := c.run("token", "rotate", "rotor", "--dry-run", "-o", "json"); res.code != core.ExitOK ||
		!strings.Contains(res.out, "token.rotate") {
		t.Errorf("--dry-run printed %q (exit %d), want the plan", res.out, res.code)
	}
}
