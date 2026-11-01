// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// A project key is typed by hand in whatever case the reader sees it in, and
// the CLI shows it upper case, so every argument that names a project or a task
// accepts any case and reaches the same stored record.
func TestProjectKeysAndRefsAreCaseInsensitiveOnTheCommandLine(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")
	c.mustRun("project", "create", "web", "Web")
	c.mustRun("task", "add", "patch the kernel", "--project", "infra")

	tests := []struct {
		name string
		args []string
		ref  string
	}{
		{"task add --project", []string{"task", "add", "rotate keys", "--project", "INFRA"}, "infra-2"},
		{"task add --project, mixed case", []string{"task", "add", "renew certs", "--project", "Infra"}, "infra-3"},
		{"task show REF", []string{"task", "show", "INFRA-1"}, "infra-1"},
		{"task show ref, mixed case", []string{"task", "show", "Infra-1"}, "infra-1"},
		{"task copy --to", []string{"task", "copy", "INFRA-1", "--to", "WEB"}, "web-1"},
		{"template use --project", []string{"template", "use", "kernel", "--project", "WEB"}, "web-2"},
	}
	c.mustRun("template", "save", "infra-1", "--name", "kernel")
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := runJSON[core.Task](t, c, tc.args...)
			if got.Ref != tc.ref {
				t.Fatalf("tix %s returned %q, want %q", strings.Join(tc.args, " "), got.Ref, tc.ref)
			}
		})
	}

	for _, args := range [][]string{
		{"project", "show", "INFRA"},
		{"project", "show", "infra"},
	} {
		if p := decodeProject(t, c.mustRun(append(args, "-o", "json")...).out); p.Key != "infra" {
			t.Errorf("tix %s returned key %q, want the stored infra", strings.Join(args, " "), p.Key)
		}
	}

	for _, args := range [][]string{
		{"task", "ls", "--filter", "project:INFRA"},
		{"task", "ls", "--filter", "project:infra"},
		{"task", "ls", "--project", "INFRA"},
	} {
		tasks := runJSON[[]core.Task](t, c, args...)
		if len(tasks) != 3 {
			t.Errorf("tix %s listed %d tasks, want infra's 3", strings.Join(args, " "), len(tasks))
		}
		for _, task := range tasks {
			if !strings.HasPrefix(task.Ref, "infra-") {
				t.Errorf("tix %s listed %q, and its JSON ref is no longer the stored lower case", strings.Join(args, " "), task.Ref)
			}
		}
	}

	if got := c.mustRun("stats", "-p", "INFRA").out; !strings.Contains(got, "project INFRA") {
		t.Errorf("stats -p INFRA does not name the project:\n%s", got)
	}
}

// The table and text output show a project key upper case, while the JSON a
// script reads keeps the stored value it always had.
func TestTablesShowKeysUpperCaseAndJSONKeepsTheStoredValue(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")
	c.mustRun("project", "create", "web", "Web")
	c.mustRun("task", "add", "patch the kernel", "--project", "infra")

	tests := []struct {
		name  string
		args  []string
		shown string
	}{
		{"task ls", []string{"task", "ls"}, "INFRA-1"},
		{"task show", []string{"task", "show", "infra-1"}, "INFRA-1"},
		{"project ls", []string{"project", "ls"}, "INFRA"},
		{"bulk outcome", []string{"task", "bulk", "priority", "infra-1", "--priority", "high"}, "INFRA-1"},
		{"bulk copy names the copy", []string{"task", "bulk", "copy", "infra-1", "--project", "web"}, "WEB-1"},
		{"tag outcome", []string{"tag", "add", "infra-1", "ops"}, "INFRA-1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := c.mustRun(tc.args...).out
			if !strings.Contains(out, tc.shown) {
				t.Fatalf("tix %s does not show %q:\n%s", strings.Join(tc.args, " "), tc.shown, out)
			}
			if strings.Contains(out, strings.ToLower(tc.shown)+" ") {
				t.Fatalf("tix %s still shows the lower-case %q:\n%s", strings.Join(tc.args, " "), strings.ToLower(tc.shown), out)
			}
		})
	}

	if task := runJSON[core.Task](t, c, "task", "show", "INFRA-1"); task.Ref != "infra-1" {
		t.Fatalf("task show -o json ref = %q, want the stored infra-1", task.Ref)
	}
}
