// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestTaskMove(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")
	c.mustRun("project", "create", "web", "Web")
	for _, title := range []string{"one", "two", "three"} {
		c.mustRun("task", "add", "-p", "infra", title)
	}

	var moved core.TaskMove
	if err := json.Unmarshal([]byte(c.mustRun("task", "move", "infra-1", "--to", "web", "-o", "json").out), &moved); err != nil {
		t.Fatal(err)
	}
	if moved.Task.Ref != "web-1" || moved.FromRef != "infra-1" || !moved.StatusKept {
		t.Fatalf("move = %+v, want infra-1 now web-1 with its status", moved)
	}
	if got := showTask(t, c, "infra-1"); got.Ref != "web-1" || got.Title != "one" {
		t.Fatalf("the old reference shows %+v, want the moved task", got)
	}

	rows := outcomes(t, c.mustRun("task", "move", "infra-2,infra-3", "--to", "web", "-o", "json").out)
	if len(rows) != 2 || rows[0].Moved != "web-2" || rows[1].Moved != "web-3" {
		t.Fatalf("a comma list moved %+v, want web-2 and web-3", rows)
	}

	if got := c.run("task", "move", "web-1"); got.code != core.ExitUsage {
		t.Errorf("no --to: exit = %d, want usage", got.code)
	}
	if got := c.run("task", "move", "web-1", "--to", "web"); got.code != core.ExitUsage {
		t.Errorf("own project: exit = %d, want usage", got.code)
	}
	plan := c.mustRun("task", "move", "web-1", "--to", "infra", "--dry-run", "-o", "json")
	if !strings.Contains(plan.out, `"task.move"`) || showTask(t, c, "web-1").Ref != "web-1" {
		t.Errorf("dry run = %s, or it moved the task", plan.out)
	}
}
