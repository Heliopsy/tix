// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/store"
	gossh "golang.org/x/crypto/ssh"
)

// heldStore delays the first unscoped read after arm until the test releases
// it, which is how a key's resolution is held mid-flight without a sleep.
//
// The unscoped read is the enrolled lookup's first act, so a connection parked
// inside it has authenticated and reached the lookup and has not yet been
// answered. That is precisely the window in which a key nobody enrolled used
// to be holding a session slot.
type heldStore struct {
	store.Store

	mu      sync.Mutex
	armed   bool
	hold    chan struct{}
	entered chan struct{}
}

// arm makes the next unscoped read block, and returns the channel that reports
// it has arrived.
func (h *heldStore) arm() <-chan struct{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.armed = true
	h.hold = make(chan struct{})
	h.entered = make(chan struct{})
	return h.entered
}

// release lets the held read through and disarms, so every later read runs
// straight away.
func (h *heldStore) release() {
	h.mu.Lock()
	hold := h.hold
	h.armed = false
	h.hold = nil
	h.mu.Unlock()
	if hold != nil {
		close(hold)
	}
}

func (h *heldStore) Unscoped(ctx context.Context, fn func(store.UnscopedTx) error) error {
	h.mu.Lock()
	armed, hold, entered := h.armed, h.hold, h.entered
	if armed {
		h.armed = false
	}
	h.mu.Unlock()
	if armed {
		close(entered)
		select {
		case <-hold:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return h.Store.Unscoped(ctx, fn)
}

// TestAnUnenrolledKeyHoldsNoCapacityAnEnrolledKeyNeeds pins the ordering of the
// concurrency gate against a real listener whose only session slot is
// contended.
//
// A stranger is parked inside its own lookup, which is the longest an
// unenrolled connection can survive, and an enrolled key then asks for the
// slot. The enrolled session must be served: the stranger is entitled to
// nothing, so it may hold nothing.
func TestAnUnenrolledKeyHoldsNoCapacityAnEnrolledKeyNeeds(t *testing.T) {
	srv, held, st := newCappedEnrolledListener(t, 1)
	known, fingerprint := newSigner(t)
	enrolSigner(t, st, "acme", "alice", fingerprint)
	stranger, _ := newSigner(t)

	entered := held.arm()
	defer held.release()

	strangerSession := openHeldSession(t, srv.Addr(), "acme", stranger)
	select {
	case <-entered:
	case <-time.After(30 * time.Second):
		t.Fatal("the stranger never reached the lookup, so nothing was holding the listener")
	}

	enrolledSession := openHeldSession(t, srv.Addr(), "acme", known)
	deadline := time.Now().Add(30 * time.Second)
	for {
		if perKey, _ := srv.live.counts(fingerprint); perKey == 1 {
			break
		}
		if refusal := enrolledSession.stderr.String(); strings.Contains(refusal, "limit of") {
			t.Fatalf("the enrolled key was refused while a stranger held the listener: %q", refusal)
		}
		if time.Now().After(deadline) {
			t.Fatalf("the enrolled key never got the slot within 30s; its stderr said %q",
				enrolledSession.stderr.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
	if refusal := enrolledSession.stderr.String(); refusal != "" {
		t.Errorf("the enrolled session was told %q, want nothing", refusal)
	}

	// The stranger is genuinely a stranger, and genuinely got as far as the
	// lookup. Without this the assertion above would pass on a listener that
	// had refused the stranger at the handshake, or on a key that turned out
	// to be enrolled, neither of which exercises the ordering.
	held.release()
	strangerSession.waitFor(t, "not enrolled")
	if perKey, _ := srv.live.counts(fingerprintOf(stranger)); perKey != 0 {
		t.Errorf("the refused stranger holds %d slots, want none", perKey)
	}
}

// fingerprintOf renders a signer's key the way the listener records it.
func fingerprintOf(signer gossh.Signer) string {
	return gossh.FingerprintSHA256(signer.PublicKey())
}

// newCappedEnrolledListener binds a serving listener over a store whose reads
// the test can hold, admitting max sessions in total.
func newCappedEnrolledListener(t *testing.T, max int) (*Server, *heldStore, store.Store) {
	t.Helper()
	p, _, st := newProvisioner(t)
	held := &heldStore{Store: st}
	srv, err := New(Options{
		Service:      p.service,
		Store:        held,
		Clock:        clock.New(),
		Addr:         "127.0.0.1:0",
		HostKeyPath:  filepath.Join(t.TempDir(), "host_key"),
		IdleTimeout:  time.Hour,
		ReapInterval: time.Hour,
		MaxSessions:  max,
		Logger:       slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := srv.Listen(); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = srv.Serve(ctx); close(done) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return srv, held, st
}

// heldSession is one live session with its standard error collected.
type heldSession struct {
	stderr *syncBuffer
}

// openHeldSession authenticates, asks for a pseudo-terminal and starts the
// interface, leaving the session running.
func openHeldSession(t *testing.T, addr, user string, signer gossh.Signer) *heldSession {
	t.Helper()
	client, err := gossh.Dial("tcp", addr, &gossh.ClientConfig{
		User:            user,
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), // #nosec G106 -- the test generated this host key
		Timeout:         30 * time.Second,
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	sess, err := client.NewSession()
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	collected := &syncBuffer{}
	stderr, err := sess.StderrPipe()
	if err != nil {
		t.Fatalf("stderr: %v", err)
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout: %v", err)
	}
	if err := sess.RequestPty("xterm-256color", 40, 120, gossh.TerminalModes{}); err != nil {
		t.Fatalf("pty: %v", err)
	}
	if err := sess.Shell(); err != nil {
		t.Fatalf("shell: %v", err)
	}
	go func() { _, _ = io.Copy(io.Discard, stdout) }()
	go func() { _, _ = io.Copy(collected, stderr) }()
	return &heldSession{stderr: collected}
}

// waitFor blocks until the session's standard error carries want.
func (h *heldSession) waitFor(t *testing.T, want string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		if strings.Contains(h.stderr.String(), want) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%q never arrived on stderr; it said %q", want, h.stderr.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
}
