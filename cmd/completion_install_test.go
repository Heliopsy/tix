// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// completionEnv is an environment confined to a throwaway home. Every variable
// the command reads is set, because an unset XDG_DATA_HOME would send the
// install into the real ~/.local/share of whoever runs the suite.
type completionEnv struct {
	home  string
	data  string
	conf  string
	shell string
}

func newCompletionEnv(t *testing.T, shell string) completionEnv {
	t.Helper()
	home := t.TempDir()
	return completionEnv{
		home:  home,
		data:  filepath.Join(home, "data"),
		conf:  filepath.Join(home, "conf"),
		shell: shell,
	}
}

func (e completionEnv) environ() []string {
	return []string{
		"HOME=" + e.home,
		"XDG_DATA_HOME=" + e.data,
		"XDG_CONFIG_HOME=" + e.conf,
		"SHELL=" + e.shell,
		"NO_COLOR=1",
	}
}

func (e completionEnv) run(args ...string) result {
	var out, errb bytes.Buffer
	code := Run(args, strings.NewReader(""), &out, &errb, e.environ(), e.home)
	return result{code: code, out: out.String(), err: errb.String()}
}

// bashPath and friends spell out the layout the spec fixes, rather than reusing
// completionLayouts, so a change to the table is a test failure and not a test
// that quietly follows it.
func (e completionEnv) bashPath() string {
	return filepath.Join(e.data, "bash-completion", "completions", "tix")
}

func (e completionEnv) zshPath() string {
	return filepath.Join(e.data, "zsh", "site-functions", "_tix")
}

func (e completionEnv) fishPath() string {
	return filepath.Join(e.conf, "fish", "completions", "tix.fish")
}

func TestCompletionInstallWritesWhereTheShellLooks(t *testing.T) {
	for _, tc := range []struct {
		name  string
		shell string
		args  []string
		path  func(completionEnv) string
		note  string
	}{
		{
			name:  "bash from the environment",
			shell: "/bin/bash",
			args:  []string{"completion", "install"},
			path:  completionEnv.bashPath,
			note:  "bash-completion",
		},
		{
			name:  "zsh from the environment",
			shell: "/usr/bin/zsh",
			args:  []string{"completion", "install"},
			path:  completionEnv.zshPath,
			note:  "fpath",
		},
		{
			name:  "fish from the environment",
			shell: "/usr/local/bin/fish",
			args:  []string{"completion", "install"},
			path:  completionEnv.fishPath,
		},
		{
			name:  "an explicit shell overrides the environment",
			shell: "/bin/bash",
			args:  []string{"completion", "install", "--shell", "fish"},
			path:  completionEnv.fishPath,
		},
		{
			name:  "an explicit shell is enough with no SHELL at all",
			shell: "",
			args:  []string{"completion", "install", "--shell", "zsh"},
			path:  completionEnv.zshPath,
			note:  "fpath",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newCompletionEnv(t, tc.shell)
			got := env.run(tc.args...)
			if got.code != core.ExitOK {
				t.Fatalf("exit = %d, want 0\nstdout: %s\nstderr: %s", got.code, got.out, got.err)
			}
			want := tc.path(env)
			if !strings.Contains(got.out, want) {
				t.Errorf("stdout = %q, want the path %q", got.out, want)
			}
			body, err := os.ReadFile(want) // #nosec G304 -- a path under the test's own temp home
			if err != nil {
				t.Fatalf("reading the installed script: %v", err)
			}
			if len(body) == 0 {
				t.Error("the installed script is empty")
			}
			if tc.note == "" {
				if lines := strings.Count(strings.TrimSpace(got.out), "\n"); lines != 0 {
					t.Errorf("stdout = %q, want only the path for a shell that needs nothing further", got.out)
				}
			} else if !strings.Contains(got.out, tc.note) {
				t.Errorf("stdout = %q, want it to mention %q", got.out, tc.note)
			}
		})
	}
}

func TestCompletionInstallNeverEscapesTheUsersDirectories(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		t.Run(shell, func(t *testing.T) {
			env := newCompletionEnv(t, "/bin/"+shell)
			got := env.run("completion", "install")
			if got.code != core.ExitOK {
				t.Fatalf("exit = %d\nstderr: %s", got.code, got.err)
			}
			path := strings.TrimSpace(strings.TrimPrefix(strings.Split(got.out, "\n")[0], "wrote "))
			if !strings.HasPrefix(path, env.home+string(filepath.Separator)) {
				t.Errorf("installed to %q, which is outside the home %q", path, env.home)
			}
			for _, rc := range []string{".bashrc", ".zshrc", ".profile"} {
				if _, err := os.Stat(filepath.Join(env.home, rc)); err == nil {
					t.Errorf("%s was created; install must not touch startup files", rc)
				}
			}
		})
	}
}

