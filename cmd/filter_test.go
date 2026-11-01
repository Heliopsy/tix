// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func savedFilters(t *testing.T, c *cli) []core.SavedFilter {
	t.Helper()
	got := c.mustRun("filter", "ls", "-o", "json")
	var out []core.SavedFilter
	if err := json.Unmarshal([]byte(got.out), &out); err != nil {
		t.Fatalf("not json: %v\n%s", err, got.out)
	}
	return out
}

func taskTitles(t *testing.T, c *cli, args ...string) []string {
	t.Helper()
	got := c.mustRun(append(append([]string{"task", "ls"}, args...), "-o", "json")...)
	var tasks []core.Task
	if err := json.Unmarshal([]byte(got.out), &tasks); err != nil {
		t.Fatalf("not json: %v\n%s", err, got.out)
	}
	out := make([]string, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, task.Title)
	}
	return out
}

func TestFilterCommandsKeepNamedFilters(t *testing.T) {
	c := newCLI(t)
	c.mustRun("filter", "save", "ops", "--filter", "tag:ops", "--view", "board", "--sort", "priority")
	c.mustRun("filter", "save", "mine", "--filter", "assignee:me", "--shared")
	list := savedFilters(t, c)
	if len(list) != 2 || list[0].Name != "ops" || list[0].View != "board" || list[0].Sort != "priority" || !list[1].Shared {
		t.Fatalf("ls = %+v", list)
	}
	if show := c.mustRun("filter", "show", "OPS"); !strings.Contains(show.out, "tag:ops") {
		t.Fatalf("show printed %q", show.out)
	}

	c.mustRun("filter", "rename", "ops", "ops board")
	c.mustRun("filter", "set", "ops board", "--filter", "tag:ops -status:done", "--shared")
	c.mustRun("filter", "order", "mine")
	list = savedFilters(t, c)
	if list[0].Name != "mine" || list[1].Name != "ops board" || list[1].Query != "tag:ops -status:done" || !list[1].Shared {
		t.Fatalf("after rename, set and order: %+v", list)
	}

	c.mustRun("filter", "rm", "mine", "--dry-run")
	if n := len(savedFilters(t, c)); n != 2 {
		t.Fatalf("a dry run removed a filter: %d left", n)
	}
	c.mustRun("filter", "rm", "mine")
	if n := len(savedFilters(t, c)); n != 1 {
		t.Fatalf("after rm: %d left", n)
	}

	cases := []struct {
		name string
		args []string
		code int
	}{
		{"set with no flag", []string{"filter", "set", "ops board"}, core.ExitUsage},
		{"a taken name", []string{"filter", "save", "OPS BOARD"}, core.ExitConflict},
		{"an unknown view", []string{"filter", "save", "x", "--view", "calendar"}, core.ExitUsage},
		{"an unreadable expression", []string{"filter", "save", "x", "--filter", "nokey:1"}, core.ExitUsage},
		{"no such filter", []string{"filter", "show", "nope"}, core.ExitNotFound},
		{"listing from no such filter", []string{"task", "ls", "--saved", "nope"}, core.ExitNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := c.run(tc.args...); got.code != tc.code {
				t.Fatalf("exited %d, want %d\n%s", got.code, tc.code, got.err)
			}
		})
	}
}

func TestTaskLsStartsFromASavedFilterAndNamesMine(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "b ops", "--tag", "ops", "--priority", "low")
	c.mustRun("task", "add", "a ops", "--tag", "ops", "--priority", "highest", "--assignee", "local")
	c.mustRun("task", "add", "other", "--assignee", "local")
	c.mustRun("filter", "save", "ops", "--filter", "tag:ops", "--sort", "title")

	if got := taskTitles(t, c, "--saved", "ops"); len(got) != 2 || got[0] != "a ops" || got[1] != "b ops" {
		t.Fatalf("--saved ops = %v, want both ops tasks by title", got)
	}
	if got := taskTitles(t, c, "--saved", "ops", "--sort", "priority", "--desc"); len(got) != 2 || got[0] != "b ops" {
		t.Fatalf("a typed sort does not win over the saved one: %v", got)
	}
	if got := taskTitles(t, c, "--saved", "ops", "--mine"); len(got) != 1 || got[0] != "a ops" {
		t.Fatalf("--saved ops --mine = %v", got)
	}
	if got := taskTitles(t, c, "--mine"); len(got) != 2 {
		t.Fatalf("--mine = %v, want the two tasks assigned to the caller", got)
	}
	if got := taskTitles(t, c, "--filter", "-assignee:me"); len(got) != 1 || got[0] != "b ops" {
		t.Fatalf("-assignee:me = %v", got)
	}
}
