package output

import (
	"io"
	"os"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// Colour mode names accepted by the output.color key and the colour flags.
const (
	ColorAuto   = "auto"
	ColorAlways = "always"
	ColorNever  = "never"
)

// ColorModes lists every accepted colour mode.
var ColorModes = []string{ColorAuto, ColorAlways, ColorNever}

// EnvNoColor is the cross-tool opt-out described by no-color.org.
const EnvNoColor = "NO_COLOR"

// EnvTixNoColor is the tix-scoped spelling of EnvNoColor.
const EnvTixNoColor = "TIX_NO_COLOR"

// Mode decides whether ANSI colour may be written.
type Mode int

// Colour modes.
const (
	// ModeAuto colours only a terminal.
	ModeAuto Mode = iota
	// ModeAlways colours whatever the writer is, for piping into a pager.
	ModeAlways
	// ModeNever never colours.
	ModeNever
)

// ParseMode maps a colour mode name onto a Mode, reporting an unknown name.
func ParseMode(name string) (Mode, bool) {
	switch strings.TrimSpace(name) {
	case ColorAuto, "":
		return ModeAuto, true
	case ColorAlways:
		return ModeAlways, true
	case ColorNever:
		return ModeNever, true
	default:
		return ModeAuto, false
	}
}

// NoColorSet reports whether a NO_COLOR-style variable opts this process out.
// Per no-color.org the variable counts when present and not empty.
func NoColorSet(lookup func(string) string) bool {
	for _, name := range []string{EnvNoColor, EnvTixNoColor} {
		if lookup(name) != "" {
			return true
		}
	}
	return false
}

// sgr is a select graphic rendition parameter list, such as "1;31".
type sgr string

// Styles. Meaning first: state category, priority, identity, severity.
const (
	styleRef        sgr = "36"
	styleHeader     sgr = "1"
	styleMuted      sgr = "90"
	styleTodo       sgr = "33"
	styleInProgress sgr = "94"
	styleDone       sgr = "32"
	styleHighest    sgr = "1;31"
	styleHigh       sgr = "31"
	styleLow        sgr = "90"
	styleBlocked    sgr = "31"
	styleError      sgr = "1;31"
	styleWarn       sgr = "33"
)

// Painter applies ANSI styles when colour is enabled for one writer, and
// carries the TimeStyle every timestamp it renders is formatted through.
type Painter struct {
	on    bool
	style TimeStyle
}

// NewPainter returns a Painter for writing to w under the given mode, with the
// zero-value TimeStyle. Prefer NewPainterWithStyle wherever a timestamp will
// be rendered; this constructor stays for callers, such as an error label,
// that never format a time.
func NewPainter(mode Mode, w io.Writer) Painter {
	return NewPainterWithStyle(mode, w, TimeStyle{})
}

// NewPainterWithStyle returns a Painter for writing to w under the given
// mode, rendering every timestamp through style.
func NewPainterWithStyle(mode Mode, w io.Writer, style TimeStyle) Painter {
	return Painter{on: colorAllowed(mode, w), style: style}
}

// Enabled reports whether this Painter writes escape codes.
func (p Painter) Enabled() bool { return p.on }

func (p Painter) apply(s sgr, text string) string {
	if !p.on || text == "" {
		return text
	}
	return "\x1b[" + string(s) + "m" + text + "\x1b[0m"
}

// Ref styles a task reference so it stands out when scanning a listing.
func (p Painter) Ref(text string) string { return p.apply(styleRef, text) }

// Header styles a table header cell.
func (p Painter) Header(text string) string { return p.apply(styleHeader, text) }

// Muted styles secondary text such as a diagnostic or a table rule.
func (p Painter) Muted(text string) string { return p.apply(styleMuted, text) }

// Error styles a failure written to standard error.
func (p Painter) Error(text string) string { return p.apply(styleError, text) }

// Warn styles a warning written to standard error.
func (p Painter) Warn(text string) string { return p.apply(styleWarn, text) }

// Status styles a workflow state by the category it belongs to. An unknown
// state keeps the default colour, because workflows are user defined and a
// guessed category would be a lie.
func (p Painter) Status(status string) string {
	return p.StatusIn(StateCategoryOf(status), status)
}

// StatusIn styles text by a state category the caller already knows.
func (p Painter) StatusIn(category core.StateCategory, text string) string {
	switch category {
	case core.CategoryTodo:
		return p.apply(styleTodo, text)
	case core.CategoryInProgress:
		return p.apply(styleInProgress, text)
	case core.CategoryDone:
		return p.apply(styleDone, text)
	default:
		return text
	}
}

// Priority styles a priority label so the top of the queue stands out.
func (p Painter) Priority(priority core.Priority, text string) string {
	switch priority {
	case core.PriorityHighest:
		return p.apply(styleHighest, text)
	case core.PriorityHigh:
		return p.apply(styleHigh, text)
	case core.PriorityLow, core.PriorityLowest:
		return p.apply(styleLow, text)
	default:
		return text
	}
}

// Blocked styles a blocked marker.
func (p Painter) Blocked(blocked bool, text string) string {
	if !blocked {
		return text
	}
	return p.apply(styleBlocked, text)
}

// knownCategories maps the state keys tix ships, plus the spellings other
// trackers use, onto their category. A key that is not listed has no known
// category: only the workflow that defines it can say.
var knownCategories = map[string]core.StateCategory{
	"todo":        core.CategoryTodo,
	"to_do":       core.CategoryTodo,
	"backlog":     core.CategoryTodo,
	"open":        core.CategoryTodo,
	"new":         core.CategoryTodo,
	"blocked":     core.CategoryTodo,
	"doing":       core.CategoryInProgress,
	"in_progress": core.CategoryInProgress,
	"in-progress": core.CategoryInProgress,
	"started":     core.CategoryInProgress,
	"review":      core.CategoryInProgress,
	"wip":         core.CategoryInProgress,
	"done":        core.CategoryDone,
	"closed":      core.CategoryDone,
	"complete":    core.CategoryDone,
	"completed":   core.CategoryDone,
	"resolved":    core.CategoryDone,
	"cancelled":   core.CategoryDone,
	"canceled":    core.CategoryDone,
}

// StateCategoryOf returns the category a state key belongs to, or the empty
// category when only the workflow that defines the state could know.
func StateCategoryOf(status string) core.StateCategory {
	return knownCategories[strings.ToLower(strings.TrimSpace(status))]
}

// colorAllowed reports whether ANSI colour may be written to w.
func colorAllowed(mode Mode, w io.Writer) bool {
	switch mode {
	case ModeNever:
		return false
	case ModeAlways:
		return true
	default:
		return !NoColorSet(os.Getenv) && isTerminal(w)
	}
}

// isTerminal reports whether w is a character device.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// paletteStyles maps each project palette colour onto the closest terminal
// colour. A colour outside the palette has no style, so it renders plainly.
var paletteStyles = map[core.ProjectColor]sgr{
	core.ColorSlate:  "90",
	core.ColorRed:    "31",
	core.ColorAmber:  "33",
	core.ColorGreen:  "32",
	core.ColorTeal:   "36",
	core.ColorBlue:   "34",
	core.ColorViolet: "35",
	core.ColorPink:   "95",
}

// swatchGlyph marks a colour sample in a table cell.
const swatchGlyph = "● "

// Swatch renders a project colour as a sample in that colour, and as the bare
// colour name when colour is off. An empty colour renders as nothing.
func (p Painter) Swatch(c core.ProjectColor) string {
	if c == core.ColorNone {
		return ""
	}
	style, ok := paletteStyles[c]
	if !ok || !p.on {
		return string(c)
	}
	return p.apply(style, swatchGlyph+string(c))
}
