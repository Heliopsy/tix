// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/store"
	gossh "golang.org/x/crypto/ssh"
)

// stub is a model that does nothing, so a test of the activity tap is a test
// of the tap and not of the interface.
type stub struct{}

func (stub) Init() tea.Cmd                       { return nil }
func (stub) Update(tea.Msg) (tea.Model, tea.Cmd) { return stub{}, nil }
func (stub) View() string                        { return "" }

// TestIdlenessIsMeasuredFromInputNotFromTraffic is the property that keeps the
// keepalive from defeating the idle timeout: a keepalive and its reply are
// traffic, and every message a session carries that is not a person typing
// leaves the idle clock where it was.
func TestIdlenessIsMeasuredFromInputNotFromTraffic(t *testing.T) {
	clk := clock.NewFakeAt()
	act := &activity{}
	act.touch(clk.Now())
	var model tea.Model = watched{Model: stub{}, act: act, clk: clk}

	quiet := []tea.Msg{
		tea.WindowSizeMsg{Width: 80, Height: 24},
		tea.QuitMsg{},
		struct{ name string }{"an event the board redrew for"},
	}
	for _, msg := range quiet {
		clk.Advance(time.Minute)
		model, _ = model.Update(msg)
		if got := act.idleFor(clk.Now()); got == 0 {
			t.Fatalf("%T reset the idle clock, so traffic alone keeps a session alive", msg)
		}
	}
	if got, want := act.idleFor(clk.Now()), 3*time.Minute; got != want {
		t.Fatalf("idle = %v, want %v after only traffic", got, want)
	}

	clk.Advance(time.Minute)
	if _, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}); act.idleFor(clk.Now()) != 0 {
		t.Fatalf("idle = %v after a keystroke, want it reset", act.idleFor(clk.Now()))
	}
}

// TestASessionWhoseClientVanishesIsDropped simulates a client that goes away
// without a clean disconnect: its packets stop arriving, but nothing closes
// the connection. That is a closed laptop or an expired NAT entry, and it is
// exactly what an idle timeout cannot see.
func TestASessionWhoseClientVanishesIsDropped(t *testing.T) {
	srv, _ := newLiveServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := srv.Listen(); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	go func() { _ = srv.Serve(ctx) }()

	r := newRelay(t, srv.Addr())
	client := dialRelay(t, r.addr())
	defer func() { _ = client.Close() }()

	select {
	case <-r.dropped:
		t.Fatal("the listener dropped a client that was still answering")
	case <-time.After(500 * time.Millisecond):
	}

	r.freeze()
	select {
	case <-r.dropped:
	case <-time.After(20 * time.Second):
		t.Fatal("a session whose client vanished was never dropped")
	}
}

// TestDefaultsMatchTheConfiguredOnes keeps the listener's own
// defaults and the configuration keys from drifting apart, which would make
// `tix ssh` behave differently depending on which layer supplied a value.
func TestDefaultsMatchTheConfiguredOnes(t *testing.T) {
	cfg := config.Defaults().SSH
	tests := []struct {
		key  string
		got  any
		want any
	}{
		{"ssh.listen", cfg.Listen, DefaultAddr},
		{"ssh.tenant_ttl", time.Duration(cfg.TenantTTL), DefaultTenantTTL},
		{"ssh.reap_interval", time.Duration(cfg.ReapInterval), DefaultReapInterval},
		{"ssh.max_tenants", cfg.MaxTenants, DefaultMaxTenants},
		{"ssh.max_tasks", cfg.MaxTasks, DefaultMaxTasks},
		{"ssh.lease_ttl", time.Duration(cfg.LeaseTTL), DefaultLeaseTTL},
		{"ssh.rate_per_hour", cfg.RatePerHour, DefaultRatePerHour},
		{"ssh.rate_burst", cfg.RateBurst, DefaultRateBurst},
		{"ssh.idle_timeout", time.Duration(cfg.IdleTimeout), DefaultIdleTimeout},
		{"ssh.keepalive_interval", time.Duration(cfg.KeepaliveInterval), DefaultKeepaliveInterval},
		{"ssh.keepalive_max_missed", cfg.KeepaliveMaxMissed, DefaultKeepaliveMaxMissed},
		{"ssh.max_sessions_per_key", cfg.MaxSessionsPerKey, DefaultMaxSessionsPerKey},
		{"ssh.max_sessions", cfg.MaxSessions, DefaultMaxSessions},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("%s defaults to %v, but the listener defaults to %v", tc.key, tc.got, tc.want)
		}
	}
}

