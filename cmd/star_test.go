// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func listedRefs(t *testing.T, c *cli, args ...string) ([]string, []bool) {
	t.Helper()
	got := c.mustRun(append([]string{"task", "ls", "-o", "json", "--sort", "seq"}, args...)...)
	var tasks []core.Task
	if err := json.Unmarshal([]byte(got.out), &tasks); err != nil {
		t.Fatalf("not json: %v\n%s", err, got.out)
	}
	refs := make([]string, len(tasks))
	starred := make([]bool, len(tasks))
	for i, task := range tasks {
		refs[i], starred[i] = task.Ref, task.Starred
	}
	return refs, starred
}

func TestTaskStarPutsTheTaskFirst(t *testing.T) {
	c := newCLI(t)
	for _, title := range []string{"one", "two", "three"} {
		c.mustRun("task", "add", title)
	}
	c.mustRun("task", "star", "default-3")

	cases := []struct {
		name        string
		args        []string
		wantRefs    []string
		wantStarred []bool
	}{
		{"starred leads", nil, []string{"default-3", "default-1", "default-2"}, []bool{true, false, false}},
		{"--starred", []string{"--starred"}, []string{"default-3"}, []bool{true}},
		{"--starred=false", []string{"--starred=false"}, []string{"default-1", "default-2"}, []bool{false, false}},
		{"is:starred", []string{"--filter", "is:starred"}, []string{"default-3"}, []bool{true}},
		{"descending keeps the group", []string{"--desc"}, []string{"default-3", "default-2", "default-1"}, []bool{true, false, false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			refs, starred := listedRefs(t, c, tc.args...)
			if !slices.Equal(refs, tc.wantRefs) || !slices.Equal(starred, tc.wantStarred) {
				t.Fatalf("listed %v starred %v, want %v %v", refs, starred, tc.wantRefs, tc.wantStarred)
			}
		})
	}

	show := c.mustRun("task", "show", "default-3", "-o", "json")
	var task core.Task
	if err := json.Unmarshal([]byte(show.out), &task); err != nil || !task.Starred {
		t.Fatalf("show -o json starred = %v (%v)\n%s", task.Starred, err, show.out)
	}
	table := c.mustRun("task", "ls", "--sort", "seq").out
	if !strings.Contains(table, "* default-3") || strings.Contains(table, "* default-1") {
		t.Fatalf("table marks the wrong rows:\n%s", table)
	}

	c.mustRun("task", "unstar", "default-3")
	if refs, _ := listedRefs(t, c); !slices.Equal(refs, []string{"default-1", "default-2", "default-3"}) {
		t.Fatalf("after unstar listed %v", refs)
	}
	c.mustRun("task", "unstar", "default-3")
}

func TestTaskStarDryRunAndErrors(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "one")
	c.mustRun("task", "star", "default-1", "--dry-run")
	if _, starred := listedRefs(t, c); starred[0] {
		t.Fatal("a dry run starred the task")
	}
	if got := c.run("task", "star", "missing-1"); got.code != core.ExitNotFound {
		t.Fatalf("star of an unknown task exited %d, want %d", got.code, core.ExitNotFound)
	}
}
