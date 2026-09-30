// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"image/color"
	"io"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
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
var (
	colorRef        = lipgloss.Color("6")  // cli 36
	colorTitle      = lipgloss.Color("15") // cli 1, brightened for a header bar
	colorMuted      = lipgloss.Color("8")  // cli 90
	colorTodo       = lipgloss.Color("3")  // cli 33
	colorInProgress = lipgloss.Color("12") // cli 94
	colorWaiting    = lipgloss.Color("5")  // cli 35
	colorDone       = lipgloss.Color("2")  // cli 32
	colorUrgent     = lipgloss.Color("1")  // cli 31, bold for the highest
	colorAccent     = lipgloss.Color("14")
	colorOK         = lipgloss.Color("10")
)

// Theme holds the styles a frame is rendered with.
type Theme struct {
	// Color reports whether this theme writes attributes at all. A run without
	// colour writes none, not even weight, so a frame it draws carries no
	// escape sequence anywhere.
	Color bool
	// profile is the colour depth every style this theme builds is flattened
	// to. It is resolved per connection, so a theme built against one client's
	// terminal cannot change the depth another client is drawn at.
	profile  colorprofile.Profile
	Title    lipgloss.Style
	Header   lipgloss.Style
	Selected lipgloss.Style
	// SelectedQuiet is the selected row at the low phase of the pulse. It
	// keeps the selection's colour and drops only its weight, so a reader who
	// looks mid-cycle still sees which row is selected.
	SelectedQuiet lipgloss.Style
	Claimed       lipgloss.Style
	Dim           lipgloss.Style
	Error         lipgloss.Style
	Status        lipgloss.Style
	Ref           lipgloss.Style
	Blocked       lipgloss.Style
	Empty         lipgloss.Style
	Bar           lipgloss.Style
	Column        lipgloss.Style
	Focused       lipgloss.Style
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

// NewTheme builds the styles a frame is rendered with, emitting no attributes
// at all without colour.
//
// The zero profile leaves every colour at full fidelity and lets the output
// layer flatten it, which is what a single local terminal wants; a caller
// drawing several terminals at once passes one profile per terminal.
//
// brand is the tenant's resolved accent, and it reaches only the three places
// a brand belongs: the header, the selection, and the focused column's border.
// Everything else keeps a fixed colour because it carries meaning rather than
// identity, and a tenant that themes itself red must not lose the red that
// means blocked.
//
// An empty brand accent keeps the built-in accent, which is what a tenant that
// names no theme and every non-tenant context gets.
func NewTheme(profile colorprofile.Profile, color bool, brand core.Theme) Theme {
	t := Theme{profile: profile}
	if !color {
		plain := lipgloss.NewStyle()
		border := lipgloss.NewStyle().Border(lipgloss.NormalBorder())
		t.Title, t.Header, t.Selected = plain, plain, plain
		t.SelectedQuiet, t.Claimed, t.Dim = plain, plain, plain
		t.Error, t.Status, t.Ref = plain, plain, plain
		t.Blocked, t.Empty, t.Bar = plain, plain, plain
		t.Column, t.Focused = border, border
		return t
	}
	accent := colorAccent
	header := lipgloss.Color("12")
	if brand.Accent != "" {
		// Validated as #rrggbb in core before it ever gets here; lipgloss
		// takes hex directly and the profile downsamples it for a shallower
		// terminal.
		accent = lipgloss.Color(brand.Accent)
		header = accent
	}
	t.Color = true
	t.Title = t.Foreground(colorTitle).Bold(true)
	t.Header = t.Foreground(header).Bold(true)
	t.Selected = t.Foreground(accent).Bold(true)
	t.SelectedQuiet = t.Foreground(accent)
	t.Claimed = t.Foreground(lipgloss.Color("11"))
	t.Dim = t.Foreground(colorMuted)
	t.Error = t.Foreground(colorUrgent).Bold(true)
	t.Status = t.Foreground(colorOK)
	t.Ref = t.Foreground(colorRef)
	t.Blocked = t.Foreground(colorUrgent)
	t.Empty = t.Foreground(colorMuted).Italic(true)
	t.Bar = t.Foreground(colorMuted)
	t.Column = t.Style().Border(lipgloss.NormalBorder()).BorderForeground(t.flatten(colorMuted))
	t.Focused = t.Style().Border(lipgloss.RoundedBorder()).BorderForeground(t.flatten(accent))
	return t
}

// Selection styles the selected row at one phase of the pulse. The emphasised
// phase is what a static selection also draws, so a reader who turned the
// motion off and a reader whose session has gone idle see the same row.
func (t Theme) Selection(emphasis bool) lipgloss.Style {
	if emphasis {
		return t.Selected
	}
	return t.SelectedQuiet
}

// Style starts an uncoloured style. Anything that carries a colour goes
// through Foreground instead, which is what binds it to this terminal's depth.
func (t Theme) Style() lipgloss.Style { return lipgloss.NewStyle() }

// Foreground starts a style coloured for this theme's terminal, so a style
// built after the theme still draws at that terminal's depth.
func (t Theme) Foreground(c color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.flatten(c))
}

