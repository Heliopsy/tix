// SPDX-License-Identifier: AGPL-3.0-or-later

package output

import (
	"bytes"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/testenv"
)

var escapes = regexp.MustCompile("\x1b\\[[0-9;]*m")

func strip(s string) string { return escapes.ReplaceAllString(s, "") }

// charDevice opens a character device, which is what the auto mode looks for.
func charDevice(t *testing.T) *os.File {
	t.Helper()
	f, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		testenv.Skip(t, testenv.Capability{
			Name: "character-device",
			Why:  "the null device cannot be opened here: " + err.Error(),
			How:  "run the suite on a host that exposes " + os.DevNull,
		})
	}
	t.Cleanup(func() { _ = f.Close() })
	// The gate reads the file mode itself rather than calling isTerminal.
	// Gating on the function under test turns every failure of it into a
	// skip, which reads exactly like a pass.
	info, statErr := f.Stat()
	if statErr != nil || info.Mode()&os.ModeCharDevice == 0 {
		testenv.Skip(t, testenv.Capability{
			Name: "character-device",
			Why:  "the null device is not a character device here, so auto colour cannot be exercised",
			How:  "run the suite on a host where " + os.DevNull + " is a character device",
		})
	}
	return f
}

// TestIsTerminalRecognisesACharacterDevice guards automatic colour detection
// at its source. isTerminal answering false for a real character device turns
// auto mode off everywhere without any other symptom, and the writers that
// are not terminals answer false for their own reasons, so only the positive
// case pins it.
func TestIsTerminalRecognisesACharacterDevice(t *testing.T) {
	device := charDevice(t)
	if !isTerminal(device) {
		t.Fatal("isTerminal said no to a character device: auto colour is off everywhere")
	}
	t.Setenv(EnvNoColor, "")
	t.Setenv(EnvTixNoColor, "")
	if !colorAllowed(ModeAuto, device) {
		t.Fatal("auto mode refused colour on a character device")
	}

	regular, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = regular.Close() })
	if isTerminal(regular) {
		t.Error("isTerminal said yes to a regular file")
	}
	if isTerminal(new(bytes.Buffer)) {
		t.Error("isTerminal said yes to a writer that is not a file at all")
	}
}

func colourfulTasks() []core.Task {
	base := sampleTask()
	states := []string{"todo", "doing", "done", "bespoke_state"}
	priorities := []core.Priority{
		core.PriorityHighest, core.PriorityHigh, core.PriorityNormal, core.PriorityLow,
	}
	out := make([]core.Task, 0, len(states))
	for i, state := range states {
		task := base
		task.Ref = "ENG-" + string(rune('1'+i))
		task.Status = state
		task.Priority = priorities[i]
		task.Blocked = i%2 == 0
		out = append(out, task)
	}
	return out
}

func TestMachineFormatsNeverColour(t *testing.T) {
	values := map[string]any{
		"tasks":    colourfulTasks(),
		"task":     sampleTask(),
		"taskpage": core.TaskPage{Tasks: colourfulTasks(), NextCursor: "abc"},
		"workflow": sampleWorkflow(),
	}
	for name, value := range values {
		for _, format := range []string{FormatJSON, FormatYAML, FormatNDJSON} {
			for _, mode := range []Mode{ModeAuto, ModeAlways, ModeNever} {
				var buf bytes.Buffer
				if err := NewWithMode(format, mode).Format(&buf, value); err != nil {
					t.Fatalf("%s/%s: %v", name, format, err)
				}
				if strings.Contains(buf.String(), "\x1b[") {
					t.Errorf("%s/%s/mode %d: escape codes in machine-readable output", name, format, mode)
				}
				var streamed bytes.Buffer
				s := NewStreamWithMode(format, &streamed, mode)
				for _, task := range colourfulTasks() {
					if err := s.Write(task); err != nil {
						t.Fatalf("stream write: %v", err)
					}
				}
				if err := s.Close(); err != nil {
					t.Fatalf("stream close: %v", err)
				}
				if strings.Contains(streamed.String(), "\x1b[") {
					t.Errorf("%s/mode %d: escape codes in streamed %s", name, mode, format)
				}
			}
		}
	}
}

