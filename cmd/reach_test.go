// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestFieldPutSetsADefault(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")
	c.mustRun("field", "put", "infra", "severity", "--type", "enum",
		"--option", "low", "--option", "high", "--default", "low")
	c.mustRun("field", "put", "infra", "points", "--type", "int", "--default", "3")
	c.mustRun("field", "put", "infra", "code", "--default", "42")
	c.mustRun("task", "add", "-p", "infra", "sized")

	var task struct {
		CustomFields map[string]any `json:"custom_fields"`
	}
	if err := json.Unmarshal([]byte(c.mustRun("task", "show", "infra-1", "-o", "json").out), &task); err != nil {
		t.Fatalf("not json: %v", err)
	}
	want := map[string]any{"severity": "low", "points": float64(3), "code": "42"}
	for key, value := range want {
		if task.CustomFields[key] != value {
			t.Errorf("custom field %s = %#v, want %#v", key, task.CustomFields[key], value)
		}
	}
	bad := c.run("field", "put", "infra", "tier", "--type", "enum", "--option", "a", "--default", "b")
	if bad.code != core.ExitUsage {
		t.Fatalf("a default outside the options exited %d, want %d", bad.code, core.ExitUsage)
	}
}

func TestTaskMvHonoursAVersion(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "alpha")
	c.mustRun("task", "add", "beta")
	version := func() int {
		var task struct {
			Version int `json:"version"`
		}
		if err := json.Unmarshal([]byte(c.mustRun("task", "show", "default-1", "-o", "json").out), &task); err != nil {
			t.Fatalf("not json: %v", err)
		}
		return task.Version
	}
	current := version()
	stale := c.run("task", "mv", "default-1", "doing", "--version", "999")
	if stale.code != core.ExitConflict {
		t.Fatalf("a stale version exited %d, want %d: %s", stale.code, core.ExitConflict, stale.err)
	}
	staleHops := c.run("task", "mv", "default-1", "done", "--hops", "--version", "999")
	if staleHops.code != core.ExitConflict {
		t.Fatalf("a stale version with --hops exited %d, want %d: %s", staleHops.code, core.ExitConflict, staleHops.err)
	}
	if version() != current {
		t.Fatal("a refused move changed the task")
	}
	two := c.run("task", "mv", "default-1", "default-2", "doing", "--version", "1")
	if two.code != core.ExitUsage {
		t.Fatalf("--version over two tasks exited %d, want %d", two.code, core.ExitUsage)
	}
	c.mustRun("task", "mv", "default-1", "doing", "--version", itoa(current))
	if version() == current {
		t.Fatal("the move at the current version did not land")
	}
}

// nextCursor is the cursor a listing announces on its diagnostic stream.
var nextCursor = regexp.MustCompile(`next cursor (\S+)`)

func TestListingsResumeFromTheCursorTheyAnnounce(t *testing.T) {
	c := newCLI(t)
	for _, key := range []string{"alpha", "beta"} {
		c.mustRun("project", "create", key, key)
		c.mustRun("user", "create", key+"@example.test", "--handle", key, "--password", "correct horse battery")
	}
	for _, list := range [][]string{
		{"project", "ls"}, {"user", "ls"}, {"actor", "ls"},
	} {
		t.Run(strings.Join(list, " "), func(t *testing.T) {
			first := c.mustRun(append(list, "--limit", "1", "-o", "json")...)
			m := nextCursor.FindStringSubmatch(first.err)
			if m == nil {
				t.Fatalf("one row of several announced no cursor: %s", first.err)
			}
			second := c.mustRun(append(list, "--limit", "1", "--cursor", m[1], "-o", "json")...)
			if ids(t, first.out) == ids(t, second.out) {
				t.Fatalf("the cursor returned the first page again: %s", second.out)
			}
		})
	}
	c.mustRun("webhook", "deliveries", "--cursor", "")
	keyed := ids(t, c.mustRun("project", "ls", "--key", "beta", "-o", "json").out)
	if !strings.Contains(keyed, "beta") || strings.Contains(keyed, "alpha") {
		t.Fatalf("project ls --key beta listed %s", keyed)
	}
}

// ids joins the identifying fields of a JSON listing, one array or one
// object per line.
func ids(t *testing.T, out string) string {
	t.Helper()
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			var row map[string]any
			if err := json.Unmarshal([]byte(line), &row); err != nil {
				t.Fatalf("not json: %v\n%s", err, out)
			}
			rows = append(rows, row)
		}
	}
	var parts []string
	for _, row := range rows {
		for _, field := range []string{"key", "email", "handle", "id"} {
			if v, ok := row[field].(string); ok {
				parts = append(parts, v)
				break
			}
		}
	}
	return strings.Join(parts, ",")
}
