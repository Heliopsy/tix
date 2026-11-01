// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

func runJSON[T any](t *testing.T, c *cli, args ...string) T {
	t.Helper()
	got := c.mustRun(append(args, "-o", "json")...)
	var out T
	if err := json.Unmarshal([]byte(got.out), &out); err != nil {
		t.Fatalf("not json: %v\n%s", err, got.out)
	}
	return out
}

func TestTemplateCommands(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")
	c.mustRun("project", "create", "web", "Web")
	c.mustRun("task", "add", "-p", "infra", "release {N}", "--tag", "ops")
	c.mustRun("task", "add", "-p", "infra", "notes", "--parent", "infra-1")

	saved := runJSON[core.TaskTemplate](t, c, "template", "save", "infra-1", "--name", "release", "--with-subtasks", "--due-in", "3d")
	if saved.Name != "release" || saved.ProjectKey != "infra" || len(saved.Task.Subtasks) != 1 || saved.DueIn != core.Duration(72*time.Hour) {
		t.Fatalf("saved = %+v", saved)
	}

	t.Run("ls and show", func(t *testing.T) {
		if got := c.mustRun("template", "ls"); !strings.Contains(got.out, "release") || !strings.Contains(got.out, "3d") {
			t.Fatalf("ls = %s", got.out)
		}
		if shown := runJSON[core.TaskTemplate](t, c, "template", "show", "RELEASE"); shown.ID != saved.ID {
			t.Fatalf("show = %+v", shown)
		}
	})
	t.Run("use creates the tree and fills the placeholders", func(t *testing.T) {
		got := runJSON[core.Task](t, c, "template", "use", "release", "--project", "web")
		if got.Ref != "web-1" || got.Title != "release 1" || got.DueAt == nil {
			t.Fatalf("use = %+v", got)
		}
		if child := showTask(t, c, "web-2"); child.Title != "notes" || child.ParentID != got.ID {
			t.Fatalf("subtask = %+v", child)
		}
		if n := len(refsOf(t, c.mustRun("task", "ls", "-p", "infra", "-o", "ndjson").out)); n != 2 {
			t.Fatalf("infra holds %d tasks, want the 2 it had", n)
		}
	})
	t.Run("use with --no-due and a title", func(t *testing.T) {
		got := runJSON[core.Task](t, c, "template", "use", "release", "--no-due", "--title", "hotfix {N}")
		if got.Title != "hotfix 2" || got.DueAt != nil || !strings.HasPrefix(got.Ref, "infra-") {
			t.Fatalf("use = %+v", got)
		}
		if r := c.run("template", "use", "release", "--no-due", "--due-in", "1d"); r.code != core.ExitUsage {
			t.Fatalf("--no-due with --due-in: exit %d", r.code)
		}
	})
	t.Run("edit renames and rescopes", func(t *testing.T) {
		got := runJSON[core.TaskTemplate](t, c, "template", "edit", "release", "--name", "ship", "--tenant-wide",
			"--tag", "ops", "--tag", "release", "--due-in", "0")
		if got.Name != "ship" || !got.TenantWide() || len(got.Task.Tags) != 2 || got.DueIn != 0 {
			t.Fatalf("edit = %+v", got)
		}
		if r := c.run("template", "edit", "ship", "--project", "web", "--tenant-wide"); r.code != core.ExitUsage {
			t.Fatalf("--project with --tenant-wide: exit %d", r.code)
		}
	})
	t.Run("a taken name is a conflict", func(t *testing.T) {
		if r := c.run("template", "save", "infra-1", "--name", "SHIP"); r.code != core.ExitConflict {
			t.Fatalf("exit = %d, want %d\n%s", r.code, core.ExitConflict, r.err)
		}
	})
	t.Run("rm, with a dry run first", func(t *testing.T) {
		if got := c.mustRun("template", "rm", "ship", "--dry-run", "-o", "json"); !strings.Contains(got.out, `"template.delete"`) {
			t.Fatalf("plan = %s", got.out)
		}
		c.mustRun("template", "rm", "ship")
		if r := c.run("template", "show", "ship"); r.code != core.ExitNotFound {
			t.Fatalf("show after rm: exit %d", r.code)
		}
	})
}
