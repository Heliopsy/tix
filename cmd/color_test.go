package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

const escape = "\x1b["

func TestColourFlagsAndEnvironment(t *testing.T) {
	cases := []struct {
		name    string
		env     []string
		args    []string
		want    bool
		wantErr bool
	}{
		{name: "a pipe is plain by default", args: []string{"task", "ls"}},
		{
			name: "--color forces colour onto a pipe",
			args: []string{"--color", "task", "ls"}, want: true,
		},
		{
			name: "the colour key forces colour onto a pipe",
			env:  []string{"TIX_OUTPUT_COLOR=always"},
			args: []string{"task", "ls"}, want: true,
		},
		{
			name: "--no-color beats the colour key",
			env:  []string{"TIX_OUTPUT_COLOR=always"},
			args: []string{"--no-color", "task", "ls"},
		},
		{
			name: "NO_COLOR disables it",
			env:  []string{"NO_COLOR=1"},
			args: []string{"task", "ls"},
		},
		{
			name: "TIX_NO_COLOR disables it",
			env:  []string{"NO_COLOR=", "TIX_NO_COLOR=1"},
			args: []string{"task", "ls"},
		},
		{
			name: "--color and --no-color together are a usage error",
			args: []string{"--color", "--no-color", "task", "ls"}, wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newCLI(t)
			c.mustRun("task", "add", "colour me")

			got := runEnv(c, tc.env, tc.args...)
			if tc.wantErr {
				if got.code != core.ExitUsage {
					t.Fatalf("exit = %d, want %d\n%s", got.code, core.ExitUsage, got.err)
				}
				return
			}
			if got.code != core.ExitOK {
				t.Fatalf("exit = %d: %s", got.code, got.err)
			}
			if has := strings.Contains(got.out, escape); has != tc.want {
				t.Fatalf("escape codes present = %v, want %v\n%q", has, tc.want, got.out)
			}
			if !strings.Contains(got.out, "colour me") {
				t.Fatalf("the task is missing from the listing: %q", got.out)
			}
		})
	}
}

func TestForcedColourNeverReachesMachineFormats(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "colour me")

	for _, format := range []string{"json", "yaml", "ndjson"} {
		got := runEnv(c, []string{"TIX_OUTPUT_COLOR=always"}, "--color", "task", "ls", "-o", format)
		if got.code != core.ExitOK {
			t.Fatalf("%s: exit %d: %s", format, got.code, got.err)
		}
		if strings.Contains(got.out, escape) {
			t.Errorf("%s output carries escape codes: %q", format, got.out)
		}
	}

	got := runEnv(c, nil, "--color", "task", "ls", "-o", "json")
	var tasks []core.Task
	if err := json.Unmarshal([]byte(got.out), &tasks); err != nil {
		t.Fatalf("forced-colour json does not parse: %v\n%s", err, got.out)
	}
}

func TestErrorsAreColouredOnStderr(t *testing.T) {
	c := newCLI(t)

	plain := runEnv(c, nil, "task", "show", "NOPE-1")
	if plain.code == core.ExitOK {
		t.Fatal("expected a failure")
	}
	if strings.Contains(plain.err, escape) {
		t.Errorf("a piped error carries escape codes: %q", plain.err)
	}

	coloured := runEnv(c, nil, "--color", "task", "show", "NOPE-1")
	if !strings.Contains(coloured.err, escape) {
		t.Errorf("--color did not colour the error: %q", coloured.err)
	}
	if !strings.Contains(coloured.err, "error:") {
		t.Errorf("the error label is missing: %q", coloured.err)
	}
}
