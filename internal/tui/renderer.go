// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"io"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// NewRenderer builds a renderer that takes its colour depth from one client's
// own terminal.
//
// lipgloss resolves a depth once per renderer and then caches it, so a process
// drawing several terminals at once needs one renderer each: share one and
// every client renders at the depth of whichever was measured first. The probe
// normally ends in a character-device test, which a network stream can never
// pass, so it is skipped here and the terminal type the client declared
// decides instead. environ is read last wins, which is how a session reports
// the terminal type of the pseudo-terminal it allocated.
func NewRenderer(environ []string, out io.Writer) *lipgloss.Renderer {
	return lipgloss.NewRenderer(out,
		termenv.WithEnvironment(environSource(environ)),
		termenv.WithUnsafe())
}

// environSource adapts an environ slice to what termenv reads from.
type environSource []string

// Environ implements termenv.Environ.
func (e environSource) Environ() []string { return e }

// Getenv implements termenv.Environ.
func (e environSource) Getenv(name string) string {
	value, _ := lookupEnv(e, name)
	return value
}
