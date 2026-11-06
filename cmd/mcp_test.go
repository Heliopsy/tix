// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func mcpStates(t *testing.T, out string) map[string]string {
	t.Helper()
	var rows []struct {
		Tool  string `json:"tool"`
		State string `json:"state"`
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("tix mcp --list -o json: %v\n%s", err, out)
	}
	states := map[string]string{}
	for _, r := range rows {
		states[r.Tool] = r.State
	}
	return states
}

func TestMCPListDescribesThePlanWithoutAServer(t *testing.T) {
	c := newCLI(t)
	off := mcpStates(t, c.mustRun("mcp", "--list", "-o", "json").out)
	on := mcpStates(t, c.mustRun("mcp", "--list", "--allow-destructive", "-o", "json").out)
	for tool, want := range map[string][2]string{
		"task_create":   {"exposed", "exposed"},
		"task_delete":   {"destructive", "exposed"},
		"session_login": {"hidden", "hidden"},
	} {
		if off[tool] != want[0] || on[tool] != want[1] {
			t.Errorf("%s is %q, and %q with --allow-destructive; want %q and %q", tool, off[tool], on[tool], want[0], want[1])
		}
	}
	if _, err := os.Stat(c.dbPath()); !os.IsNotExist(err) {
		t.Errorf("tix mcp --list touched the zero-config store: %v", err)
	}
}

func TestMCPRefusesWhatItCannotServe(t *testing.T) {
	c := newCLI(t)
	if got := c.run("mcp"); got.code != core.ExitUsage || !strings.Contains(got.err, "talks to a tix server") {
		t.Errorf("tix mcp with no server exited %d: %s", got.code, got.err)
	}
	if _, err := os.Stat(c.dbPath()); !os.IsNotExist(err) {
		t.Errorf("tix mcp with no server opened the zero-config store: %v", err)
	}
	bad := filepath.Join(c.home, "overlay.yaml")
	if err := os.WriteFile(bad, []byte("operations:\n  task.nope: {hide: true, reason: x}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := c.run("mcp", "--list", "--overlay", bad); got.code != core.ExitUsage || !strings.Contains(got.err, "task.nope") {
		t.Errorf("tix mcp with an overlay naming no operation exited %d: %s", got.code, got.err)
	}
}

// TestMCPServesTheAPIOverStdio drives the real command end to end: a JSON-RPC
// session on standard input against a running `tix serve` on a scratch store.
func TestMCPServesTheAPIOverStdio(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "write the mcp docs")
	var created struct {
		Token string `json:"token"`
	}
	out := c.mustRun("token", "create", "mcp", "--scope", "task:read", "-o", "json").out
	if err := json.Unmarshal([]byte(out), &created); err != nil {
		t.Fatalf("token create: %v (%s)", err, out)
	}
	addr := freePort(t)
	done := make(chan result, 1)
	go func() { done <- c.run("serve", "--listen", addr) }()
	waitFor(t, "http://"+addr+"/healthz")
	defer stopServe(t, done)

	session := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"task_list","arguments":{}}}`,
	}
	replies := c.mcpSession(t, session, "--server", "http://"+addr, "--token", created.Token)
	tools := string(replies[2])
	if !strings.Contains(tools, `"name":"task_list"`) || strings.Contains(tools, `"name":"task_create"`) {
		t.Errorf("a task:read token was offered the wrong tools: %s", tools)
	}
	if call := string(replies[3]); !strings.Contains(call, "write the mcp docs") || strings.Contains(call, `"isError":true`) {
		t.Errorf("task_list over stdio answered %s", call)
	}
}

// mcpSession runs tix mcp and sends session one line at a time, waiting for
// the reply to each request before sending the next, as a client does: the
// server answers concurrently, so a burst can be answered out of order or
// reach tools/list before initialize has finished.
func (c *cli) mcpSession(t *testing.T, session []string, args ...string) map[float64]json.RawMessage {
	t.Helper()
	stdin, feed := io.Pipe()
	replies, writer := io.Pipe()
	done := make(chan int, 1)
	var errb bytes.Buffer
	go func() {
		done <- Run(append([]string{"mcp"}, args...), stdin, writer, &errb, c.environ(), c.home)
		_ = writer.Close()
	}()

	got := map[float64]json.RawMessage{}
	scanner := bufio.NewScanner(replies)
	scanner.Buffer(nil, 1<<22)
	for _, line := range session {
		var req struct {
			ID *float64 `json:"id"`
		}
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			t.Fatalf("session line %q: %v", line, err)
		}
		if _, err := io.WriteString(feed, line+"\n"); err != nil {
			t.Fatalf("writing to tix mcp: %v (%s)", err, errb.String())
		}
		if req.ID == nil {
			continue
		}
		for got[*req.ID] == nil {
			if !scanner.Scan() {
				t.Fatalf("tix mcp closed its output before answering request %v: %s", *req.ID, errb.String())
			}
			var msg struct {
				ID     float64         `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  json.RawMessage `json:"error"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
				t.Fatalf("tix mcp wrote a line that is not JSON-RPC: %q", scanner.Text())
			}
			if msg.Error != nil {
				t.Fatalf("tix mcp answered request %v with an error: %s", msg.ID, msg.Error)
			}
			got[msg.ID] = msg.Result
		}
	}
	_ = feed.Close()
	go func() { _, _ = io.Copy(io.Discard, replies) }()
	if code := <-done; code != core.ExitOK {
		t.Fatalf("tix mcp exited %d: %s", code, errb.String())
	}
	return got
}

func TestMCPOffersGlobalAdminToolsOnlyToAGlobalAdmin(t *testing.T) {
	named := []core.Scope{core.ScopeAll, core.ScopeGlobalAdmin}
	tests := []struct {
		name  string
		actor *core.Actor
		want  bool
	}{
		{"names the scope and is one", &core.Actor{Scopes: named, GlobalAdmin: true}, true},
		{"names the scope and is not one", &core.Actor{Scopes: named}, false},
		{"is one but the token holds only *", &core.Actor{Scopes: []core.Scope{core.ScopeAll}, GlobalAdmin: true}, false},
	}
	for _, tc := range tests {
		if got := mcpHasScope(tc.actor, core.ScopeGlobalAdmin); got != tc.want {
			t.Errorf("%s: offered = %v, want %v", tc.name, got, tc.want)
		}
	}
	if !mcpHasScope(&core.Actor{Scopes: []core.Scope{core.ScopeAll}}, core.ScopeTenantAdmin) {
		t.Error("an ordinary scope stopped being offered")
	}
}
