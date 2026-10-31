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

func showProject(t *testing.T, c *cli, key string) core.Project {
	t.Helper()
	got := c.mustRun("project", "show", key, "-o", "json")
	var p core.Project
	if err := json.Unmarshal([]byte(got.out), &p); err != nil {
		t.Fatalf("not json: %v\n%s", err, got.out)
	}
	return p
}

func TestProjectIconFlags(t *testing.T) {
	c := newCLI(t)
	dir := t.TempDir()
	logo := filepath.Join(dir, "logo.png")
	png := []byte("\x89PNG\r\n\x1a\ncli icon")
	if err := os.WriteFile(logo, png, 0o600); err != nil {
		t.Fatal(err)
	}

	c.mustRun("project", "create", "infra", "Infrastructure", "--icon-name", "server")
	if p := showProject(t, c, "infra"); p.IconName != "server" || p.Icon != "" {
		t.Fatalf("created with icon %q / %q", p.Icon, p.IconName)
	}
	table := c.mustRun("project", "ls")
	glyph, _ := core.BuiltinIconGlyph("server")
	if !strings.Contains(table.out, glyph+" server") {
		t.Fatalf("the listing does not show the built-in icon's stand-in:\n%s", table.out)
	}

	c.mustRun("project", "edit", "infra", "--icon-file", logo)
	p := showProject(t, c, "infra")
	if p.IconName != "" || p.IconImage != core.ProjectIconDigest(png) {
		t.Fatalf("after --icon-file: %+v", p)
	}
	if table := c.mustRun("project", "ls"); !strings.Contains(table.out, core.ProjectImageGlyph+" image") {
		t.Fatalf("the listing does not mark the uploaded icon:\n%s", table.out)
	}

	copied := filepath.Join(dir, "copy.png")
	c.mustRun("project", "icon", "infra", "--out", copied)
	if got, err := os.ReadFile(copied); err != nil || !bytes.Equal(got, png) {
		t.Fatalf("tix project icon --out wrote %q (%v)", got, err)
	}
	if got := c.mustRun("project", "icon", "infra"); got.out != string(png) {
		t.Fatalf("tix project icon to a pipe wrote %q", got.out)
	}

	c.mustRun("project", "edit", "infra", "--icon-file", "")
	if p := showProject(t, c, "infra"); p.IconKind() != core.IconKindNone {
		t.Fatalf("an empty --icon-file did not clear the icon: %+v", p)
	}
	if got := c.run("project", "icon", "infra"); got.code != core.ExitNotFound {
		t.Fatalf("icon of a project without an image exited %d, want %d", got.code, core.ExitNotFound)
	}

	c.mustRun("project", "edit", "infra", "--icon-name", "rocket")
	if p := showProject(t, c, "infra"); p.IconName != "rocket" {
		t.Fatalf("edit --icon-name did not set the icon: %+v", p)
	}

	refusals := [][]string{
		{"project", "create", "web", "Web", "--icon", "W", "--icon-name", "globe"},
		{"project", "create", "web", "Web", "--icon-name", "unicorn"},
		{"project", "edit", "infra", "--icon", "W", "--icon-file", logo},
		{"project", "edit", "infra", "--icon-file", filepath.Join(dir, "missing.png")},
	}
	for _, args := range refusals {
		if got := c.run(args...); got.code != core.ExitUsage {
			t.Errorf("tix %s exited %d, want %d\n%s", strings.Join(args, " "), got.code, core.ExitUsage, got.err)
		}
	}
	if got := c.run("project", "show", "web"); got.code != core.ExitNotFound {
		t.Fatal("a refused create wrote the project")
	}
}
