// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"strings"
	"testing"
	"time"
)

func TestColorForReadsWhatTheClientSaid(t *testing.T) {
	tests := []struct {
		name    string
		environ []string
		want    bool
	}{
		{"a colour terminal", []string{"TERM=xterm-256color"}, true},
		{"a plain terminal", []string{"TERM=vt100"}, true},
		{"a dumb terminal", []string{"TERM=dumb"}, false},
		{"no terminal named", []string{"LANG=C"}, false},
		{"an empty terminal type", []string{"TERM="}, false},
		{"NO_COLOR", []string{"TERM=xterm-256color", "NO_COLOR=1"}, false},
		{"TIX_NO_COLOR", []string{"TERM=xterm-256color", "TIX_NO_COLOR=1"}, false},
		{"an empty NO_COLOR is not a choice", []string{"TERM=xterm-256color", "NO_COLOR="}, true},
		{
			name: "the pseudo-terminal's type wins, because the session appends it last",
			// A client can set TERM with an env request; the pty-req is the
			// one that describes the terminal actually allocated.
			environ: []string{"TERM=dumb", "TERM=xterm-256color"},
			want:    true,
		},
		{"nothing at all", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := colorFor(tc.environ); got != tc.want {
				t.Fatalf("colorFor(%q) = %v, want %v", tc.environ, got, tc.want)
			}
		})
	}
}

func TestASessionLandsWhereItsModeHasABoard(t *testing.T) {
	tests := []struct {
		name string
		demo bool
		want string
	}{
		{"a sandbox opens its seeded board", true, seedProjectKey},
		{"a hosted session opens the project list", false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{opts: Options{Demo: tc.demo}}
			if got := s.openProject(); got != tc.want {
				t.Fatalf("openProject() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestOnlyASandboxSaysAnythingOnTheWayOut guards the claim rather than the
// wording: a hosted session must not tell somebody their real board was a
// sandbox that is about to be deleted.
func TestOnlyASandboxSaysAnythingOnTheWayOut(t *testing.T) {
	sandbox := (&Server{opts: Options{Demo: true, TenantTTL: 6 * time.Hour}}).notice()
	for _, want := range []string{"demo sandbox", "6h", "\r\n"} {
		if !strings.Contains(sandbox, want) {
			t.Errorf("a sandbox's notice %q does not mention %q", sandbox, want)
		}
	}
	hosted := (&Server{opts: Options{Demo: false, TenantTTL: 6 * time.Hour}}).notice()
	if hosted != "" {
		t.Fatalf("a hosted session left %q behind, want nothing", hosted)
	}
}

// TestASandboxSessionIsHandedItsNoticeOnTheWayOut takes the farewell the whole
// way: notice() producing a line is not the same as the line reaching a
// client, and the step between them is the one that decides whether a session
// carries a notice at all.
//
// The assertion is on the client's own output after the interface has given
// the terminal back, which is where a person would read it.
func TestASandboxSessionIsHandedItsNoticeOnTheWayOut(t *testing.T) {
	srv, _ := newPacedServer(t, func(o *Options) { o.TenantTTL = 6 * time.Hour })
	session := dialCollected(t, srv.Addr())
	waitForLiveSessions(t, srv, 1)
	session.quit(t)
	<-session.ended

	got := session.stdout.String()
	for _, want := range []string{"demo sandbox", "6h"} {
		if !strings.Contains(got, want) {
			t.Errorf("the sandbox session ended without mentioning %q; it printed %q", want, got)
		}
	}
}

// TestAHostedSessionIsHandedNoNotice is the other half. A notice withheld in
// the wrong direction tells somebody their real board is a sandbox on a timer,
// which is both false and alarming.
func TestAHostedSessionIsHandedNoNotice(t *testing.T) {
	addr, st := newEnrolledListener(t)
	signer, fingerprint := newSigner(t)
	enrolSigner(t, st, "acme", "alice", fingerprint)

	session := dialCollectedAs(t, addr, "acme", signer)
	session.quit(t)
	<-session.ended

	for _, unwanted := range []string{"demo sandbox", "deleted after"} {
		if strings.Contains(session.stdout.String(), unwanted) {
			t.Errorf("a hosted session was told %q on the way out", unwanted)
		}
	}
}
