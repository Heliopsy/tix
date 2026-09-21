package tui

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// charDevice opens a writer the colour rules count as a terminal.
func charDevice(t *testing.T) io.Writer {
	t.Helper()
	f, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("opening %s: %v", os.DevNull, err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func TestColorEnabled(t *testing.T) {
	tests := []struct {
		name    string
		environ []string
		tty     bool
		want    bool
	}{
		{"colour by default on a terminal", []string{"TERM=xterm-256color"}, true, true},
		{"NO_COLOR disables", []string{"TERM=xterm-256color", "NO_COLOR=1"}, true, false},
		{"NO_COLOR with any value disables", []string{"NO_COLOR=please"}, true, false},
		{"TIX_NO_COLOR disables", []string{"TIX_NO_COLOR=1"}, true, false},
		{"empty NO_COLOR does not disable", []string{"TERM=xterm", "NO_COLOR="}, true, true},
		{"dumb terminal disables", []string{"TERM=dumb"}, true, false},
		{"no environment at all", nil, true, true},
		{"a pipe is not a terminal", []string{"TERM=xterm-256color"}, false, false},
		{"a pipe stays plain even without NO_COLOR", nil, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out io.Writer = &bytes.Buffer{}
			if tc.tty {
				out = charDevice(t)
			}
			if got := ColorEnabled(tc.environ, out); got != tc.want {
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
		"claimed": theme.Claimed, "dim": theme.Dim, "error": theme.Error,
		"status": theme.Status, "ref": theme.Ref, "blocked": theme.Blocked,
		"empty": theme.Empty, "bar": theme.Bar,
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

func TestColourlessThemeStillStylesNothingByMeaning(t *testing.T) {
	theme := NewTheme(false)
	for _, got := range []string{
		theme.Category(core.CategoryInProgress).Render("doing"),
		theme.Priority(core.PriorityHighest).Render("P1"),
		theme.Project(core.ColorViolet).Render("infra"),
	} {
		if strings.Contains(got, "\x1b") {
			t.Fatalf("a colourless theme emitted %q", got)
		}
	}
}

func TestSelectionMarkerDoesNotRelyOnColor(t *testing.T) {
	if SelectionMarker(true) == SelectionMarker(false) {
		t.Fatal("selection is indistinguishable without colour")
	}
	if lipgloss.Width(SelectionMarker(true)) != lipgloss.Width(SelectionMarker(false)) {
		t.Fatal("selection markers have different widths and would shift the layout")
	}
}

// sgrParams extracts the select graphic rendition parameters of a styled
// string, so two surfaces can be compared on the attributes they actually
// write rather than on how each one spells them.
func sgrParams(s string) []string {
	re := regexp.MustCompile(`\x1b\[([0-9;]*)m`)
	var out []string
	for _, match := range re.FindAllStringSubmatch(s, -1) {
		for _, p := range strings.Split(match[1], ";") {
			if p != "" && p != "0" {
				out = append(out, p)
			}
		}
	}
	return out
}

// renderANSI states the select graphic rendition parameter a terminal writes
// for an ANSI colour, which is how a terminal renders it whatever profile the
// test process happens to have been started under.
func renderANSI(color lipgloss.Color) []string {
	n, err := strconv.Atoi(string(color))
	if err != nil || n < 0 || n > 15 {
		return nil
	}
	if n < 8 {
		return []string{strconv.Itoa(30 + n)}
	}
	return []string{strconv.Itoa(90 + n - 8)}
}

func TestStateCategoryColourMatchesTheCLI(t *testing.T) {
	painter := output.NewPainter(output.ModeAlways, nil)
	tests := []struct {
		category core.StateCategory
		status   string
	}{
		{core.CategoryTodo, "todo"},
		{core.CategoryInProgress, "in_progress"},
		{core.CategoryDone, "done"},
	}
	for _, tc := range tests {
		t.Run(string(tc.category), func(t *testing.T) {
			color, ok := CategoryColor(tc.category)
			if !ok {
				t.Fatalf("category %q has no colour", tc.category)
			}
			want := sgrParams(painter.StatusIn(tc.category, tc.status))
			if got := renderANSI(color); !equalParams(got, want) {
				t.Fatalf("category %q renders %v, the cli renders %v", tc.category, got, want)
			}
		})
	}
	if _, ok := CategoryColor(core.StateCategory("invented")); ok {
		t.Fatal("an unknown category was given a colour, which would be a guess")
	}
}

func TestPriorityColourMatchesTheCLI(t *testing.T) {
	painter := output.NewPainter(output.ModeAlways, nil)
	for _, p := range []core.Priority{
		core.PriorityHighest, core.PriorityHigh, core.PriorityNormal,
		core.PriorityLow, core.PriorityLowest,
	} {
		color, bold, ok := PriorityColor(p)
		want := sgrParams(painter.Priority(p, "x"))
		if !ok {
			if len(want) != 0 {
				t.Fatalf("priority %d is undistinguished here but the cli renders %v", p, want)
			}
			continue
		}
		got := renderANSI(color)
		if bold {
			got = append([]string{"1"}, got...)
		}
		if !equalParams(got, want) {
			t.Fatalf("priority %d renders %v, the cli renders %v", p, got, want)
		}
	}
}

func TestProjectColourCoversThePalette(t *testing.T) {
	for _, c := range core.ProjectColors() {
		if _, ok := ProjectColor(c); !ok {
			t.Fatalf("project colour %q has no terminal colour", c)
		}
	}
	if _, ok := ProjectColor(core.ColorNone); ok {
		t.Fatal("the empty colour was given a style")
	}
}

func TestPriorityLabelNamesEveryPriority(t *testing.T) {
	seen := map[string]bool{}
	for p := core.PriorityHighest; p <= core.PriorityLowest; p++ {
		label := PriorityLabel(p)
		if label == "" || label == "unset" || seen[label] {
			t.Fatalf("priority %d has label %q", p, label)
		}
		seen[label] = true
	}
	if PriorityLabel(core.Priority(0)) != "unset" {
		t.Fatal("an out of range priority was named as though it were valid")
	}
}

func TestProjectGlyphAlwaysRendersSomething(t *testing.T) {
	if ProjectGlyph(core.Project{Icon: "🚀"}) != "🚀" {
		t.Fatal("a project's own icon was dropped")
	}
	if ProjectGlyph(core.Project{}) == "" {
		t.Fatal("a project without an icon rendered nothing, so rows would not line up")
	}
}

// equalParams compares two attribute lists as sets, since order is not part of
// what either surface promises.
func equalParams(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]int{}
	for _, v := range a {
		seen[v]++
	}
	for _, v := range b {
		seen[v]--
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}
