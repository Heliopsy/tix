// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func copiedTask(t *testing.T, c *cli, args ...string) core.Task {
	t.Helper()
	got := c.mustRun(append([]string{"task", "copy", "-o", "json"}, args...)...)
	var task core.Task
	if err := json.Unmarshal([]byte(got.out), &task); err != nil {
		t.Fatalf("not json: %v\n%s", err, got.out)
	}
	return task
}

func TestTaskCopy(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")
	c.mustRun("project", "create", "web", "Web")
	c.mustRun("task", "add", "-p", "infra", "rotate keys", "--tag", "sec")
	c.mustRun("task", "add", "-p", "infra", "child", "--parent", "infra-1")

	t.Run("across projects with subtasks", func(t *testing.T) {
		got := copiedTask(t, c, "infra-1", "--to", "web", "--with-subtasks")
		if got.Ref != "web-1" || got.Title != "rotate keys" || len(got.Tags) != 1 || got.CopiedFromID != "" {
			t.Fatalf("copy = %+v", got)
		}
		if child := showTask(t, c, "web-2"); child.Title != "child" || child.ParentID != got.ID {
			t.Fatalf("subtask copy = %+v, want child under %s", child, got.ID)
		}
	})
	t.Run("in place with a new title", func(t *testing.T) {
		got := copiedTask(t, c, "infra-1", "--title", "rotate again")
		if got.Ref != "infra-3" || got.Title != "rotate again" {
			t.Fatalf("copy = %+v", got)
		}
		if n := len(refsOf(t, c.mustRun("task", "ls", "-p", "infra", "-o", "ndjson").out)); n != 3 {
			t.Fatalf("infra holds %d tasks, want 3: the subtree came along without --with-subtasks", n)
		}
	})
	t.Run("dry run writes nothing", func(t *testing.T) {
		got := c.mustRun("task", "copy", "infra-1", "--to", "web", "--dry-run", "-o", "json")
		if !strings.Contains(got.out, `"task.copy"`) {
			t.Fatalf("plan = %s", got.out)
		}
		if n := len(refsOf(t, c.mustRun("task", "ls", "-p", "web", "-o", "ndjson").out)); n != 2 {
			t.Fatalf("web holds %d tasks after a dry run, want 2", n)
		}
	})
	t.Run("show names the original of a linked copy", func(t *testing.T) {
		got := copiedTask(t, c, "infra-1", "--to", "web", "--link")
		source := showTask(t, c, "infra-1")
		if got.CopiedFromID != source.ID {
			t.Fatalf("copied_from_id = %q, want %q", got.CopiedFromID, source.ID)
		}
		shown := c.mustRun("task", "show", got.Ref)
		if !strings.Contains(shown.err, "copied from: INFRA-1") {
			t.Fatalf("task show does not name the original: %q", shown.err)
		}
	})
	t.Run("unknown project", func(t *testing.T) {
		for _, args := range [][]string{{"--to", "nope"}, {"--to", "nope", "--dry-run"}} {
			got := c.run(append([]string{"task", "copy", "infra-1"}, args...)...)
			if got.code != core.ExitNotFound {
				t.Fatalf("%v: exit = %d, want %d\n%s", args, got.code, core.ExitNotFound, got.err)
			}
		}
	})
}
