// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/tui"
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

// TestTheTerminalTypeReachesTheInterface guards the one fact colour over SSH
// rests on: the terminal type a client asked for in its pty-req is in the
// environment the interface reads. The ssh package does not put it there, so
// nothing else would notice it going missing, and a session with no TERM is
// drawn without colour at all.
func TestTheTerminalTypeReachesTheInterface(t *testing.T) {
	tests := []struct {
		name    string
		environ []string
		term    string
		pty     bool
		want    []string
	}{
		{
			name: "a pseudo-terminal's type is appended",
			term: "xterm-256color", pty: true,
			want: []string{"TERM=xterm-256color"},
		},
		{
			name:    "and it wins over one the client set with an env request",
			environ: []string{"TERM=dumb", "LANG=C"},
			term:    "xterm-256color", pty: true,
			want: []string{"TERM=dumb", "LANG=C", "TERM=xterm-256color"},
		},
		{
			name:    "a session with no pseudo-terminal names no terminal",
			environ: []string{"LANG=C"},
			term:    "xterm-256color", pty: false,
			want: []string{"LANG=C"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := withTerm(tc.environ, tc.term, tc.pty)
			if len(got) != len(tc.want) {
				t.Fatalf("withTerm(%q, %q, %v) = %q, want %q", tc.environ, tc.term, tc.pty, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("withTerm(%q, %q, %v) = %q, want %q", tc.environ, tc.term, tc.pty, got, tc.want)
				}
			}
			if !tc.pty {
				return
			}
			// The environment is read last wins, so the appended type is the
			// one the colour decision and the profile both see.
			if !colorFor(got) {
				t.Fatalf("a %s session was drawn without colour from %q", tc.term, got)
			}
		})
	}
}

func TestASessionLandsWhereItsModeHasABoard(t *testing.T) {
	tests := []struct {
		name  string
		demo  bool
		want  string
		start bool
	}{
		{"a sandbox opens its seeded board", true, seedProjectKey, false},
		{"a hosted session opens the start menu", false, "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{opts: Options{Demo: tc.demo}}
			got, err := s.route(nil)
			if err != nil {
				t.Fatalf("route(nil) = %v", err)
			}
			if got.iface != "board" || got.project != tc.want || got.start != tc.start {
				t.Fatalf("route(nil) = %+v, want the board interface on %q with start %v", got, tc.want, tc.start)
			}
		})
	}
}

// TestASessionCanNameTheBoardItWants covers the argument that used to be read
// off the wire and dropped. A client naming a board got the project list and
// no word about why, and over ssh there is no flag to pass instead, so this
// was the only way to say it and it did nothing.
func TestASessionCanNameTheBoardItWants(t *testing.T) {
	for _, demo := range []bool{false, true} {
		for _, command := range [][]string{{"infra"}, {"board", "infra"}, {"BOARD", "infra"}} {
			s := &Server{opts: Options{Demo: demo}}
			got, err := s.route(command)
			if err != nil {
				t.Fatalf("demo=%v: route(%q) = %v", demo, command, err)
			}
			if got.iface != "board" || got.project != "infra" || got.start {
				t.Errorf("demo=%v: route(%q) = %+v, want the infra board", demo, command, got)
			}
		}
	}
}

// TestARegisteredNameIsReservedAndBoardReachesTheProject is the name clash:
// `watch` is a project key anybody may create, and the bare word is the stream
// on every tenant, so `board watch` is how that project opens.
func TestARegisteredNameIsReservedAndBoardReachesTheProject(t *testing.T) {
	s := &Server{opts: Options{}}
	for _, word := range []string{"watch", "WATCH", "Watch"} {
		got, err := s.route([]string{word})
		if err != nil {
			t.Fatalf("route([%s]) = %v", word, err)
		}
		if got.iface != "watch" || got.project != "" {
			t.Errorf("route([%s]) = %+v, want the watch stream and no board", word, got)
		}
	}
	got, err := s.route([]string{"board", "watch"})
	if err != nil {
		t.Fatalf("route([board watch]) = %v", err)
	}
	if got.iface != "board" || got.project != "watch" {
		t.Fatalf("route([board watch]) = %+v, want the board of the project keyed watch", got)
	}
	got, err = s.route([]string{"board"})
	if err != nil || got.iface != "board" || got.project != "" || got.start {
		t.Fatalf("route([board]) = %+v, %v, want the project list", got, err)
	}
}

