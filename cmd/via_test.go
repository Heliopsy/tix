// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// viaEntries lists the audit entries `tix audit ls` selects with args.
func viaEntries(t *testing.T, c *cli, args ...string) []core.AuditEntry {
	t.Helper()
	out := c.mustRun(append([]string{"audit", "ls", "-o", "ndjson"}, args...)...).out
	var entries []core.AuditEntry
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var e core.AuditEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("reading %q: %v", line, err)
		}
		entries = append(entries, e)
	}
	return entries
}

func TestAuditListsAndFiltersTheChannel(t *testing.T) {
	c := auditCLI(t)
	local := viaEntries(t, c, "--via", "cli-local")
	if len(local) == 0 {
		t.Fatal("--via cli-local kept nothing, though every entry was written on the database")
	}
	for _, e := range local {
		if e.Via.Kind != core.ViaCLILocal {
			t.Errorf("--via cli-local kept %s via %q", e.Action, e.Via.Kind)
		}
	}
	if got := viaEntries(t, c, "--via", "ssh"); len(got) != 0 {
		t.Errorf("--via ssh kept %d entries nobody made over ssh", len(got))
	}
	if got := viaEntries(t, c, "--filter", "-via:cli-local"); len(got) != 0 {
		t.Errorf("-via:cli-local kept %d entries", len(got))
	}
	if got := c.run("audit", "ls", "--via", "telepathy"); got.code != core.ExitUsage || !strings.Contains(got.err, "cli-remote") {
		t.Errorf("an unknown channel exited %d: %s", got.code, got.err)
	}
	table := c.mustRun("audit", "ls").out
	if !strings.Contains(table, " VIA ") {
		t.Errorf("the table has no VIA column:\n%s", table)
	}
	if !strings.Contains(table, "cli-local") {
		t.Errorf("the table names no channel:\n%s", table)
	}
}

// The remote command line and tix mcp are told apart on the server, and each
// is found by its channel and by the token it used.
func TestTheServerTellsTheRemoteCLIFromMCP(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "seed the store")
	mint := func(name string) string {
		var created struct {
			Token string `json:"token"`
		}
		out := c.mustRun("token", "create", name, "--scope", "*", "-o", "json").out
		if err := json.Unmarshal([]byte(out), &created); err != nil {
			t.Fatalf("token create: %v (%s)", err, out)
		}
		return created.Token
	}
	laptop, agent := mint("laptop"), mint("agent")
	addr := freePort(t)
	done := make(chan result, 1)
	go func() { done <- c.run("serve", "--listen", addr) }()
	waitFor(t, "http://"+addr+"/healthz")
	defer stopServe(t, done)

	c.mustRun("--server", "http://"+addr, "--token", laptop, "task", "add", "from the laptop")
	replies := c.mcpSession(t, []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"task_create","arguments":{"title":"from the agent"}}}`,
	}, "--server", "http://"+addr, "--token", agent)
	if call := string(replies[2]); strings.Contains(call, `"isError":true`) {
		t.Fatalf("task_create over mcp failed: %s", call)
	}

	for _, tc := range []struct {
		via, token string
	}{{"cli-remote", "laptop"}, {"mcp", "agent"}} {
		got := viaEntries(t, c, "--via", tc.via, "--action", "task.create")
		if len(got) != 1 || got[0].Via.TokenName != tc.token {
			t.Errorf("--via %s selected %+v, want the one create on %s", tc.via, got, tc.token)
		}
		if byToken := viaEntries(t, c, "--token", tc.token, "--action", "task.create"); len(byToken) != 1 ||
			byToken[0].Via.Kind != core.ViaKind(tc.via) {
			t.Errorf("--token %s selected %+v", tc.token, byToken)
		}
	}
}
