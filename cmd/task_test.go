// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// runEnv runs one invocation with extra variables added to the environment.
func runEnv(c *cli, extra []string, args ...string) result {
	c.t.Helper()
	var out, errb bytes.Buffer
	code := Run(args, strings.NewReader(""), &out, &errb, append(c.environ(), extra...), c.home)
	return result{code: code, out: out.String(), err: errb.String()}
}

// writeFile writes a file under the throwaway home directory.
func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}

func TestDefaultProjectPrecedence(t *testing.T) {
	cases := []struct {
		name   string
		flag   bool
		env    bool
		dotenv bool
		file   bool
		want   string
	}{
		{"flag beats all", true, true, true, true, "flagp"},
		{"env beats dotenv and file", false, true, true, true, "envp"},
		{"dotenv beats file", false, false, true, true, "dotenvp"},
		{"file beats default", false, false, false, true, "filep"},
		{"default when nothing set", false, false, false, false, "default"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newCLI(t)
			if tc.want != "default" {
				c.mustRun("project", "create", tc.want, strings.ToUpper(tc.want))
			}
			if tc.file {
				writeFile(t, filepath.Join(c.home, "conf", "tix", "config.yaml"), "project: filep\n")
			}
			if tc.dotenv {
				writeFile(t, filepath.Join(c.home, ".env"), "TIX_PROJECT=dotenvp\n")
			}
			args := []string{"task", "add", "-o", "json", "write the report"}
			if tc.flag {
				args = append([]string{"task", "add", "-p", "flagp", "-o", "json"}, "write the report")
			}
			var extra []string
			if tc.env {
				extra = []string{"TIX_PROJECT=envp"}
			}

			got := runEnv(c, extra, args...)
			if got.code != core.ExitOK {
				t.Fatalf("task add exited %d: %s", got.code, got.err)
			}
			var task core.Task
			if err := json.Unmarshal([]byte(got.out), &task); err != nil {
				t.Fatalf("not json: %v\n%s", err, got.out)
			}
			if !strings.HasPrefix(task.Ref, tc.want+"-") {
				t.Fatalf("ref = %q, want it in project %q", task.Ref, tc.want)
			}
		})
	}
}

func TestDefaultProjectFiltersTaskLs(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infra")
	c.mustRun("task", "add", "-p", "default", "shared task")
	c.mustRun("task", "add", "-p", "infra", "infra task")

	writeFile(t, filepath.Join(c.home, "conf", "tix", "config.yaml"), "project: infra\n")

	scoped := c.mustRun("task", "ls", "-o", "json").out
	if !strings.Contains(scoped, "infra task") {
		t.Fatalf("task ls = %q, want the default project's task", scoped)
	}
	if strings.Contains(scoped, "shared task") {
		t.Fatalf("task ls = %q, want it scoped to the default project", scoped)
	}

	explicit := c.mustRun("task", "ls", "-p", "default", "-o", "json").out
	if !strings.Contains(explicit, "shared task") || strings.Contains(explicit, "infra task") {
		t.Fatalf("task ls -p default = %q, want the flag to win", explicit)
	}
}

func TestConfigShowReportsTheProjectSource(t *testing.T) {
	c := newCLI(t)
	writeFile(t, filepath.Join(c.home, "conf", "tix", "config.yaml"), "project: filep\n")

	fromFile := c.mustRun("config", "show", "--sources", "-o", "json").out
	if !strings.Contains(fromFile, `"project"`) || !strings.Contains(fromFile, "file") {
		t.Fatalf("config show = %q, want project from the file layer", fromFile)
	}

	fromEnv := runEnv(c, []string{"TIX_PROJECT=envp"}, "config", "show", "--sources", "-o", "json")
	if fromEnv.code != core.ExitOK {
		t.Fatalf("config show exited %d: %s", fromEnv.code, fromEnv.err)
	}
	if !strings.Contains(fromEnv.out, "envp") {
		t.Fatalf("config show = %q, want the environment value", fromEnv.out)
	}
}

func TestUnimplementedModesExitTwo(t *testing.T) {
	cases := []struct {
		name      string
		variable  string
		key       string
		supported string
	}{
		{"auth mode", "TIX_AUTH_MODE=oidc", "auth.mode", "token"},
		{"hook mode", "TIX_HOOKS_MODE=enforce", "hooks.mode", "off"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newCLI(t)
			got := runEnv(c, []string{tc.variable}, "task", "ls")
			if got.code != core.ExitUsage {
				t.Fatalf("exit = %d, want %d\n%s", got.code, core.ExitUsage, got.err)
			}
			if !strings.Contains(got.err, tc.key) || !strings.Contains(got.err, "not implemented") {
				t.Fatalf("stderr = %q, want it to refuse %q", got.err, tc.key)
			}
			if !strings.Contains(got.err, tc.supported) {
				t.Fatalf("stderr = %q, want it to name %q as supported", got.err, tc.supported)
			}
		})
	}
}

