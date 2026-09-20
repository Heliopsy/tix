package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// EnvNoColor is the variable that disables colour output.
const EnvNoColor = "NO_COLOR"

// Theme holds the styles a frame is rendered with.
type Theme struct {
	Color    bool
	Title    lipgloss.Style
	Header   lipgloss.Style
	Selected lipgloss.Style
	Claimed  lipgloss.Style
	Dim      lipgloss.Style
	Error    lipgloss.Style
	Status   lipgloss.Style
}

// ColorEnabled reports whether colour may be used for the given environment.
func ColorEnabled(environ []string) bool {
	if value, ok := lookupEnv(environ, EnvNoColor); ok && value != "" {
		return false
	}
	term, _ := lookupEnv(environ, "TERM")
	return term != "dumb"
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
func NewTheme(color bool) Theme {
	if !color {
		plain := lipgloss.NewStyle()
		return Theme{
			Title: plain, Header: plain, Selected: plain,
			Claimed: plain, Dim: plain, Error: plain, Status: plain,
		}
	}
	return Theme{
		Color:    true,
		Title:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")),
		Header:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")),
		Selected: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14")),
		Claimed:  lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
		Dim:      lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		Error:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9")),
		Status:   lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
	}
}

// SelectionMarker renders selection as text so colour is never the only cue.
func SelectionMarker(selected bool) string {
	if selected {
		return "> "
	}
	return "  "
}
