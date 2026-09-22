package server_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/server"
	"github.com/heliopsy/tix/internal/sshd"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/wire"
)

// freePort binds and releases a port, returning an address nothing is
// listening on. Reserving one this way is what lets a test say "no listener
// bound this" rather than only "no error was reported".
func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a port: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("releasing the reserved port: %v", err)
	}
	return addr
}

// nothingListensOn asserts the address is free, by taking it.
func nothingListensOn(t *testing.T, addr string) {
	t.Helper()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("something is listening on %s: %v", addr, err)
	}
	_ = ln.Close()
}

// sshHandshake reports the identification string the listener answers with,
// which is the cheapest proof that it is the SSH server and not a socket that
// merely accepted.
func sshHandshake(t *testing.T, addr string) string {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatalf("dialling the ssh listener: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("setting a deadline: %v", err)
	}
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("reading the ssh banner: %v", err)
	}
	return strings.TrimSpace(string(buf[:n]))
}

// withSSHListener assembles a serve process carrying a real SSH listener on
// addr, wired exactly as the command wires it.
func withSSHListener(t *testing.T, a *assembled, addr string) server.SSHListener {
	t.Helper()
	listener, err := sshd.New(sshd.Options{
		Service:     a.conn.Service,
		Store:       a.store,
		Clock:       clock.New(),
		Addr:        addr,
		HostKeyPath: filepath.Join(t.TempDir(), "ssh_host_ed25519_key"),
	})
	if err != nil {
		t.Fatalf("building the ssh listener: %v", err)
	}
	return listener
}