func TestNonTTYWriterGetsNoColour(t *testing.T) {
	t.Setenv(EnvNoColor, "")
	t.Setenv(EnvTixNoColor, "")

	var buf bytes.Buffer
	if err := NewWithMode(FormatTable, ModeAuto).Format(&buf, colourfulTasks()); err != nil {
		t.Fatalf("format: %v", err)
	}
	if strings.Contains(buf.String(), "\x1b[") {
		t.Error("a buffer got escape codes in auto mode")
	}

	file, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	if NewPainter(ModeAuto, file).Enabled() {
		t.Error("a regular file got colour in auto mode")
	}
	if NewPainter(ModeAuto, &buf).Enabled() {
		t.Error("a non-file writer got colour in auto mode")
	}
}

func TestColourModePrecedenceForOneWriter(t *testing.T) {
	device := charDevice(t)
	var buf bytes.Buffer
	cases := []struct {
		name    string
		mode    Mode
		noColor string
		tix     string
		writer  interface{ Write([]byte) (int, error) }
		want    bool
	}{
		{"auto on a terminal", ModeAuto, "", "", device, true},
		{"auto off a terminal", ModeAuto, "", "", &buf, false},
		{"NO_COLOR beats a terminal", ModeAuto, "1", "", device, false},
		{"TIX_NO_COLOR beats a terminal", ModeAuto, "", "1", device, false},
		{"empty NO_COLOR is not set", ModeAuto, "", "", device, true},
		{"never on a terminal", ModeNever, "", "", device, false},
		{"always off a terminal", ModeAlways, "", "", &buf, true},
		{"always beats NO_COLOR", ModeAlways, "1", "1", &buf, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvNoColor, tc.noColor)
			t.Setenv(EnvTixNoColor, tc.tix)
			if got := NewPainter(tc.mode, tc.writer).Enabled(); got != tc.want {
				t.Fatalf("Enabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestColouredTableKeepsPlainColumnWidths(t *testing.T) {
	values := []any{
		colourfulTasks(),
		core.TaskPage{Tasks: colourfulTasks(), NextCursor: "cur123"},
		[]core.Workflow{sampleWorkflow()},
		core.Claim{Task: ptrTask(sampleTask()), LeaseExpiresAt: refTime2, LeaseToken: "lease_1"},
		[]core.Task{},
	}
	for i, value := range values {
		var plain, coloured bytes.Buffer
		if err := NewWithMode(FormatTable, ModeNever).Format(&plain, value); err != nil {
			t.Fatalf("plain %d: %v", i, err)
		}
		if err := NewWithMode(FormatTable, ModeAlways).Format(&coloured, value); err != nil {
			t.Fatalf("coloured %d: %v", i, err)
		}
		if !strings.Contains(coloured.String(), "\x1b[") {
			t.Fatalf("value %d: forced colour produced no escape codes", i)
		}
		if got, want := strip(coloured.String()), plain.String(); got != want {
			t.Errorf("value %d: coloured table does not align with the plain one:\ngot:\n%s\nwant:\n%s",
				i, got, want)
		}
	}
}

func TestColourCarriesMeaning(t *testing.T) {
	var buf bytes.Buffer
	if err := NewWithMode(FormatTable, ModeAlways).Format(&buf, colourfulTasks()); err != nil {
		t.Fatalf("format: %v", err)
	}
	out := buf.String()
	cases := map[string]string{
		"reference":   "\x1b[" + string(styleRef) + "mENG-1",
		"todo":        "\x1b[" + string(styleTodo) + "mtodo",
		"in progress": "\x1b[" + string(styleInProgress) + "mdoing",
		"done":        "\x1b[" + string(styleDone) + "mdone",
		"highest":     "\x1b[" + string(styleHighest) + "mhighest",
		"low":         "\x1b[" + string(styleLow) + "mlow",
		"header":      "\x1b[" + string(styleHeader) + "mREF",
	}
	for name, want := range cases {
		if !strings.Contains(out, want) {
			t.Errorf("%s is not coloured: %q", name, out)
		}
	}
	if strings.Contains(out, "\x1b[33mbespoke_state") {
		t.Error("an unknown state was coloured as if its category were known")
	}
}

func TestParseMode(t *testing.T) {
	cases := []struct {
		in   string
		want Mode
		ok   bool
	}{
		{ColorAuto, ModeAuto, true},
		{"", ModeAuto, true},
		{ColorAlways, ModeAlways, true},
		{ColorNever, ModeNever, true},
		{" never ", ModeNever, true},
		{"yes", ModeAuto, false},
	}
	for _, tc := range cases {
		got, ok := ParseMode(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("ParseMode(%q) = %v, %v; want %v, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestStateCategoryOf(t *testing.T) {
	cases := map[string]core.StateCategory{
		"todo":        core.CategoryTodo,
		"BLOCKED":     core.CategoryBlocked,
		"in_progress": core.CategoryInProgress,
		"doing\t":     core.CategoryInProgress,
		"on_hold":     core.CategoryWaiting,
		"cancelled":   core.CategoryCancelled,
		"done":        core.CategoryDone,
		"waiting_ops": "",
	}
	for status, want := range cases {
		if got := StateCategoryOf(status); got != want {
			t.Errorf("StateCategoryOf(%q) = %q, want %q", status, got, want)
		}
	}
}

func TestNoColorSet(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"unset", map[string]string{}, false},
		{"empty", map[string]string{EnvNoColor: "", EnvTixNoColor: ""}, false},
		{"no_color", map[string]string{EnvNoColor: "1"}, true},
		{"tix_no_color", map[string]string{EnvTixNoColor: "anything"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NoColorSet(func(name string) string { return tc.env[name] }); got != tc.want {
				t.Fatalf("NoColorSet = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPainterLeavesEmptyTextAlone(t *testing.T) {
	p := NewPainter(ModeAlways, &bytes.Buffer{})
	if got := p.Ref(""); got != "" {
		t.Errorf("empty text was painted: %q", got)
	}
	if got := p.Priority(core.PriorityNormal, "normal"); got != "normal" {
		t.Errorf("normal priority was painted: %q", got)
	}
	if got := p.Blocked(false, "no"); got != "no" {
		t.Errorf("unblocked marker was painted: %q", got)
	}
	if got := p.StatusIn("", "mystery"); got != "mystery" {
		t.Errorf("unknown category was painted: %q", got)
	}
}

func TestProjectTableShowsColourAndIcon(t *testing.T) {
	project := sampleProject()
	project.Color = core.ColorViolet
	project.Icon = "🚀"

	var plain bytes.Buffer
	if err := NewWithMode(FormatTable, ModeNever).Format(&plain, []core.Project{project}); err != nil {
		t.Fatalf("plain: %v", err)
	}
	if strings.Contains(plain.String(), "\x1b[") {
		t.Errorf("colour off still wrote escape codes:\n%s", plain.String())
	}
	for _, want := range []string{"COLOR", "ICON", "violet", "🚀"} {
		if !strings.Contains(plain.String(), want) {
			t.Errorf("project table is missing %q:\n%s", want, plain.String())
		}
	}

	var painted bytes.Buffer
	if err := NewWithMode(FormatTable, ModeAlways).Format(&painted, []core.Project{project}); err != nil {
		t.Fatalf("painted: %v", err)
	}
	if !strings.Contains(painted.String(), "\x1b[35m"+swatchGlyph+"violet") {
		t.Errorf("colour on did not paint a swatch:\n%q", painted.String())
	}
}

func TestProjectTableWithoutColourOrIcon(t *testing.T) {
	var buf bytes.Buffer
	if err := NewWithMode(FormatTable, ModeAlways).Format(&buf, []core.Project{sampleProject()}); err != nil {
		t.Fatalf("format: %v", err)
	}
	if strings.Contains(strip(buf.String()), swatchGlyph) {
		t.Errorf("a project with no colour drew a swatch:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "ENG") {
		t.Errorf("project row missing:\n%s", buf.String())
	}
}

func TestSwatchCoversThePalette(t *testing.T) {
	on := Painter{on: true}
	off := Painter{on: false}
	if got := on.Swatch(core.ColorNone); got != "" {
		t.Errorf("empty colour rendered %q", got)
	}
	for _, c := range core.ProjectColors() {
		if got := off.Swatch(c); got != string(c) {
			t.Errorf("colour off rendered %q for %s", got, c)
		}
		got := on.Swatch(c)
		if !strings.HasPrefix(got, "\x1b[") || strip(got) != swatchGlyph+string(c) {
			t.Errorf("colour on rendered %q for %s", got, c)
		}
	}
}

// TestEveryCategoryIsPaintedDistinctlyAndDegrades is the command line's half
// of the collision guard the board carries. The widened vocabulary is worth
// having only if a reader can tell the six apart, and the pair that mattered
// is cancelled against done: painted alike, abandoned work reads as finished.
//
// It asserts through the painter rather than looking for a colour, so a style
// constant edited to another category's parameter fails here.
func TestEveryCategoryIsPaintedDistinctlyAndDegrades(t *testing.T) {
	on := Painter{on: true}
	painted := map[string]core.StateCategory{}
	for _, c := range core.StateCategories() {
		got := on.StatusIn(c, "x")
		if got == "x" {
			t.Errorf("category %q is painted plainly, so it carries no meaning", c)
		}
		if other, clash := painted[got]; clash {
			t.Errorf("categories %q and %q are painted identically as %q", other, c, got)
		}
		painted[got] = c
	}
	if len(painted) != len(core.StateCategories()) {
		t.Fatalf("%d categories paint %d distinct styles", len(core.StateCategories()), len(painted))
	}
	// A category this build does not know is painted plainly rather than
	// guessed at, which is the rule a user-defined workflow depends on.
	if got := on.StatusIn(core.StateCategory("invented"), "x"); got != "x" {
		t.Errorf("an unknown category was painted %q", got)
	}
	// With colour off, every category writes the text and nothing else.
	off := Painter{}
	for _, c := range core.StateCategories() {
		if got := off.StatusIn(c, "x"); got != "x" {
			t.Errorf("category %q wrote %q with colour off", c, got)
		}
	}
}

// TestTheTerminalAndTheBoardAgreeOnEveryCategorysParameter is the parity the
// comment over the board's colour table claims and nothing asserted. Those
// colours are written as the parameters below minus the 30/90 offset, so the
// two surfaces can only drift while this goes unchecked.
func TestTheTerminalAndTheBoardAgreeOnEveryCategorysParameter(t *testing.T) {
	want := map[core.StateCategory]string{
		core.CategoryTodo:       "33",
		core.CategoryInProgress: "94",
		core.CategoryBlocked:    "31",
		core.CategoryWaiting:    "35",
		core.CategoryDone:       "32",
		core.CategoryCancelled:  "90",
	}
	if len(want) != len(core.StateCategories()) {
		t.Fatalf("the table names %d categories for a vocabulary of %d",
			len(want), len(core.StateCategories()))
	}
	on := Painter{on: true}
	for _, c := range core.StateCategories() {
		param, ok := want[c]
		if !ok {
			t.Fatalf("category %q has no parameter pinned", c)
		}
		if got := on.StatusIn(c, "x"); got != "\x1b["+param+"mx\x1b[0m" {
			t.Errorf("category %q paints %q, want parameter %s", c, got, param)
		}
	}
}
