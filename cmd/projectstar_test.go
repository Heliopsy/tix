// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// listedProjects runs project ls as JSON and returns the keys with their
// starred marks.
func listedProjects(t *testing.T, c *cli, args ...string) map[string]bool {
	t.Helper()
	got := c.mustRun(append([]string{"project", "ls", "-o", "json"}, args...)...)
	var projects []core.Project
	if err := json.Unmarshal([]byte(got.out), &projects); err != nil {
		t.Fatalf("not json: %v\n%s", err, got.out)
	}
	out := make(map[string]bool, len(projects))
	for _, p := range projects {
		out[p.Key] = p.Starred
	}
	return out
}

func TestProjectStarMarksAndFiltersTheListing(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")
	c.mustRun("project", "create", "web", "Website")

	c.mustRun("project", "star", "web", "--dry-run")
	if got := listedProjects(t, c, "--starred"); len(got) != 0 {
		t.Fatalf("a dry run starred %v", got)
	}

	c.mustRun("project", "star", "web")
	c.mustRun("project", "star", "web")
	if got := listedProjects(t, c, "--starred"); len(got) != 1 || !got["web"] {
		t.Fatalf("--starred listed %v, want web alone", got)
	}
	if got := listedProjects(t, c); !got["web"] || got["infra"] {
		t.Fatalf("ls marks %v, want web alone starred", got)
	}
	table := c.mustRun("project", "ls").out
	if !strings.Contains(table, "* WEB") || strings.Contains(table, "* INFRA") {
		t.Fatalf("the table marks the wrong rows:\n%s", table)
	}
	if shown := decodeProject(t, c.mustRun("project", "show", "web", "-o", "json").out); !shown.Starred {
		t.Fatal("project show does not carry the star")
	}

	c.mustRun("project", "unstar", "web")
	c.mustRun("project", "unstar", "web")
	if got := listedProjects(t, c, "--starred"); len(got) != 0 {
		t.Fatalf("after unstar --starred listed %v", got)
	}
	if got := c.run("project", "star", "missing"); got.code != core.ExitNotFound {
		t.Fatalf("star of an unknown project exited %d, want %d", got.code, core.ExitNotFound)
	}
}