func TestBothListenersServeOneDatabase(t *testing.T) {
	sshAddr := freePort(t)
	var ssh server.SSHListener
	a := assemble(t, func(o *server.Options) {
		o.Addr = "127.0.0.1:0"
	})
	ssh = withSSHListener(t, a, sshAddr)

	// Assembling twice would open a second database, so the listener is
	// attached to a server built over the same connection instead.
	srv := reassemble(t, a, func(o *server.Options) { o.SSH = ssh })
	if err := srv.Listen(); err != nil {
		t.Fatalf("listening: %v", err)
	}
	if got := srv.SSHAddr(); got != sshAddr {
		t.Fatalf("ssh addr = %q, want %q", got, sshAddr)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()

	if banner := sshHandshake(t, srv.SSHAddr()); !strings.HasPrefix(banner, "SSH-2.0-") {
		t.Errorf("ssh banner = %q, want an SSH-2.0 identification", banner)
	}

	base := "http://" + srv.Addr()
	resp, err := http.Get(base + wire.RouteHealth)
	if err != nil {
		t.Fatalf("requesting health: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("health status = %d, want 200", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// One process, one database: a task created through the very service
	// handle the SSH listener's sessions call is readable over HTTP.
	made, err := a.conn.Service.CreateTask(adminContext(a), core.CreateTaskInput{
		Title: "made over ssh",
	})
	if err != nil {
		t.Fatalf("creating a task through the ssh service handle: %v", err)
	}
	if body := getTasks(t, base, a); !strings.Contains(body, made.ID) {
		t.Errorf("task list over http did not carry %q made over the other surface: %s",
			made.ID, body)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serving: %v", err)
	}
	nothingListensOn(t, sshAddr)
}

func TestNoSSHListenerWithoutAnAddress(t *testing.T) {
	// The address the command would have used had one been given. Nothing may
	// bind it, before, during or after.
	candidate := freePort(t)

	a := assemble(t, nil)
	nothingListensOn(t, candidate)

	if err := a.srv.Listen(); err != nil {
		t.Fatalf("listening: %v", err)
	}
	if got := a.srv.SSHAddr(); got != "" {
		t.Fatalf("ssh addr = %q, want no listener", got)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.srv.Serve(ctx) }()

	resp, err := http.Get("http://" + a.srv.Addr() + wire.RouteHealth)
	if err != nil {
		t.Fatalf("requesting health: %v", err)
	}
	_ = resp.Body.Close()

	nothingListensOn(t, candidate)
	if got := a.srv.SSHAddr(); got != "" {
		t.Errorf("ssh addr = %q while serving, want no listener", got)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serving: %v", err)
	}
}

func TestSSHPortConflictFailsBeforeHTTPAccepts(t *testing.T) {
	sshAddr := freePort(t)
	held, err := net.Listen("tcp", sshAddr)
	if err != nil {
		t.Fatalf("holding the ssh port: %v", err)
	}
	defer func() { _ = held.Close() }()

	httpAddr := freePort(t)
	a := assemble(t, nil)
	srv := reassemble(t, a, func(o *server.Options) {
		o.Addr = httpAddr
		o.SSH = withSSHListener(t, a, sshAddr)
	})

	if err := srv.Listen(); err == nil {
		t.Fatal("listening succeeded with the ssh port already taken")
	}
	if got := srv.Addr(); got != "" {
		t.Errorf("http addr = %q, want nothing bound", got)
	}
	// The HTTP port is still free, so the process failed at startup rather
	// than after it had begun answering requests.
	nothingListensOn(t, httpAddr)

	if _, err := http.Get("http://" + httpAddr + wire.RouteHealth); err == nil {
		t.Error("the http surface answered after a failed startup")
	}
}

func TestShutdownDrainsBothListeners(t *testing.T) {
	cases := []struct {
		name       string
		drainFor   time.Duration
		wantClosed bool
		wantNotice bool
	}{
		{name: "sessions finish inside the timeout", drainFor: 10 * time.Millisecond},
		{
			name:       "a session outlives the timeout",
			drainFor:   time.Hour,
			wantClosed: true,
			wantNotice: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubSSH{drainFor: tc.drainFor}
			a := assemble(t, nil)
			srv := reassemble(t, a, func(o *server.Options) {
				o.SSH = stub
				o.ShutdownTimeout = 200 * time.Millisecond
			})
			if err := srv.Listen(); err != nil {
				t.Fatalf("listening: %v", err)
			}
			if !stub.listened() {
				t.Fatal("the ssh listener was never bound")
			}

			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() { done <- srv.Serve(ctx) }()

			release := make(chan struct{})
			inFlight := blockingRequest(t, "http://"+srv.Addr()+wire.RouteHealth, release)

			cancel()
			close(release)
			if err := <-done; err != nil {
				t.Fatalf("serving: %v", err)
			}
			<-inFlight

			if !stub.drained() {
				t.Error("the ssh listener was never asked to drain")
			}
			if budget := stub.budget(); budget < 100*time.Millisecond || budget > time.Second {
				t.Errorf("drain budget = %v, want about the shutdown timeout", budget)
			}
			if got := stub.closed(); got != tc.wantClosed {
				t.Errorf("closed = %v, want %v", got, tc.wantClosed)
			}
			if got := stub.noticed(); got != tc.wantNotice {
				t.Errorf("session notice = %v, want %v", got, tc.wantNotice)
			}
			if !stub.stopped() {
				t.Error("the ssh worker never returned, so shutdown did not end its lifetime")
			}
		})
	}
}

func TestShutdownClosesAListenerThatCannotDrain(t *testing.T) {
	stub := &plainSSH{}
	a := assemble(t, nil)
	srv := reassemble(t, a, func(o *server.Options) {
		o.SSH = stub
		o.ShutdownTimeout = 200 * time.Millisecond
	})
	if err := srv.Listen(); err != nil {
		t.Fatalf("listening: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serving: %v", err)
	}
	if !stub.closed() {
		t.Error("a listener that cannot drain was left open")
	}
}

// blockingRequest issues a request that the caller releases, so a shutdown
// runs with something genuinely in flight.
func blockingRequest(t *testing.T, url string, release <-chan struct{}) <-chan struct{} {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-release
		resp, err := http.Get(url)
		if err != nil {
			return
		}
		_ = resp.Body.Close()
	}()
	return done
}

// stubSSH stands in for the listener, because what shutdown owes a session is
// a budget and a notice, and both are observable here without a client.
type stubSSH struct {
	drainFor time.Duration

	mu       sync.Mutex
	bound    bool
	didDrain bool
	didClose bool
	notice   bool
	budgetd  time.Duration

	stop chan struct{}
	once sync.Once
}

func (s *stubSSH) Listen() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bound = true
	s.stop = make(chan struct{})
	return nil
}

