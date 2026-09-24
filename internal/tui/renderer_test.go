// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"github.com/heliopsy/tix/internal/core"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// session is one client's terminal and the bytes it is owed.
type session struct {
	name    string
	environ []string
	profile termenv.Profile
	// ref is the reference style, which is ANSI colour 6 in every palette the
	// interface uses, and hex is a colour only a deeper terminal can render.
	ref string
	hex string
}

var sessions = []session{
	{
		name:    "a truecolour client",
		environ: []string{"TERM=xterm-256color", "COLORTERM=truecolor"},
		profile: termenv.TrueColor,
		ref:     "\x1b[36mTIX-1\x1b[0m",
		hex:     "\x1b[38;2;255;95;0mx\x1b[0m",
	},
	{
		name:    "a 256 colour client",
		environ: []string{"TERM=xterm-256color"},
		profile: termenv.ANSI256,
		ref:     "\x1b[36mTIX-1\x1b[0m",
		hex:     "\x1b[38;5;202mx\x1b[0m",
	},
	{
		name:    "a sixteen colour client",
		environ: []string{"TERM=xterm"},
		profile: termenv.ANSI,
		ref:     "\x1b[36mTIX-1\x1b[0m",
		hex:     "\x1b[91mx\x1b[0m",
	},
	{
		name:    "a client whose terminal has no colour at all",
		environ: []string{"TERM=vt100"},
		profile: termenv.Ascii,
		ref:     "TIX-1",
		hex:     "x",
	},
}

func TestNewRendererReadsTheClientsOwnTerminal(t *testing.T) {
	for _, tc := range sessions {
		t.Run(tc.name, func(t *testing.T) {
			r := NewRenderer(tc.environ, io.Discard)
			if got := r.ColorProfile(); got != tc.profile {
				t.Fatalf("profile for %q = %v, want %v", tc.environ, got, tc.profile)
			}
		})
	}
}

// TestSessionsHeldAtOnceRenderAtTheirOwnDepth is the whole point of the
// per-session renderer: lipgloss resolves a depth once per renderer and caches
// it, so the failure this guards against only appears while several clients
// are held at the same time. Every session therefore builds its renderer and
// renders inside its own goroutine, all of them released together and looping
// so the construction of one overlaps the rendering of another. A single
// shared renderer fails this: every session would carry the depth of whichever
// was measured first.
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
				r := NewRenderer(tc.environ, io.Discard)
				theme := NewTheme(r, true, core.Theme{})
				if got := theme.Ref.Render("TIX-1"); got != tc.ref {
					t.Errorf("%s rendered the reference as %q, want %q", tc.name, got, tc.ref)
					return
				}
				hex := r.NewStyle().Foreground(lipgloss.Color("#ff5f00")).Render("x")
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

// TestThemeIgnoresTheDefaultRendererWhenGivenOne fixes the regression the
// per-session renderer exists to prevent: one session's terminal reaching
// another's. The default renderer is left flattened for the length of the
// test, and a themed session must render in colour regardless.
func TestThemeIgnoresTheDefaultRendererWhenGivenOne(t *testing.T) {
	previous := lipgloss.DefaultRenderer().ColorProfile()
	lipgloss.SetColorProfile(termenv.Ascii)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })

	if got := NewTheme(nil, true, core.Theme{}).Ref.Render("TIX-1"); strings.Contains(got, "\x1b[") {
		t.Fatalf("the default renderer was flattened yet rendered %q", got)
	}
	theme := NewTheme(NewRenderer([]string{"TERM=xterm-256color"}, io.Discard), true, core.Theme{})
	if got := theme.Ref.Render("TIX-1"); got != "\x1b[36mTIX-1\x1b[0m" {
		t.Fatalf("a session with its own renderer rendered %q, want the reference in colour", got)
	}
	if got := theme.Style().Foreground(colorRef).Render("TIX-1"); got != "\x1b[36mTIX-1\x1b[0m" {
		t.Fatalf("a style built from the theme rendered %q, want the reference in colour", got)
	}
}
