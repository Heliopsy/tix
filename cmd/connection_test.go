// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestConnectionListReportsAnIdleServer(t *testing.T) {
	c := newCLI(t)
	got := c.mustRun("connection", "ls", "-o", "json")

	var list core.ConnectionList
	if err := json.Unmarshal([]byte(got.out), &list); err != nil {
		t.Fatalf("connection ls output is not json: %v\n%s", err, got.out)
	}
	if list.ServerID == "" {
		t.Error("the answer does not name the server it came from")
	}
	if len(list.Connections) != 0 {
		t.Errorf("a process holding nothing listed %+v", list.Connections)
	}
	if !strings.Contains(got.err, "this server only") {
		t.Errorf("stderr does not say the view is one server's: %q", got.err)
	}
}

func TestConnectionKillReportsAnIdentifierThatIsNotLive(t *testing.T) {
	c := newCLI(t)
	got := c.run("connection", "kill", "01JNOTHINGHERE")
	if got.code != core.KindNotFound.ExitCode() {
		t.Errorf("exit = %d, want %d\n%s", got.code, core.KindNotFound.ExitCode(), got.err)
	}
}

func TestConnectionKillNeedsAnIdentifier(t *testing.T) {
	c := newCLI(t)
	if got := c.run("connection", "kill"); got.code != core.ExitUsage {
		t.Errorf("exit = %d, want %d", got.code, core.ExitUsage)
	}
}