func TestCompletionInstallDryRunWritesNothing(t *testing.T) {
	env := newCompletionEnv(t, "/bin/bash")
	got := env.run("completion", "install", "--dry-run")
	if got.code != core.ExitOK {
		t.Fatalf("exit = %d\nstderr: %s", got.code, got.err)
	}
	if !strings.Contains(got.out, env.bashPath()) {
		t.Errorf("stdout = %q, want the path that would be written", got.out)
	}
	if _, err := os.Stat(env.bashPath()); err == nil {
		t.Fatal("--dry-run created the file")
	}
	if _, err := os.Stat(filepath.Dir(env.bashPath())); err == nil {
		t.Error("--dry-run created the parent directory")
	}
}

func TestCompletionInstallTwiceLeavesTheSameFile(t *testing.T) {
	env := newCompletionEnv(t, "/bin/zsh")
	if got := env.run("completion", "install"); got.code != core.ExitOK {
		t.Fatalf("first install exited %d\nstderr: %s", got.code, got.err)
	}
	first, err := os.ReadFile(env.zshPath()) // #nosec G304 -- a path under the test's own temp home
	if err != nil {
		t.Fatalf("reading the first install: %v", err)
	}
	if got := env.run("completion", "install"); got.code != core.ExitOK {
		t.Fatalf("second install exited %d\nstderr: %s", got.code, got.err)
	}
	second, err := os.ReadFile(env.zshPath()) // #nosec G304 -- a path under the test's own temp home
	if err != nil {
		t.Fatalf("reading the second install: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("the second install produced %d bytes, the first %d", len(second), len(first))
	}
}

func TestCompletionInstallUninstall(t *testing.T) {
	for _, tc := range []struct {
		name    string
		install bool
		want    string
	}{
		{name: "removes what was written", install: true, want: "removed"},
		{name: "reports an absent file", install: false, want: "nothing to remove"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newCompletionEnv(t, "/bin/fish")
			if tc.install {
				if got := env.run("completion", "install"); got.code != core.ExitOK {
					t.Fatalf("install exited %d\nstderr: %s", got.code, got.err)
				}
			}
			got := env.run("completion", "install", "--uninstall")
			if got.code != core.ExitOK {
				t.Fatalf("exit = %d, want 0\nstderr: %s", got.code, got.err)
			}
			if !strings.Contains(got.out, tc.want) || !strings.Contains(got.out, env.fishPath()) {
				t.Errorf("stdout = %q, want %q and the path", got.out, tc.want)
			}
			if _, err := os.Stat(env.fishPath()); err == nil {
				t.Error("the file is still there after --uninstall")
			}
		})
	}
}

func TestCompletionInstallRefusesAnUnsupportedShell(t *testing.T) {
	for _, tc := range []struct {
		name  string
		shell string
		args  []string
	}{
		{name: "from the environment", shell: "/bin/ksh", args: []string{"completion", "install"}},
		{name: "from the flag", shell: "/bin/bash", args: []string{"completion", "install", "--shell", "ksh"}},
		{name: "no shell anywhere", shell: "", args: []string{"completion", "install"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newCompletionEnv(t, tc.shell)
			got := env.run(tc.args...)
			if got.code != core.ExitUsage {
				t.Fatalf("exit = %d, want %d\nstdout: %s\nstderr: %s",
					got.code, core.ExitUsage, got.out, got.err)
			}
			for _, shell := range []string{"bash", "zsh", "fish"} {
				if !strings.Contains(got.err, shell) {
					t.Errorf("stderr = %q, want it to name %q", got.err, shell)
				}
			}
		})
	}
}

func TestCompletionStillGeneratesToStdout(t *testing.T) {
	env := newCompletionEnv(t, "/bin/bash")
	got := env.run("completion", "bash")
	if got.code != core.ExitOK {
		t.Fatalf("exit = %d\nstderr: %s", got.code, got.err)
	}
	if !strings.Contains(got.out, "tix") {
		t.Errorf("stdout = %q, want a completion script", got.out)
	}
	if _, err := os.Stat(env.bashPath()); err == nil {
		t.Error("completion bash wrote a file; it only generates")
	}
}
