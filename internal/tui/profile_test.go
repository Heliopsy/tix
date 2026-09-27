// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"sync"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/heliopsy/tix/internal/core"
)

// session is one client's terminal and the bytes it is owed.
type session struct {
	name    string
	environ []string
	profile colorprofile.Profile
	// ref is the reference style, which is ANSI colour 6 in every palette the
	// interface uses, and hex is a colour only a deeper terminal can render.
	ref string
	hex string
}

var sessions = []session{
	{
		name:    "a truecolour client",
		environ: []string{"TERM=xterm-256color", "COLORTERM=truecolor"},
		profile: colorprofile.TrueColor,
		ref:     "\x1b[36mTIX-1\x1b[m",
		hex:     "\x1b[38;2;255;95;0mx\x1b[m",
	},
	{
		name:    "a 256 colour client",
		environ: []string{"TERM=xterm-256color"},
		profile: colorprofile.ANSI256,
		ref:     "\x1b[36mTIX-1\x1b[m",
		hex:     "\x1b[38;5;202mx\x1b[m",
	},
	{
		name:    "a sixteen colour client",
		environ: []string{"TERM=xterm"},
		profile: colorprofile.ANSI,
		ref:     "\x1b[36mTIX-1\x1b[m",
		hex:     "\x1b[91mx\x1b[m",
	},
	{
		name:    "a client whose terminal has no colour at all",
		environ: []string{"TERM=dumb"},
		profile: colorprofile.NoTTY,
		ref:     "TIX-1",
		hex:     "x",
	},
}

func TestNewProfileReadsTheClientsOwnTerminal(t *testing.T) {
	for _, tc := range sessions {
		t.Run(tc.name, func(t *testing.T) {
			if got := NewProfile(tc.environ); got != tc.profile {
				t.Fatalf("profile for %q = %v, want %v", tc.environ, got, tc.profile)
			}
		})
	}
}

// TestSessionsHeldAtOnceRenderAtTheirOwnDepth is the whole point of the
// per-session profile: a depth belongs to one client's terminal, so the failure
// this guards against only appears while several clients are held at the same
// time. Every session therefore resolves its profile and renders inside its own
// goroutine, all of them released together and looping so the construction of
// one overlaps the rendering of another. A single shared profile fails this:
// every session would carry the depth of whichever was measured first.
func TestSessionsHeldAtOnceRenderAtTheirOwnDepth(t *testing.T) {
	const rounds = 500

	start := make(chan struct{})
	var open sync.WaitGroup
	var done sync.WaitGroup
	open.Add(len(sessions))
	done.Add(len(sessions))

	for _, tc := range sessions {
		go func() {
			defer done.Done()
			open.Done()
			<-start
			for range rounds {
				theme := NewTheme(NewProfile(tc.environ), true, core.Theme{})
				if got := theme.Ref.Render("TIX-1"); got != tc.ref {
					t.Errorf("%s rendered the reference as %q, want %q", tc.name, got, tc.ref)
					return
				}
				hex := theme.Foreground(lipgloss.Color("#ff5f00")).Render("x")
				if hex != tc.hex {
					t.Errorf("%s rendered a deep colour as %q, want %q", tc.name, hex, tc.hex)
					return
				}
			}
		}()
	}

	open.Wait()
	close(start)
	done.Wait()

	if t.Failed() {
		return
	}
	seen := map[string]string{}
	for _, tc := range sessions {
		if other, clash := seen[tc.hex]; clash {
			t.Fatalf("%s and %s are indistinguishable, so this proves nothing", other, tc.name)
		}
		seen[tc.hex] = tc.name
	}
}

// TestThemeKeepsItsOwnProfile fixes the regression the per-session profile
// exists to prevent: one session's terminal reaching another's. A theme built
// for a flattened terminal must stay flat while a theme built beside it for a
// colour terminal still renders in colour.
func TestThemeKeepsItsOwnProfile(t *testing.T) {
	flat := NewTheme(NewProfile([]string{"TERM=dumb"}), true, core.Theme{})
	if got := flat.Ref.Render("TIX-1"); strings.Contains(got, "\x1b[") {
		t.Fatalf("a flattened terminal rendered %q", got)
	}
	theme := NewTheme(NewProfile([]string{"TERM=xterm-256color"}), true, core.Theme{})
	if got := theme.Ref.Render("TIX-1"); got != "\x1b[36mTIX-1\x1b[m" {
		t.Fatalf("a session with its own profile rendered %q, want the reference in colour", got)
	}
	if got := theme.Foreground(colorRef).Render("TIX-1"); got != "\x1b[36mTIX-1\x1b[m" {
		t.Fatalf("a style built from the theme rendered %q, want the reference in colour", got)
	}
	if got := flat.Foreground(colorRef).Render("TIX-1"); got != "TIX-1" {
		t.Fatalf("a style built from the flattened theme rendered %q", got)
	}
}