// TestWatchReadsTheFlagsTixWatchTakes keeps the SSH spelling of watch the
// same as the command line's, plus project keys as words.
func TestWatchReadsTheFlagsTixWatchTakes(t *testing.T) {
	s := &Server{opts: Options{}}
	got, err := s.route([]string{"watch", "infra", "--project", "ops,web", "--type", "task.*",
		"--actor", "ada", "--actor", "bob", "--since", "42", "--limit=3", "-o", "ndjson", "house"})
	if err != nil {
		t.Fatalf("route = %v", err)
	}
	w := got.watch
	want := watchRequest{
		projects: []string{"infra", "ops", "web", "house"}, types: []string{"task.*"},
		actors: []string{"ada", "bob"}, since: 42, limit: 3, ndjson: true,
	}
	if !slices.Equal(w.projects, want.projects) || !slices.Equal(w.types, want.types) ||
		!slices.Equal(w.actors, want.actors) || w.since != want.since || w.limit != want.limit || w.ndjson != want.ndjson {
		t.Fatalf("watch read %+v, want %+v", w, want)
	}
}

// TestNamingSomethingThatIsNotABoardIsRefused keeps two different wrong things
// apart. A key that is merely unknown belongs to the interface, which answers
// it against the projects this actor can reach; a key that could not name a
// board at all, or a command line that is reaching for a shell, is refused
// before a session is built, naming every command that does exist.
func TestNamingSomethingThatIsNotABoardIsRefused(t *testing.T) {
	tests := []struct {
		name    string
		command []string
	}{
		{"an empty argument", []string{""}},
		{"a path", []string{"../../etc/passwd"}},
		{"a control character", []string{"infra\r\n"}},
		{"longer than a key may be", []string{strings.Repeat("a", 65)}},
		{"reaching for a shell", []string{"ls", "-la"}},
		{"a board with two keys", []string{"board", "a", "b"}},
		{"a board that is not a key", []string{"board", "../x"}},
		{"watch with an unknown flag", []string{"watch", "--follow"}},
		{"watch with a negative cursor", []string{"watch", "--since", "-1"}},
		{"watch with a format it cannot stream", []string{"watch", "-o", "yaml"}},
		{"watch with a project that is not a key", []string{"watch", "../x"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{opts: Options{}}
			got, err := s.route(tc.command)
			if err == nil {
				t.Fatalf("route(%q) = %+v, want an error", tc.command, got)
			}
			if got.project != "" {
				t.Errorf("route(%q) = %+v alongside its error, want no board", tc.command, got)
			}
		})
	}
}

// TestTheRefusalNamesEveryInterface is the registry's third reader: a person
// told no is told what yes looks like.
func TestTheRefusalNamesEveryInterface(t *testing.T) {
	_, err := (&Server{}).route([]string{"ls", "-la"})
	if err == nil {
		t.Fatal("a shell command was accepted")
	}
	for _, i := range tui.Interfaces() {
		if !strings.Contains(err.Error(), "`"+i.Usage+"`") {
			t.Errorf("the refusal %q does not name %q", err, i.Usage)
		}
	}
}

// TestEveryInterfaceHasALauncher is the registry's listener half: the menu
// half is in internal/tui.
func TestEveryInterfaceHasALauncher(t *testing.T) {
	registered := map[string]bool{}
	for _, i := range tui.Interfaces() {
		registered[i.Name] = true
		if launchers[i.Name] == nil {
			t.Errorf("interface %q is registered and the listener has no launcher for it", i.Name)
		}
		got, err := (&Server{}).route([]string{i.Name})
		if err != nil || got.iface != i.Name {
			t.Errorf("route([%s]) = %+v, %v, want that interface", i.Name, got, err)
		}
	}
	for name := range launchers {
		if !registered[name] {
			t.Errorf("the listener has a launcher for %q, which no interface registers", name)
		}
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

func TestTheTransferCommandNamesOnlyAPlainUsername(t *testing.T) {
	cases := map[string]string{
		"acme":                  "acme",
		"tix":                   "tix",
		"":                      NeutralUser,
		"evil\x1b[2J":           NeutralUser,
		"a b":                   NeutralUser,
		strings.Repeat("a", 65): NeutralUser,
	}
	for in, want := range cases {
		if got := printableUser(in); got != want {
			t.Errorf("printableUser(%q) = %q, want %q", in, got, want)
		}
	}
}
