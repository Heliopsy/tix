// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestIconsTheTerminalCannotDrawGetAStandIn(t *testing.T) {
	rocket, _ := core.BuiltinIconGlyph("rocket")
	cases := []struct {
		p     core.Project
		glyph string
		line  string
	}{
		{core.Project{Key: "a", Icon: "W"}, "W", "W"},
		{core.Project{Key: "b", IconName: "rocket"}, rocket, "built-in rocket"},
		{core.Project{Key: "c", IconImage: "abc"}, core.ProjectImageGlyph, "uploaded image"},
	}
	for _, tc := range cases {
		if got := ProjectGlyph(tc.p); got != tc.glyph {
			t.Errorf("%s: glyph %q, want %q", tc.p.Key, got, tc.glyph)
		}
		if row := ProjectIcon(tc.p); !strings.HasPrefix(row, tc.glyph) {
			t.Errorf("%s: list row starts %q, want the glyph %q", tc.p.Key, row, tc.glyph)
		}
		frame := renderLines(ProjectView(ProjectState{Project: tc.p}))
		if got := setupRow(t, frame, "project:", "icon"); !strings.Contains(got, tc.glyph) || !strings.Contains(got, tc.line) {
			t.Errorf("%s: icon line %q, want %q and %q", tc.p.Key, got, tc.glyph, tc.line)
		}
	}
}