// TestKeepaliveNoticesADeadClientLongBeforeTheIdleTimeout states the reason
// the two settings are separate, in the numbers themselves.
func TestKeepaliveNoticesADeadClientLongBeforeTheIdleTimeout(t *testing.T) {
	notice := DefaultKeepaliveInterval * time.Duration(DefaultKeepaliveMaxMissed+1)
	if notice > 3*time.Minute {
		t.Fatalf("a dead client is noticed after %v, which is too long to hold a slot", notice)
	}
	if notice >= DefaultLeaseTTL*2 {
		t.Errorf("a dead client outlives its lease by %v, which is the demo's worst story", notice)
	}
	if notice >= DefaultIdleTimeout {
		t.Errorf("the keepalive notices nothing the idle timeout would not have")
	}
}

// newLiveServer returns a listener on a real clock, with a keepalive short
// enough for a test to wait out.
func newLiveServer(t *testing.T, with ...func(*Options)) (*Server, store.Store) {
	t.Helper()
	p, _, st := newProvisioner(t)
	o := Options{
		Service:            p.service,
		Store:              st,
		Clock:              clock.New(),
		Demo:               true,
		Addr:               "127.0.0.1:0",
		HostKeyPath:        filepath.Join(t.TempDir(), "host_key"),
		KeepaliveInterval:  100 * time.Millisecond,
		KeepaliveMaxMissed: 2,
		IdleTimeout:        time.Hour,
		Logger:             slog.New(slog.DiscardHandler),
	}
	for _, apply := range with {
		apply(&o)
	}
	srv, err := New(o)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	return srv, st
}

// relay forwards a connection between a client and the listener until it is
// frozen, after which it keeps both sockets open and delivers nothing. That is
// a vanished client as the listener sees one: no close, no error, no packets.
type relay struct {
	ln      net.Listener
	target  string
	frozen  atomic.Bool
	dropped chan struct{}
	once    sync.Once
}

func newRelay(t *testing.T, target string) *relay {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("relay listen: %v", err)
	}
	r := &relay{ln: ln, target: target, dropped: make(chan struct{})}
	t.Cleanup(func() { _ = ln.Close() })
	go r.accept()
	return r
}

func (r *relay) addr() string { return r.ln.Addr().String() }

func (r *relay) freeze() { r.frozen.Store(true) }

func (r *relay) accept() {
	for {
		client, err := r.ln.Accept()
		if err != nil {
			return
		}
		upstream, err := net.Dial("tcp", r.target)
		if err != nil {
			_ = client.Close()
			return
		}
		go r.pump(client, upstream, true)
		go r.pump(upstream, client, false)
	}
}

// pump copies one direction, dropping everything once frozen. The direction
// coming from the listener reports the moment the listener gave up.
func (r *relay) pump(dst io.Writer, src net.Conn, fromListener bool) {
	buf := make([]byte, 4096)
	for {
		n, err := src.Read(buf)
		if n > 0 && !r.frozen.Load() {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			if fromListener {
				r.once.Do(func() { close(r.dropped) })
			}
			return
		}
	}
}

// dialRelay opens a session with a fresh key through the relay and leaves the
// interface running.
func dialRelay(t *testing.T, addr string) *gossh.Client {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	signer, err := gossh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	client, err := gossh.Dial("tcp", addr, &gossh.ClientConfig{
		User:            "visitor",
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), // #nosec G106 -- the test generated this host key
		Timeout:         10 * time.Second,
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	sess, err := client.NewSession()
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout: %v", err)
	}
	go func() { _, _ = io.Copy(io.Discard, stdout) }()
	if err := sess.RequestPty("xterm-256color", 40, 120, gossh.TerminalModes{}); err != nil {
		t.Fatalf("pty: %v", err)
	}
	if err := sess.Shell(); err != nil {
		t.Fatalf("shell: %v", err)
	}
	return client
}
