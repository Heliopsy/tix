// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestTaskMove(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")
	c.mustRun("project", "create", "web", "Web")
	c.mustRun("task", "add", "-p", "infra", "rotate keys")
	c.mustRun("task", "add", "-p", "infra", "child", "--parent", "infra-1")
	c.mustRun("task", "add", "-p", "infra", "other")

	t.Run("dry run writes nothing", func(t *testing.T) {
		got := c.mustRun("task", "move", "infra-1", "--to", "web", "--dry-run", "-o", "json")
		if !strings.Contains(got.out, `"task.move"`) {
			t.Fatalf("plan = %s", got.out)
		}
		showTask(t, c, "infra-1")
	})
	t.Run("refusals exit with their codes", func(t *testing.T) {
		source := showTask(t, c, "infra-1")
		cases := []struct {
			args []string
			code int
		}{
			{[]string{"infra-1"}, core.ExitUsage},
			{[]string{"infra-1", "--to", "infra"}, core.ExitUsage},
			{[]string{"infra-1", "--to", "nope"}, core.ExitNotFound},
			{[]string{"infra-1", "--to", "web", "--version", strconv.Itoa(source.Version + 1)}, core.ExitConflict},
			{[]string{"infra-1", "infra-3", "--to", "web", "--version", "1"}, core.ExitUsage},
		}
		for _, tc := range cases {
			if got := c.run(append([]string{"task", "move"}, tc.args...)...); got.code != tc.code {
				t.Errorf("%v: exit = %d, want %d\n%s", tc.args, got.code, tc.code, got.err)
			}
		}
	})
	t.Run("across projects with the subtree", func(t *testing.T) {
		source := showTask(t, c, "infra-1")
		got := c.mustRun("task", "move", "infra-1", "--to", "web", "--version", strconv.Itoa(source.Version), "-o", "json")
		var moved core.Task
		if err := json.Unmarshal([]byte(got.out), &moved); err != nil {
			t.Fatalf("not json: %v\n%s", err, got.out)
		}
		if moved.Ref != "web-1" || moved.ID != source.ID {
			t.Fatalf("moved = %+v, want web-1 with id %s", moved, source.ID)
		}
		if child := showTask(t, c, "web-2"); child.Title != "child" || child.ParentID != source.ID {
			t.Fatalf("subtask = %+v, want child under %s", child, source.ID)
		}
		if got := c.run("task", "show", "infra-1"); got.code != core.ExitNotFound {
			t.Fatalf("the old reference exits %d, want %d", got.code, core.ExitNotFound)
		}
	})
}
