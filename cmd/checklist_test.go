// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func shownTask(t *testing.T, c *cli, ref string) core.Task {
	t.Helper()
	got := c.mustRun("task", "show", ref, "-o", "json")
	var task core.Task
	if err := json.Unmarshal([]byte(got.out), &task); err != nil {
		t.Fatalf("not json: %v\n%s", err, got.out)
	}
	return task
}

func TestTaskCheckTicksOneItem(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "rotate the keys", "--body", "- [ ] revoke\n- [x] issue\n- [ ] announce\n")

	task := shownTask(t, c, "default-1")
	if task.Checklist == nil || *task.Checklist != (core.Checklist{Done: 1, Total: 3}) {
		t.Fatalf("checklist = %+v, want 1/3", task.Checklist)
	}
	if got := c.mustRun("task", "show", "default-1"); !strings.Contains(got.err, "checklist: 1/3 done") {
		t.Fatalf("task show does not state the checklist:\n%s", got.err)
	}

	c.mustRun("task", "check", "default-1", "3")
	c.mustRun("task", "uncheck", "default-1", "2")
	task = shownTask(t, c, "default-1")
	if task.Body != "- [ ] revoke\n- [ ] issue\n- [x] announce\n" {
		t.Fatalf("body after check 3 and uncheck 2 = %q", task.Body)
	}

	stale := c.run("task", "check", "default-1", "1", "--version", "1")
	if stale.code != 4 {
		t.Fatalf("a stale version exits %d, want 4:\n%s", stale.code, stale.err)
	}
	missing := c.run("task", "check", "default-1", "9")
	if missing.code != 2 || !strings.Contains(missing.err, "has 3") {
		t.Fatalf("an item past the end exits %d, want 2 naming the total:\n%s", missing.code, missing.err)
	}
	if bad := c.run("task", "check", "default-1", "first"); bad.code != 2 {
		t.Fatalf("a non-number exits %d, want 2", bad.code)
	}
}

func TestChecklistFiltersListTheRightTasks(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "open", "--body", "- [ ] one\n")
	c.mustRun("task", "add", "done", "--body", "- [x] one\n")
	c.mustRun("task", "add", "none")
	cases := map[string][]string{
		"has:checklist":  {"default-1", "default-2"},
		"-has:checklist": {"default-3"},
		"checklist:open": {"default-1"},
		"checklist:done": {"default-2"},
	}
	for expr, want := range cases {
		refs, _ := listedRefs(t, c, "--filter", expr)
		if strings.Join(refs, ",") != strings.Join(want, ",") {
			t.Errorf("%q listed %v, want %v", expr, refs, want)
		}
	}
}