// flatten drops a colour to the depth this theme's terminal declared. The zero
// Theme keeps it as it is and leaves the flattening to the output layer.
func (t Theme) flatten(c color.Color) color.Color {
	if t.profile == colorprofile.Unknown {
		return c
	}
	return t.profile.Convert(c)
}

// CategoryColor returns the colour a workflow state category is drawn in.
// An unknown category has no colour, because workflows are user defined and a
// guessed category would be a lie, which is the rule the CLI follows too.
//
// Blocked borrows the urgent colour and cancelled borrows the muted one, for
// the reason DueColor borrows urgent for overdue: both say the thing a reader
// scanning a board needs, and a hue per category beyond what a reader can name
// is a distinction without a difference.
func CategoryColor(c core.StateCategory) (color.Color, bool) {
	switch c {
	case core.CategoryTodo:
		return colorTodo, true
	case core.CategoryInProgress:
		return colorInProgress, true
	case core.CategoryBlocked:
		return colorUrgent, true
	case core.CategoryWaiting:
		return colorWaiting, true
	case core.CategoryDone:
		return colorDone, true
	case core.CategoryCancelled:
		return colorMuted, true
	default:
		return nil, false
	}
}

// PriorityColor returns the colour a priority is drawn in, and whether the
// priority is distinguished at all. Normal priority carries no colour so the
// ends of the range are what stands out.
func PriorityColor(p core.Priority) (color.Color, bool, bool) {
	switch p {
	case core.PriorityHighest:
		return colorUrgent, true, true
	case core.PriorityHigh:
		return colorUrgent, false, true
	case core.PriorityLow, core.PriorityLowest:
		return colorMuted, false, true
	default:
		return nil, false, false
	}
}

// DueColor returns the colour a due state is drawn in, whether it is drawn
// bold, and whether the state is distinguished at all. A deadline still some
// way off carries no colour, the same way a normal priority does: what is
// marked is what is running out of time.
//
// Overdue borrows the colour "blocked" already uses, because both say the same
// thing to a reader scanning a board — this one is not going to move on its
// own — and a sixth hue on a card would be a distinction without a difference.
func DueColor(d core.DueState) (color.Color, bool, bool) {
	switch d {
	case core.DueOverdue:
		return colorUrgent, true, true
	case core.DueSoon:
		return colorTodo, false, true
	default:
		return nil, false, false
	}
}

// Due styles text by how near its deadline is. Like every other style here it
// answers the uncoloured theme with a plain style, so the marker the board
// draws is carried by its own text on a terminal getting no escapes at all.
func (t Theme) Due(d core.DueState) lipgloss.Style {
	fg, bold, ok := DueColor(d)
	if !ok || !t.Color {
		return t.Style()
	}
	return t.Foreground(fg).Bold(bold)
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
var projectColors = map[core.ProjectColor]color.Color{
	core.ColorSlate:  lipgloss.Color("8"),
	core.ColorRed:    lipgloss.Color("1"),
	core.ColorAmber:  lipgloss.Color("3"),
	core.ColorGreen:  lipgloss.Color("2"),
	core.ColorTeal:   lipgloss.Color("6"),
	core.ColorBlue:   lipgloss.Color("4"),
	core.ColorViolet: lipgloss.Color("5"),
	core.ColorPink:   lipgloss.Color("13"),
}

// ProjectColor returns the terminal colour a project's palette colour maps to.
func ProjectColor(c core.ProjectColor) (color.Color, bool) {
	value, ok := projectColors[c]
	return value, ok
}

// Category styles text by the workflow state category it belongs to.
func (t Theme) Category(c core.StateCategory) lipgloss.Style {
	fg, ok := CategoryColor(c)
	if !ok || !t.Color {
		return t.Style()
	}
	return t.Foreground(fg)
}

// Priority styles text so the ends of the priority range stand out.
func (t Theme) Priority(p core.Priority) lipgloss.Style {
	fg, bold, ok := PriorityColor(p)
	if !ok || !t.Color {
		return t.Style()
	}
	return t.Foreground(fg).Bold(bold)
}

// Project styles text in a project's own palette colour.
func (t Theme) Project(c core.ProjectColor) lipgloss.Style {
	fg, ok := ProjectColor(c)
	if !ok || !t.Color {
		return t.Style()
	}
	return t.Foreground(fg)
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
