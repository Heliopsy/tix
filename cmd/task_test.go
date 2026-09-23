// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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
