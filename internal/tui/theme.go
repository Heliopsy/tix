// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/heliopsy/tix/internal/core"
)

// EnvNoColor is the variable that disables colour output.
const EnvNoColor = "NO_COLOR"

// EnvTixNoColor is the tix-scoped spelling of EnvNoColor.
const EnvTixNoColor = "TIX_NO_COLOR"

// Terminal colours, chosen so every styled token renders the same select
// graphic rendition parameter the CLI writes from internal/output/color.go.
// An ANSI colour 0..7 renders as 30..37 and 8..15 as 90..97, so the numbers
// here are the CLI's parameters minus that offset.
const (
	colorRef        = lipgloss.Color("6")  // cli 36
	colorTitle      = lipgloss.Color("15") // cli 1, brightened for a header bar
	colorMuted      = lipgloss.Color("8")  // cli 90
	colorTodo       = lipgloss.Color("3")  // cli 33
	colorInProgress = lipgloss.Color("12") // cli 94
	colorDone       = lipgloss.Color("2")  // cli 32
	colorUrgent     = lipgloss.Color("1")  // cli 31, bold for the highest
	colorAccent     = lipgloss.Color("14")
	colorOK         = lipgloss.Color("10")
)

// Theme holds the styles a frame is rendered with.
type Theme struct {
	Color bool
	// renderer draws every style this theme builds. Colour depth is resolved
	// per renderer, so a theme built against one client's terminal cannot
	// change the depth another client is drawn at.
	renderer *lipgloss.Renderer
	Title    lipgloss.Style
	Header   lipgloss.Style
	Selected lipgloss.Style
	Claimed  lipgloss.Style
	Dim      lipgloss.Style
	Error    lipgloss.Style
	Status   lipgloss.Style
	Ref      lipgloss.Style
	Blocked  lipgloss.Style
	Empty    lipgloss.Style
	Bar      lipgloss.Style
	Column   lipgloss.Style
	Focused  lipgloss.Style
}

// ColorEnabled reports whether colour may be written for this environment and
// this destination. It follows the CLI: either NO_COLOR spelling opts out, a
// dumb terminal opts out, and anything that is not a character device opts out.
func ColorEnabled(environ []string, out io.Writer) bool {
	for _, name := range []string{EnvNoColor, EnvTixNoColor} {
		if value, ok := lookupEnv(environ, name); ok && value != "" {
			return false
		}
	}
	if term, ok := lookupEnv(environ, "TERM"); ok && term == "dumb" {
		return false
	}
	return isTerminal(out)
}

// colorChoice resolves whether this run draws in colour. An explicit choice on
// the configuration wins, because a caller writing to something that is not a
// file knows better than a device probe can; otherwise the environment decides.
func colorChoice(cfg Config) bool {
	if cfg.Color != nil {
		return *cfg.Color
	}
	return ColorEnabled(cfg.Environ, cfg.Out)
}

// isTerminal reports whether w is a character device. A nil writer stands for
// the terminal bubbletea writes to when no output was configured.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		if w != nil {
			return false
		}
		f = os.Stdout
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// lookupEnv returns the last value of name in an environ slice.
func lookupEnv(environ []string, name string) (string, bool) {
	prefix := name + "="
	for i := len(environ) - 1; i >= 0; i-- {
		if value, ok := strings.CutPrefix(environ[i], prefix); ok {
			return value, true
		}
	}
	return "", false
}

// NewTheme builds the styles, emitting no attributes at all without colour.
// A nil renderer takes the process-wide default, which is what a single local
// terminal wants; a caller drawing several terminals at once passes one
// renderer per terminal.
func NewTheme(r *lipgloss.Renderer, color bool) Theme {
	if r == nil {
		r = lipgloss.DefaultRenderer()
	}
	if !color {
		plain := r.NewStyle()
		border := r.NewStyle().Border(lipgloss.NormalBorder())
		return Theme{
			renderer: r,
			Title:    plain, Header: plain, Selected: plain, Claimed: plain,
			Dim: plain, Error: plain, Status: plain, Ref: plain,
			Blocked: plain, Empty: plain, Bar: plain,
			Column: border, Focused: border,
		}
	}
	return Theme{
		Color:    true,
		renderer: r,
		Title:    r.NewStyle().Bold(true).Foreground(colorTitle),
		Header:   r.NewStyle().Bold(true).Foreground(lipgloss.Color("12")),
		Selected: r.NewStyle().Bold(true).Foreground(colorAccent),
		Claimed:  r.NewStyle().Foreground(lipgloss.Color("11")),
		Dim:      r.NewStyle().Foreground(colorMuted),
		Error:    r.NewStyle().Bold(true).Foreground(colorUrgent),
		Status:   r.NewStyle().Foreground(colorOK),
		Ref:      r.NewStyle().Foreground(colorRef),
		Blocked:  r.NewStyle().Foreground(colorUrgent),
		Empty:    r.NewStyle().Foreground(colorMuted).Italic(true),
		Bar:      r.NewStyle().Foreground(colorMuted),
		Column:   r.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(colorMuted),
		Focused:  r.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorAccent),
	}
}

