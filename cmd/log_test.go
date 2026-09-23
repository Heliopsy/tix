// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestLogKeysAppearInConfigShow checks the surface an operator actually looks
// at. A key that resolves but is invisible here is one nobody can confirm.
func TestLogKeysAppearInConfigShow(t *testing.T) {
	c := newCLI(t)
	got := c.mustRun("config", "show", "--sources", "-o", "json")

	var entries []struct {
		Key    string `json:"key"`
		Env    string `json:"env"`
		Value  string `json:"value"`
		Source string `json:"source"`
	}
	if err := json.Unmarshal([]byte(got.out), &entries); err != nil {
		t.Fatalf("config show did not emit json: %v\n%s", err, got.out)
	}
	byKey := map[string]string{}
	byEnv := map[string]string{}
	for _, e := range entries {
		byKey[e.Key] = e.Value
		byEnv[e.Key] = e.Env
	}

	cases := []struct {
		key   string
		env   string
		value string
	}{
		{"log.level", "TIX_LOG_LEVEL", "info"},
		{"log.format", "TIX_LOG_FORMAT", "text"},
		{"log.output", "TIX_LOG_OUTPUT", "stderr"},
		{"log.file.max_size_mb", "TIX_LOG_FILE_MAX_SIZE_MB", "100"},
		{"log.file.max_age", "TIX_LOG_FILE_MAX_AGE", "168h0m0s"},
		{"log.file.max_backups", "TIX_LOG_FILE_MAX_BACKUPS", "7"},
		{"log.file.compress", "TIX_LOG_FILE_COMPRESS", "false"},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			value, ok := byKey[tc.key]
			if !ok {
				t.Fatalf("config show does not report %q", tc.key)
			}
			if value != tc.value {
				t.Fatalf("config show reports %s = %q, want %q", tc.key, value, tc.value)
			}
			if env := byEnv[tc.key]; env != tc.env {
				t.Fatalf("config show reports %s as %s, want %s", tc.key, env, tc.env)
			}
		})
	}
}

// TestLogFlagsWinOverEveryOtherLayer covers the flag layer of the logging keys
// end to end, through the same precedence chain every other key uses.
func TestLogFlagsWinOverEveryOtherLayer(t *testing.T) {
	cases := []struct {
		name string
		args []string
		env  []string
		key  string
		want string
		src  string
	}{
		{"flag beats environment", []string{"--log-level", "error"},
			[]string{"TIX_LOG_LEVEL=warn"}, "log.level", "error", "flag"},
		{"environment applies with no flag", nil,
			[]string{"TIX_LOG_FORMAT=json"}, "log.format", "json", "environment"},
		{"a flag nobody typed is not a layer", []string{"--log-level", "error"},
			[]string{"TIX_LOG_FILE_MAX_BACKUPS=3"}, "log.file.max_backups", "3", "environment"},
		{"duration flag", []string{"--log-max-age", "48h"}, nil,
			"log.file.max_age", "48h0m0s", "flag"},
		{"bool flag", []string{"--log-compress"}, nil, "log.file.compress", "true", "flag"},
		{"int flag", []string{"--log-max-size-mb", "5"}, nil, "log.file.max_size_mb", "5", "flag"},
		{"default when nothing is set", nil, nil, "log.output", "stderr", "default"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newCLI(t)
			var out, errb bytes.Buffer
			args := append(append([]string{}, tc.args...), "config", "show", "--sources", "-o", "json")
			if code := Run(args, strings.NewReader(""), &out, &errb,
				append(c.environ(), tc.env...), c.home); code != 0 {
				t.Fatalf("config show exited %d: %s", code, errb.String())
			}
			var entries []struct {
				Key    string `json:"key"`
				Value  string `json:"value"`
				Source string `json:"source"`
			}
			if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
				t.Fatalf("config show did not emit json: %v", err)
			}
			for _, e := range entries {
				if e.Key != tc.key {
					continue
				}
				if e.Value != tc.want || e.Source != tc.src {
					t.Fatalf("%s = %q from %q, want %q from %q", tc.key, e.Value, e.Source, tc.want, tc.src)
				}
				return
			}
			t.Fatalf("config show does not report %q", tc.key)
		})
	}
}

// TestLoggerWritesWhereConfigured drives the logger the commands build, rather
// than the package under it, so the wiring is covered and not only the writer.
func TestLoggerWritesWhereConfigured(t *testing.T) {
	c := newCLI(t)
	path := filepath.Join(t.TempDir(), "logs", "tix.log")
	cases := []struct {
		name  string
		env   []string
		check func(t *testing.T, errw *bytes.Buffer)
	}{
		{"stderr by default", nil, func(t *testing.T, errw *bytes.Buffer) {
			if !strings.Contains(errw.String(), "a-test-record") {
				t.Fatalf("stderr holds %q, want the record", errw.String())
			}
		}},
		{"a file destination", []string{"TIX_LOG_OUTPUT=" + path, "TIX_LOG_FORMAT=json"},
			func(t *testing.T, errw *bytes.Buffer) {
				if strings.Contains(errw.String(), "a-test-record") {
					t.Fatalf("stderr holds %q, but the destination is a file", errw.String())
				}
				body, err := os.ReadFile(path) // #nosec G304 -- a path this test created
				if err != nil {
					t.Fatalf("reading %s: %v", path, err)
				}
				if !strings.Contains(string(body), "a-test-record") {
					t.Fatalf("%s holds %q, want the record", path, body)
				}
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errw bytes.Buffer
			g := &globals{environ: append(c.environ(), tc.env...), dir: c.home}
			root := &cobra.Command{Use: "tix"}
			registerLogFlags(root, &g.log)
			root.SetOut(&out)
			root.SetErr(&errw)

			log, err := g.logger(root)
			if err != nil {
				t.Fatalf("building the logger: %v", err)
			}
			log.Info("a-test-record")
			if g.logCloser != nil {
				if err := g.logCloser.Close(); err != nil {
					t.Fatalf("closing the log destination: %v", err)
				}
			}
			tc.check(t, &errw)
		})
	}
}

// TestLoggerRefusesAnUnopenableDestination proves a file destination fails at
// startup rather than at the first record, which is the failure nobody sees.
func TestLoggerRefusesAnUnopenableDestination(t *testing.T) {
	c := newCLI(t)
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("writing %s: %v", blocker, err)
	}
	g := &globals{
		environ: append(c.environ(), "TIX_LOG_OUTPUT="+filepath.Join(blocker, "tix.log")),
		dir:     c.home,
	}
	root := &cobra.Command{Use: "tix"}
	registerLogFlags(root, &g.log)
	if _, err := g.logger(root); err == nil {
		t.Fatal("a log destination under a regular file was accepted, want a refusal at startup")
	}
}
