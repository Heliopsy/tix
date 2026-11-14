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

func showTenant(t *testing.T, c *cli) core.Tenant {
	t.Helper()
	got := c.mustRun("tenant", "show", "default", "-o", "json")
	var out core.Tenant
	if err := json.Unmarshal([]byte(got.out), &out); err != nil {
		t.Fatalf("not json: %v\n%s", err, got.out)
	}
	return out
}

func TestTenantIconFlags(t *testing.T) {
	c := newCLI(t)
	dir := t.TempDir()
	logo := filepath.Join(dir, "logo.png")
	png := []byte("\x89PNG\r\n\x1a\ncli tenant icon")
	if err := os.WriteFile(logo, png, 0o600); err != nil {
		t.Fatal(err)
	}

	c.mustRun("tenant", "edit", "default", "--icon-name", "server")
	if got := showTenant(t, c); got.IconName != "server" || got.Icon != "" {
		t.Fatalf("after --icon-name: %+v", got)
	}
	glyph, _ := core.BuiltinIconGlyph("server")
	if table := c.mustRun("tenant", "show", "default"); !strings.Contains(table.out, glyph+" server") {
		t.Fatalf("the table does not show the built-in icon's stand-in:\n%s", table.out)
	}

	c.mustRun("tenant", "edit", "default", "--icon-file", logo)
	if got := showTenant(t, c); got.IconName != "" || got.IconImage != core.ProjectIconDigest(png) {
		t.Fatalf("after --icon-file: %+v", got)
	}
	copied := filepath.Join(dir, "copy.png")
	c.mustRun("tenant", "icon", "default", "--out", copied)
	if got, err := os.ReadFile(copied); err != nil || !bytes.Equal(got, png) {
		t.Fatalf("tix tenant icon --out wrote %q (%v)", got, err)
	}
	if got := c.mustRun("tenant", "icon", "default"); got.out != string(png) {
		t.Fatalf("tix tenant icon to a pipe wrote %q", got.out)
	}

	c.mustRun("tenant", "edit", "default", "--name", "Renamed")
	if got := showTenant(t, c); got.IconImage != core.ProjectIconDigest(png) {
		t.Fatalf("an edit naming no icon flag changed the icon: %+v", got)
	}
	c.mustRun("tenant", "edit", "default", "--icon", "")
	if got := showTenant(t, c); got.IconKind() != core.IconKindNone {
		t.Fatalf("an empty --icon did not clear the icon: %+v", got)
	}
	if got := c.run("tenant", "icon", "default"); got.code != core.ExitNotFound {
		t.Fatalf("icon of a tenant without an image exited %d, want %d", got.code, core.ExitNotFound)
	}

	refusals := [][]string{
		{"tenant", "edit", "default", "--icon", "W", "--icon-name", "globe"},
		{"tenant", "edit", "default", "--icon-name", "unicorn"},
		{"tenant", "edit", "default", "--icon-file", filepath.Join(dir, "missing.png")},
	}
	for _, args := range refusals {
		if got := c.run(args...); got.code != core.ExitUsage {
			t.Errorf("tix %s exited %d, want %d\n%s", strings.Join(args, " "), got.code, core.ExitUsage, got.err)
		}
	}
}
