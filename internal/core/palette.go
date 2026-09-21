package core

import (
	"strings"
	"unicode"
)

// ProjectColor names one swatch of the fixed project palette. A project
// carries a token rather than a hex value so every scheme can render it with
// its own tuned shade.
type ProjectColor string

// The project palette. An empty colour means the project has none.
const (
	ColorNone   ProjectColor = ""
	ColorSlate  ProjectColor = "slate"
	ColorRed    ProjectColor = "red"
	ColorAmber  ProjectColor = "amber"
	ColorGreen  ProjectColor = "green"
	ColorTeal   ProjectColor = "teal"
	ColorBlue   ProjectColor = "blue"
	ColorViolet ProjectColor = "violet"
	ColorPink   ProjectColor = "pink"
)

// projectColors is the whole vocabulary, in display order.
var projectColors = []ProjectColor{
	ColorSlate, ColorRed, ColorAmber, ColorGreen,
	ColorTeal, ColorBlue, ColorViolet, ColorPink,
}

// ProjectColors returns the palette a project colour may be chosen from.
func ProjectColors() []ProjectColor {
	out := make([]ProjectColor, len(projectColors))
	copy(out, projectColors)
	return out
}

// Valid reports whether c is the empty colour or a palette member.
func (c ProjectColor) Valid() bool {
	if c == ColorNone {
		return true
	}
	for _, known := range projectColors {
		if c == known {
			return true
		}
	}
	return false
}

// String renders the colour token.
func (c ProjectColor) String() string { return string(c) }

// NormalizeProjectColor trims and lowercases a colour, then checks it against
// the palette.
func NormalizeProjectColor(s string) (ProjectColor, error) {
	c := ProjectColor(strings.ToLower(strings.TrimSpace(s)))
	if !c.Valid() {
		return ColorNone, Invalid("project colour %q is not in the palette; choose one of %s",
			s, joinColors(projectColors))
	}
	return c, nil
}

// MaxProjectIconRunes bounds an icon so a row stays a row.
const MaxProjectIconRunes = 2

// NormalizeProjectIcon trims an icon and checks it is a short printable token,
// which is satisfied by one emoji or a one or two letter monogram.
func NormalizeProjectIcon(s string) (string, error) {
	icon := strings.TrimSpace(s)
	if icon == "" {
		return "", nil
	}
	runes := []rune(icon)
	if len(runes) > MaxProjectIconRunes {
		return "", Invalid("project icon %q is longer than %d characters; use a single emoji or a short monogram",
			s, MaxProjectIconRunes)
	}
	for _, r := range runes {
		if unicode.IsControl(r) || unicode.IsSpace(r) || !unicode.IsPrint(r) {
			return "", Invalid("project icon %q contains a character that cannot be displayed", s)
		}
	}
	return icon, nil
}

// joinColors renders the palette for an error message.
func joinColors(colors []ProjectColor) string {
	parts := make([]string, 0, len(colors))
	for _, c := range colors {
		parts = append(parts, string(c))
	}
	return strings.Join(parts, ", ")
}
