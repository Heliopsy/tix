package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/thereisnotime/tix/internal/core"
)

// decodeProject reads one project from a command's JSON output.
func decodeProject(t *testing.T, out string) core.Project {
	t.Helper()
	var p core.Project
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		t.Fatalf("decoding project from %q: %v", out, err)
	}
	return p
}

func TestProjectColourAndIconRoundTripThroughTheCLI(t *testing.T) {
	c := newCLI(t)

	created := decodeProject(t, c.mustRun("project", "create", "infra", "Infrastructure",
		"--color", "blue", "--icon", "\U0001F680", "-o", "json").out)
	if created.Color != core.ColorBlue || created.Icon != "\U0001F680" {
		t.Fatalf("created = %q/%q, want blue/rocket", created.Color, created.Icon)
	}

	edited := decodeProject(t, c.mustRun("project", "edit", "infra",
		"--color", "pink", "--icon", "IN", "-o", "json").out)
	if edited.Color != core.ColorPink || edited.Icon != "IN" {
		t.Fatalf("edited = %q/%q, want pink/IN", edited.Color, edited.Icon)
	}

	shown := decodeProject(t, c.mustRun("project", "show", "infra", "-o", "json").out)
	if shown.Color != core.ColorPink || shown.Icon != "IN" {
		t.Fatalf("shown = %q/%q, want pink/IN", shown.Color, shown.Icon)
	}

	cleared := decodeProject(t, c.mustRun("project", "edit", "infra",
		"--color", "", "--icon", "", "-o", "json").out)
	if cleared.Color != core.ColorNone || cleared.Icon != "" {
		t.Fatalf("cleared = %q/%q, want neither", cleared.Color, cleared.Icon)
	}
	if cleared.Name != "Infrastructure" {
		t.Errorf("clearing changed the name to %q", cleared.Name)
	}

	listed := c.mustRun("project", "ls", "-o", "json").out
	if !strings.Contains(listed, `"key":"infra"`) {
		t.Errorf("project ls did not list the project: %s", listed)
	}
}

func TestProjectAppearanceRejections(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")

	tests := []struct {
		name string
		args []string
	}{
		{"unknown colour on create", []string{"project", "create", "ops", "Ops", "--color", "chartreuse"}},
		{"hex colour on create", []string{"project", "create", "ops", "Ops", "--color", "#ff00ff"}},
		{"long icon on create", []string{"project", "create", "ops", "Ops", "--icon", "infra"}},
		{"unknown colour on edit", []string{"project", "edit", "infra", "--color", "chartreuse"}},
		{"long icon on edit", []string{"project", "edit", "infra", "--icon", "abc"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := c.run(tc.args...)
			if got.code != core.ExitUsage {
				t.Fatalf("exit = %d, want %d\nstdout: %s\nstderr: %s",
					got.code, core.ExitUsage, got.out, got.err)
			}
		})
	}

	shown := decodeProject(t, c.mustRun("project", "show", "infra", "-o", "json").out)
	if shown.Color != core.ColorNone || shown.Icon != "" {
		t.Fatalf("a refused change was persisted: %q/%q", shown.Color, shown.Icon)
	}
}

func TestProjectHelpNamesThePalette(t *testing.T) {
	c := newCLI(t)
	help := c.mustRun("project", "create", "--help").out
	for _, want := range []string{"--color", "--icon", "blue", "violet"} {
		if !strings.Contains(help, want) {
			t.Errorf("project create help does not mention %q", want)
		}
	}
}
