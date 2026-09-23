// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

func TestGateCapsSessionsPerKeyAndOverall(t *testing.T) {
	tests := []struct {
		name      string
		maxPerKey int
		max       int
		take      []string
		refused   string
	}{
		{
			name: "one key at its own cap", maxPerKey: 2, max: 10,
			take: []string{"a", "a", "a"}, refused: "limit of 2 concurrent sessions",
		},
		{
			name: "the listener at its cap", maxPerKey: 5, max: 2,
			take: []string{"a", "b", "c"}, refused: "limit of 2 sessions",
		},
		{
			name: "under both caps", maxPerKey: 2, max: 10,
			take: []string{"a", "b", "a", "b"},
		},
		{
			name: "a per-key cap above the listener's is the listener's", maxPerKey: 9, max: 2,
			take: []string{"a", "a", "a"}, refused: "limit of 2 sessions",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := newGate(tc.maxPerKey, tc.max)
			var last error
			for _, fingerprint := range tc.take {
				last = g.acquire(fingerprint)
			}
			if tc.refused == "" {
				if last != nil {
					t.Fatalf("acquire = %v, want every session admitted", last)
				}
				return
			}
			if last == nil {
				t.Fatal("the cap admitted one session too many")
			}
			if !strings.Contains(last.Error(), tc.refused) {
				t.Fatalf("refusal = %q, want it to name the limit %q", last, tc.refused)
			}
		})
	}
}

func TestGateRefusesAKnownAndAnUnknownKeyIdentically(t *testing.T) {
	g := newGate(4, 1)
	if err := g.acquire("known"); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	same, err := g.acquire("known"), g.acquire("stranger")
	if same == nil || err == nil {
		t.Fatal("a full listener admitted another session")
	}
	if same.Error() != err.Error() {
		t.Fatalf("refusals differ: %q and %q, which tells a stranger the key was seen", same, err)
	}
}

func TestGateReleasesSlots(t *testing.T) {
	g := newGate(1, 2)
	if err := g.acquire("a"); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if err := g.acquire("a"); err == nil {
		t.Fatal("the per-key cap admitted a second session")
	}
	g.release("a")
	if err := g.acquire("a"); err != nil {
		t.Fatalf("a released slot was not given back: %v", err)
	}
	g.release("a")
	if perKey, live := g.counts("a"); perKey != 0 || live != 0 {
		t.Fatalf("counts = %d/%d, want the key forgotten once it holds nothing", perKey, live)
	}
}

func TestGateIsSafeUnderConcurrency(t *testing.T) {
	const max = 8
	g := newGate(max, max)
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := g.acquire("a"); err == nil {
				g.release("a")
			}
		}()
	}
	wg.Wait()
	if perKey, live := g.counts("a"); perKey != 0 || live != 0 {
		t.Fatalf("counts = %d/%d after every session ended, want 0/0", perKey, live)
	}
}

// TestOneKeyCannotHoldMoreSessionsThanTheCap opens sessions over one
// connection under one key, past the cap, and reads the refusal the way a
// client would: on the session's standard error, before any board is drawn.
func TestOneKeyCannotHoldMoreSessionsThanTheCap(t *testing.T) {
	srv, _ := newLiveServer(t, func(o *Options) {
		o.MaxSessionsPerKey = 1
		o.MaxSessions = 4
		o.KeepaliveInterval = time.Hour
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := srv.Listen(); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	go func() { _ = srv.Serve(ctx) }()

	client := dialRelay(t, srv.Addr())
	defer func() { _ = client.Close() }()

	sess, err := client.NewSession()
	if err != nil {
		t.Fatalf("second session: %v", err)
	}
	defer func() { _ = sess.Close() }()
	var stderr bytes.Buffer
	sess.Stderr = &stderr
	if err := sess.RequestPty("xterm-256color", 40, 120, gossh.TerminalModes{}); err != nil {
		t.Fatalf("pty: %v", err)
	}
	if err := sess.Run(""); err == nil {
		t.Fatal("the second session under one key was served, past the cap of 1")
	}
	if got := stderr.String(); !strings.Contains(got, "limit of 1 concurrent sessions") {
		t.Fatalf("stderr = %q, want a refusal naming the limit", got)
	}
}
