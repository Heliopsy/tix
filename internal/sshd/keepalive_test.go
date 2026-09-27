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
	// fromListener counts the reads taken off the listener's socket. A frozen
	// relay still drains it, so this keeps counting once the client is gone
	// and is how a test waits for one keepalive to have been sent rather than
	// guessing at how long one takes.
	fromListener atomic.Int64
}

// sent reports how many times the listener has written to the relay.
func (r *relay) sent() int64 { return r.fromListener.Load() }

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
		if n > 0 && fromListener {
			r.fromListener.Add(1)
		}
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

// newPacedServer returns a serving listener whose every deadline is under the
// test's control, so a boundary can be hit exactly rather than waited out.
func newPacedServer(t *testing.T, with ...func(*Options)) (*Server, *clock.Fake) {
	t.Helper()
	p, clk, st := newProvisioner(t)
	o := Options{
		Service:            p.service,
		Store:              st,
		Clock:              clk,
		Demo:               true,
		Addr:               "127.0.0.1:0",
		HostKeyPath:        filepath.Join(t.TempDir(), "host_key"),
		KeepaliveInterval:  time.Minute,
		KeepaliveMaxMissed: 2,
		IdleTimeout:        time.Hour,
		ReapInterval:       time.Hour,
		Logger:             slog.New(slog.DiscardHandler),
	}
	for _, apply := range with {
		apply(&o)
	}
	srv, err := New(o)
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
	return srv, clk
}

// waitForWatchdog blocks until the session's two tickers are registered with
// the fake clock.
//
// Advancing before they exist moves the deadline instead of reaching it: the
// tickers are created from whatever the clock says at the time, while the
// activity tap was touched earlier, so the interval under test would no longer
// be the interval configured. The clock is the authority on this because it is
// the thing the watchdog waits on.
func waitForWatchdog(t *testing.T, clk *clock.Fake, want int) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for clk.Tickers() < want {
		if time.Now().After(deadline) {
			t.Fatalf("the session registered %d tickers after 30s, want %d", clk.Tickers(), want)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// TestASessionIdleForExactlyTheTimeoutIsClosed pins the idle decision at its
// boundary. A session that has gone the whole timeout without a keystroke is
// over; the comparison deciding that is the difference between a timeout of
// thirty minutes and a timeout of thirty minutes and one tick.
func TestASessionIdleForExactlyTheTimeoutIsClosed(t *testing.T) {
	const idle = 20 * time.Minute
	srv, clk := newPacedServer(t, func(o *Options) {
		o.IdleTimeout = idle
		// The watchdog checks idleness once per min(keepalive, idle), so
		// matching them puts a check at exactly the timeout and nowhere in
		// between.
		o.KeepaliveInterval = idle
	})
	before := clk.Tickers()
	session := dialCollected(t, srv.Addr())
	waitForLiveSessions(t, srv, 1)
	waitForWatchdog(t, clk, before+2)

	select {
	case <-session.ended:
		t.Fatal("the session ended before the clock moved at all")
	case <-time.After(250 * time.Millisecond):
	}

	clk.Advance(idle)
	select {
	case <-session.ended:
	case <-time.After(30 * time.Second):
		t.Fatalf("a session idle for exactly the %v timeout is still open", idle)
	}
}

// TestADeadClientIsDroppedOnTheMissedKeepaliveThatReachesTheLimit pins the
// other boundary: how many unanswered requests a client is allowed before its
// slot is taken back.
//
// Each round advances the clock by one interval and waits for the request that
// interval produced, so the count is of keepalives actually sent rather than
// of time elapsed. Two go unanswered; the third tick is the one that has to
// close the connection.
func TestADeadClientIsDroppedOnTheMissedKeepaliveThatReachesTheLimit(t *testing.T) {
	const interval = time.Minute
	const missed = 2
	srv, clk := newPacedServer(t, func(o *Options) {
		o.KeepaliveInterval = interval
		o.KeepaliveMaxMissed = missed
		o.IdleTimeout = time.Hour
	})
	before := clk.Tickers()
	r := newRelay(t, srv.Addr())
	client := dialRelay(t, r.addr())
	defer func() { _ = client.Close() }()
	waitForLiveSessions(t, srv, 1)
	waitForWatchdog(t, clk, before+2)

	r.freeze()
	for i := 1; i <= missed; i++ {
		sent := r.sent()
		clk.Advance(interval)
		waitForSend(t, r, sent)
		select {
		case <-r.dropped:
			t.Fatalf("the listener gave up after %d unanswered keepalives, want %d", i, missed)
		default:
		}
	}

	clk.Advance(interval)
	select {
	case <-r.dropped:
	case <-time.After(30 * time.Second):
		t.Fatalf("a client that left %d keepalives unanswered still holds its slot", missed)
	}
}

// waitForSend blocks until the listener has written again.
func waitForSend(t *testing.T, r *relay, since int64) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for r.sent() <= since {
		if time.Now().After(deadline) {
			t.Fatal("no keepalive reached the frozen client within 30s")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// collectedSession is a running session with its output kept and a channel
// that closes when the listener ends it.
type collectedSession struct {
	stdout *syncBuffer
	stdin  io.WriteCloser
	ended  chan struct{}
}

// quit presses the interface's quit key until the session is over, which is
// how a client leaves without the listener ending it. The interface may still
// be starting up when the first press lands, so it is repeated.
func (c *collectedSession) quit(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		select {
		case <-c.ended:
			return
		default:
		}
		if _, err := c.stdin.Write([]byte("q")); err != nil {
			<-c.ended
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the session ignored 30s of quit presses; it printed %q", c.stdout.String())
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// dialCollected opens a session with a fresh key, keeps everything it prints
// and reports when it is over.
func dialCollected(t *testing.T, addr string) *collectedSession {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	signer, err := gossh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	return dialCollectedAs(t, addr, "visitor", signer)
}

// dialCollectedAs is dialCollected under a named user and a given key, which
// is what a hosted listener needs.
func dialCollectedAs(t *testing.T, addr, user string, signer gossh.Signer) *collectedSession {
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

	collected := &collectedSession{stdout: &syncBuffer{}, ended: make(chan struct{})}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout: %v", err)
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		t.Fatalf("stderr: %v", err)
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		t.Fatalf("stdin: %v", err)
	}
	collected.stdin = stdin
	if err := sess.RequestPty("xterm-256color", 40, 120, gossh.TerminalModes{}); err != nil {
		t.Fatalf("pty: %v", err)
	}
	if err := sess.Shell(); err != nil {
		t.Fatalf("shell: %v", err)
	}
	go func() { _, _ = io.Copy(io.Discard, stderr) }()
	go func() {
		defer close(collected.ended)
		_, _ = io.Copy(collected.stdout, stdout)
	}()
	return collected
}
