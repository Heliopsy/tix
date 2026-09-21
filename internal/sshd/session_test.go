package sshd

import "testing"

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