func (s *stubSSH) Serve(ctx context.Context) error {
	<-ctx.Done()
	s.mu.Lock()
	stop := s.stop
	s.mu.Unlock()
	if stop != nil {
		s.once.Do(func() { close(stop) })
	}
	return nil
}

func (s *stubSSH) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.didClose = true
	return nil
}

func (s *stubSSH) Addr() string { return "127.0.0.1:0" }

// Drain waits for its sessions, telling the one still open at the deadline
// why it is ending.
func (s *stubSSH) Drain(ctx context.Context) error {
	deadline, ok := ctx.Deadline()
	s.mu.Lock()
	s.didDrain = true
	if ok {
		s.budgetd = time.Until(deadline)
	}
	wait := s.drainFor
	s.mu.Unlock()

	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		s.mu.Lock()
		s.notice = true
		s.mu.Unlock()
		return fmt.Errorf("closing sessions still open at shutdown: %w", ctx.Err())
	}
}

func (s *stubSSH) listened() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.bound }
func (s *stubSSH) drained() bool  { s.mu.Lock(); defer s.mu.Unlock(); return s.didDrain }
func (s *stubSSH) closed() bool   { s.mu.Lock(); defer s.mu.Unlock(); return s.didClose }
func (s *stubSSH) noticed() bool  { s.mu.Lock(); defer s.mu.Unlock(); return s.notice }

func (s *stubSSH) budget() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.budgetd
}

func (s *stubSSH) stopped() bool {
	s.mu.Lock()
	stop := s.stop
	s.mu.Unlock()
	if stop == nil {
		return false
	}
	select {
	case <-stop:
		return true
	case <-time.After(time.Second):
		return false
	}
}

// plainSSH is a listener with no drain, which shutdown must close rather than
// wait on. It deliberately does not embed stubSSH: embedding would inherit a
// Drain and make it the thing it exists to not be.
type plainSSH struct {
	mu       sync.Mutex
	didClose bool
	stop     chan struct{}
}

func (p *plainSSH) Listen() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stop = make(chan struct{})
	return nil
}

func (p *plainSSH) Serve(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (p *plainSSH) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.didClose = true
	return nil
}

func (p *plainSSH) Addr() string { return "127.0.0.1:0" }

func (p *plainSSH) closed() bool { p.mu.Lock(); defer p.mu.Unlock(); return p.didClose }

// reassemble builds a second server over the same connection, so a test can
// vary the server's options without opening another database.
func reassemble(t *testing.T, a *assembled, adjust func(*server.Options)) *server.Server {
	t.Helper()
	opts := server.Options{
		Service:       a.conn.Service,
		Store:         a.store,
		TenantID:      a.conn.Info.TenantID,
		Addr:          "127.0.0.1:0",
		PruneInterval: time.Hour,
	}
	if adjust != nil {
		adjust(&opts)
	}
	srv, err := server.Assemble(opts)
	if err != nil {
		t.Fatalf("assembling: %v", err)
	}
	return srv
}

// adminContext carries the connection's own actor, which is the identity a
// session on the SSH listener would hold.
func adminContext(a *assembled) context.Context {
	return core.WithSource(
		core.WithActor(context.Background(), a.conn.Actor),
		core.SourceTUI)
}

// getTasks reads the task list over the HTTP API with a minted token, and
// returns the body rather than the status, so a test asserts what came back.
func getTasks(t *testing.T, base string, a *assembled) string {
	t.Helper()
	ctx := context.Background()
	scope := core.TenantScope{TenantID: a.conn.Info.TenantID}
	clk := clock.New()

	minted, err := auth.MintAPIToken(clk, scope.TenantID, core.CreateTokenInput{
		Name: "over-http", ActorID: a.conn.Actor.ID, Scopes: []core.Scope{core.ScopeTaskRead},
	})
	if err != nil {
		t.Fatalf("minting token: %v", err)
	}
	issued := minted.Issued.APIToken
	if err := a.store.Update(ctx, scope, func(tx store.Tx) error {
		return tx.CreateToken(ctx, &issued, minted.Hash)
	}); err != nil {
		t.Fatalf("storing token: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+wire.RouteTasks, nil)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	req.Header.Set(wire.HeaderAuth, "Bearer "+minted.Issued.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("listing tasks: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("task list status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the task list: %v", err)
	}
	return string(body)
}