// Style starts a style bound to this theme's renderer, so a style built after
// the theme still renders at that terminal's depth. The zero Theme falls back
// to the process-wide default renderer.
func (t Theme) Style() lipgloss.Style {
	if t.renderer == nil {
		return lipgloss.NewStyle()
	}
	return t.renderer.NewStyle()
}

// CategoryColor returns the colour a workflow state category is drawn in.
// An unknown category has no colour, because workflows are user defined and a
// guessed category would be a lie, which is the rule the CLI follows too.
func CategoryColor(c core.StateCategory) (lipgloss.Color, bool) {
	switch c {
	case core.CategoryTodo:
		return colorTodo, true
	case core.CategoryInProgress:
		return colorInProgress, true
	case core.CategoryDone:
		return colorDone, true
	default:
		return "", false
	}
}

// PriorityColor returns the colour a priority is drawn in, and whether the
// priority is distinguished at all. Normal priority carries no colour so the
// ends of the range are what stands out.
func PriorityColor(p core.Priority) (lipgloss.Color, bool, bool) {
	switch p {
	case core.PriorityHighest:
		return colorUrgent, true, true
	case core.PriorityHigh:
		return colorUrgent, false, true
	case core.PriorityLow, core.PriorityLowest:
		return colorMuted, false, true
	default:
		return "", false, false
	}
}

// PriorityLabel names a priority in the CLI's words.
func PriorityLabel(p core.Priority) string {
	switch p {
	case core.PriorityHighest:
		return "highest"
	case core.PriorityHigh:
		return "high"
	case core.PriorityNormal:
		return "normal"
	case core.PriorityLow:
		return "low"
	case core.PriorityLowest:
		return "lowest"
	default:
		return "unset"
	}
}

// projectColors maps each project palette colour onto a terminal colour, the
// same way the CLI's swatches do.
var projectColors = map[core.ProjectColor]lipgloss.Color{
	core.ColorSlate:  "8",
	core.ColorRed:    "1",
	core.ColorAmber:  "3",
	core.ColorGreen:  "2",
	core.ColorTeal:   "6",
	core.ColorBlue:   "4",
	core.ColorViolet: "5",
	core.ColorPink:   "13",
}

// ProjectColor returns the terminal colour a project's palette colour maps to.
func ProjectColor(c core.ProjectColor) (lipgloss.Color, bool) {
	value, ok := projectColors[c]
	return value, ok
}

// Category styles text by the workflow state category it belongs to.
func (t Theme) Category(c core.StateCategory) lipgloss.Style {
	color, ok := CategoryColor(c)
	if !ok || !t.Color {
		return t.Style()
	}
	return t.Style().Foreground(color)
}

// Priority styles text so the ends of the priority range stand out.
func (t Theme) Priority(p core.Priority) lipgloss.Style {
	color, bold, ok := PriorityColor(p)
	if !ok || !t.Color {
		return t.Style()
	}
	return t.Style().Foreground(color).Bold(bold)
}

// Project styles text in a project's own palette colour.
func (t Theme) Project(c core.ProjectColor) lipgloss.Style {
	color, ok := ProjectColor(c)
	if !ok || !t.Color {
		return t.Style()
	}
	return t.Style().Foreground(color)
}

// SelectionMarker renders selection as text so colour is never the only cue.
func SelectionMarker(selected bool) string {
	if selected {
		return "▸ "
	}
	return "  "
}

// ProjectGlyph renders a project's icon, falling back to a neutral marker so
// every row in the list starts at the same column.
func ProjectGlyph(p core.Project) string {
	if icon := strings.TrimSpace(p.Icon); icon != "" {
		return icon
	}
	return "•"
}