// --filter is the whole expression language on the command line, so the terms
// a person types into the terminal interface's filter bar select the same
// tasks from a shell. Negation and the weak match are the two that had no
// flag at all before.
func TestTaskLsFilterExpression(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "Deploy the API gateway", "--tag", "ops")
	c.mustRun("task", "add", "rotate API keys", "--tag", "sec")
	c.mustRun("task", "add", "write runbook", "--tag", "ops")

	tests := []struct {
		name   string
		filter string
		want   []string
	}{
		{"an excluded tag", "-tag:ops", []string{"rotate API keys"}},
		{"two excluded tags leave nothing", "-tag:ops -tag:sec", nil},
		{"a weak title match", "title~api", []string{"Deploy the API gateway", "rotate API keys"}},
		{"a weak match ignores case", "title~API", []string{"Deploy the API gateway", "rotate API keys"}},
		{"a negated weak title match", "-title~api", []string{"write runbook"}},
		{"an exact title match", `title:"write runbook"`, []string{"write runbook"}},
		{"an exact match rejects a substring", "title:api", nil},
		{"selection and exclusion combine", "status:todo -tag:sec",
			[]string{"Deploy the API gateway", "write runbook"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := c.mustRun("task", "ls", "--filter", tt.filter, "-o", "json").out
			var tasks []core.Task
			if err := json.Unmarshal([]byte(out), &tasks); err != nil {
				t.Fatalf("decoding: %v\n%s", err, out)
			}
			got := make([]string, 0, len(tasks))
			for _, task := range tasks {
				got = append(got, task.Title)
			}
			sort.Strings(got)
			want := append([]string(nil), tt.want...)
			sort.Strings(want)
			if len(got) != len(want) {
				t.Fatalf("filter %q selected %v, want %v", tt.filter, got, want)
			}
			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("filter %q selected %v, want %v", tt.filter, got, want)
				}
			}
		})
	}
}

// A filter the parser cannot read is a usage error, not an empty listing: an
// empty listing reads as "nothing matched", which is the wrong answer.
func TestTaskLsRejectsAnUnreadableFilter(t *testing.T) {
	c := newCLI(t)
	for _, expr := range []string{"colour:red", "status~todo", "-sort:title"} {
		if got := c.run("task", "ls", "--filter", expr); got.code == core.ExitOK {
			t.Errorf("filter %q was accepted: %s", expr, got.out)
		}
	}
}

// The flags and the expression add up rather than one silently replacing the
// other, so an alias or a shell function carrying --filter still composes.
func TestTaskLsFilterAndFlagsCombine(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "keep me", "--tag", "ops")
	c.mustRun("task", "add", "drop me", "--tag", "ops")

	out := c.mustRun("task", "ls", "--filter", "-title~drop", "--tag", "ops", "-o", "json").out
	var tasks []core.Task
	if err := json.Unmarshal([]byte(out), &tasks); err != nil {
		t.Fatalf("decoding: %v\n%s", err, out)
	}
	if len(tasks) != 1 || tasks[0].Title != "keep me" {
		t.Fatalf("tasks = %+v, want only the one both terms keep", tasks)
	}
}

// The configured default project is a fallback. An expression that names a
// project has answered the question, so the default must not widen the
// listing back out to two projects.
func TestTaskLsFilterProjectBeatsTheConfiguredDefault(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infra")
	c.mustRun("task", "add", "in infra", "-p", "infra")
	c.mustRun("task", "add", "in default", "-p", "default")
	// The context names the database explicitly: a context with no database
	// falls back to the literal default path rather than the XDG one these
	// tasks were written to, which would make this test pass for the wrong
	// reason by listing an empty database.
	c.mustRun("ctx", "add", "home", "--db", filepath.Join(c.data, "tix", "tix.db"),
		"--project", "default", "--use")

	out := c.mustRun("task", "ls", "--filter", "project:infra", "-o", "json").out
	var tasks []core.Task
	if err := json.Unmarshal([]byte(out), &tasks); err != nil {
		t.Fatalf("decoding: %v\n%s", err, out)
	}
	if len(tasks) != 1 || tasks[0].Title != "in infra" {
		t.Fatalf("tasks = %+v, want only the project the filter named", tasks)
	}
}
