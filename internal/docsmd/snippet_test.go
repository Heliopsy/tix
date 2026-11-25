// SPDX-License-Identifier: AGPL-3.0-or-later

package docsmd_test

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/docsmd"
)

func TestRegionsDedentsAndExpandsTabs(t *testing.T) {
	src := "package main\n\nfunc f() {\n\t// region body\n\tif x {\n\t\ty()\n\t}\n\n\t// endregion\n}\n# region top\nz = 1\n# endregion\n"
	bodies, order, err := docsmd.Regions(src)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(order, ","), "body,top"; got != want {
		t.Errorf("order = %q, want %q", got, want)
	}
	if got, want := bodies["body"], "if x {\n    y()\n}"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	if got, want := bodies["top"], "z = 1"; got != want {
		t.Errorf("top = %q, want %q", got, want)
	}
}

func TestRegionsRefusesBrokenMarkers(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{"unclosed", "// region a\nx\n", "never closed"},
		{"nested", "// region a\n// region b\n// endregion\n", "inside region"},
		{"stray end", "x\n// endregion\n", "no region open"},
		{"twice", "# region a\n# endregion\n# region a\n# endregion\n", "marked twice"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := docsmd.Regions(tt.src)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want one containing %q", err, tt.want)
			}
		})
	}
}

func TestSnippetsReadsTheFenceUnderEachMarker(t *testing.T) {
	md := "intro\n\n<!-- example: a/b.py#step -->\n\n```python\nx = 1\ny = 2\n```\n\n```go\nunmarked()\n```\n"
	got, err := docsmd.Snippets(md)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d snippets, want the one marked: %+v", len(got), got)
	}
	s := got[0]
	if s.Path != "a/b.py" || s.Region != "step" || s.Lang != "python" || s.Body != "x = 1\ny = 2" || s.Line != 3 {
		t.Errorf("snippet = %+v", s)
	}
}

func TestSnippetsRefusesAMarkerWithoutAFence(t *testing.T) {
	tests := []struct {
		name, md, want string
	}{
		{"no fence", "<!-- example: a.py#x -->\n\nprose\n", "not followed by a fence"},
		{"no region", "<!-- example: a.py -->\n```python\n```\n", "is not <!-- example"},
		{"unclosed", "<!-- example: a.py#x -->\n```python\nx\n", "never closed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := docsmd.Snippets(tt.md)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want one containing %q", err, tt.want)
			}
		})
	}
}
