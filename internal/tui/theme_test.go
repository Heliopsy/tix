package tui

import (
	"strings"
	"testing"
)

func TestColorEnabled(t *testing.T) {
	tests := []struct {
		name    string
		environ []string
		want    bool
	}{
		{"colour by default", []string{"TERM=xterm-256color"}, true},
		{"NO_COLOR disables", []string{"TERM=xterm-256color", "NO_COLOR=1"}, false},
		{"NO_COLOR with any value disables", []string{"NO_COLOR=please"}, false},
		{"empty NO_COLOR does not disable", []string{"TERM=xterm", "NO_COLOR="}, true},
		{"dumb terminal disables", []string{"TERM=dumb"}, false},
		{"no environment at all", nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ColorEnabled(tc.environ); got != tc.want {
				t.Fatalf("ColorEnabled = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestThemeWithoutColorEmitsNoEscapes(t *testing.T) {
	theme := NewTheme(false)
	if theme.Color {
		t.Fatal("a colourless theme reports colour")
	}
	for name, style := range map[string]interface{ Render(...string) string }{
		"title": theme.Title, "header": theme.Header, "selected": theme.Selected,
		"claimed": theme.Claimed, "dim": theme.Dim, "error": theme.Error, "status": theme.Status,
	} {
		got := style.Render("plain text")
		if strings.Contains(got, "\x1b") {
			t.Fatalf("%s style emitted an escape sequence: %q", name, got)
		}
		if got != "plain text" {
			t.Fatalf("%s style altered its input: %q", name, got)
		}
	}
}

func TestSelectionMarkerDoesNotRelyOnColor(t *testing.T) {
	if SelectionMarker(true) == SelectionMarker(false) {
		t.Fatal("selection is indistinguishable without colour")
	}
	if len(SelectionMarker(true)) != len(SelectionMarker(false)) {
		t.Fatal("selection markers have different widths and would shift the layout")
	}
}
