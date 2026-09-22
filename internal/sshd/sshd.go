// Package sshd serves the terminal interface over SSH, giving every key
// fingerprint its own ephemeral tenant.
package sshd

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/server"
	"github.com/heliopsy/tix/internal/store"
	"github.com/muesli/termenv"
)

// DefaultAddr is the loopback address and port the listener takes by default.
// Port 22 is a deployment concern: put a redirector in front, rather than
// giving this process the privilege to bind it.
const DefaultAddr = "127.0.0.1:2222"

// Defaults for the limits that keep a public listener from becoming an
// availability problem.
const (
	DefaultTenantTTL    = 6 * time.Hour
	DefaultReapInterval = 10 * time.Minute
	DefaultMaxTenants   = 200
	DefaultMaxTasks     = 200
	DefaultLeaseTTL     = 2 * time.Minute
	DefaultRatePerHour  = 60
	DefaultRateBurst    = 5
	DefaultIdleTimeout  = 30 * time.Minute
)

// Keepalive defaults. An idle timeout closes a session nobody is typing at; it
// says nothing about a session whose client has gone, which holds its slot and
// any lease it was carrying until the idle timeout finally expires. Half a
// minute between requests and three unanswered ones notice that in about two
// minutes, which is the length of a seeded lease rather than the length of the
// idle timeout: on a demo whose argument is that a lease returns work when a
// worker dies, a zombie session sitting on a claim is the wrong demonstration.
const (
	DefaultKeepaliveInterval  = 30 * time.Second
	DefaultKeepaliveMaxMissed = 3
)

// Defaults for the caps on live sessions. The rate limiter counts connections
// per hour from one address; neither it nor the tenant cap stops one key from
// holding many sessions at once, each a program with its own subscription.
const (
	DefaultMaxSessionsPerKey = 3
	DefaultMaxSessions       = 100
)

// Options configure the SSH listener.
type Options struct {
	// Service is the tenant-agnostic service every session calls through. It
	// takes its tenant from the actor in each call's context, never from
	// shared state, which is what lets one instance serve many tenants.
	Service core.Service
	// Store provisions and reaps the ephemeral tenants, which is work no
	// tenant-scoped service method can do.
	Store store.Store
	Clock clock.Clock

	Addr        string
	HostKeyPath string
	// AllowInsecure permits binding a non-loopback address, which is the same
	// explicit choice tix serve demands before it faces a network.
	AllowInsecure bool

	// TenantTTL is how long a sandbox survives without a visit. It slides on
	// every connection, so a returning visitor keeps their board.
	TenantTTL    time.Duration
	ReapInterval time.Duration
	// MaxTenants caps live sandboxes. Reaching it refuses a new fingerprint;
	// it never evicts an existing sandbox to make room.
	MaxTenants int
	MaxTasks   int
	LeaseTTL   time.Duration

	RatePerHour int
	RateBurst   int
	// IdleTimeout closes a session nobody is typing at. It is measured from
	// the last key the interface saw, never from traffic, so the keepalive
	// below cannot hold an abandoned session open forever.
	IdleTimeout time.Duration
	// KeepaliveInterval is the gap between liveness requests, and
	// KeepaliveMaxMissed how many may go unanswered before the connection is
	// dropped.
	KeepaliveInterval  time.Duration
	KeepaliveMaxMissed int

	// MaxSessionsPerKey caps the sessions one key holds at once, MaxSessions
	// the listener as a whole.
	MaxSessionsPerKey int
	MaxSessions       int

	TimeStyle output.TimeStyle
	Logger    *slog.Logger
}

// Server is the SSH listener and the background reaper behind it.
type Server struct {
	opts     Options
	ssh      *ssh.Server
	listener net.Listener
	limiter  *limiter
	live     *gate
	verifier *auth.PublicKeyVerifier
	log      *slog.Logger
}

// New validates the options, loads or creates the host key and returns the
// server. It binds nothing; call Listen for that.
func New(o Options) (*Server, error) {
	if o.Service == nil {
		return nil, core.Invalid("the ssh listener needs a service")
	}
	if o.Store == nil {
		return nil, core.Invalid("the ssh listener needs a store")
	}
	o = o.withDefaults()
	if err := server.CheckBindSafety(o.Addr, false, o.AllowInsecure); err != nil {
		return nil, err
	}
	signer, err := hostKey(o.HostKeyPath)
	if err != nil {
		return nil, err
	}

	// lipgloss resolves its colour profile once, from this process's own
	// standard output. A server's output is a log file or a journal, so the
	// profile lands on Ascii and every style the interface builds is stripped
	// before it can reach a session. Pinning it here is what lets a session be
	// drawn in colour at all. The interface's palette is entirely ANSI-16, so
	// that is the depth chosen: everything it uses, and nothing a colour
	// terminal might not understand. Whether a particular session is drawn in
	// colour remains a per-session decision, made by colorFor from what that
	// client said.
	lipgloss.SetColorProfile(termenv.ANSI)

	s := &Server{
		opts:    o,
		log:     o.Logger,
		limiter: newLimiter(o.Clock, rateInterval(o.RatePerHour), o.RateBurst, o.MaxTenants*4),
		live:    newGate(o.MaxSessionsPerKey, o.MaxSessions),
	}
	s.verifier = auth.NewPublicKeyVerifier(&provisioner{
		store:      o.Store,
		service:    o.Service,
		clk:        o.Clock,
		maxTenants: o.MaxTenants,
		leaseTTL:   o.LeaseTTL,
		tenantTTL:  o.TenantTTL,
	})
	s.ssh = &ssh.Server{
		Addr:        o.Addr,
		Handler:     s.handle,
		HostSigners: []ssh.Signer{signer},
		// This deadline bounds a connection that has not opened a session
		// yet, where there is no interface to ask about idleness. Once a
		// session is running it is refreshed by every byte either side
		// sends, keepalives included, so the session's own idleness is
		// decided by the watchdog in keepalive.go instead.
		IdleTimeout: o.IdleTimeout,
		// Every key is accepted. SSH requires a client to prove a key, but
		// nothing requires the server to have seen it before, and that proof
		// is the whole identity here.
		PublicKeyHandler: func(ctx ssh.Context, _ ssh.PublicKey) bool {
			return s.limiter.allow(sourceOf(ctx.RemoteAddr()))
		},
	}
	return s, nil
}

// withDefaults fills every unset limit with its default.
func (o Options) withDefaults() Options {
	if o.Clock == nil {
		o.Clock = clock.New()
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Addr == "" {
		o.Addr = DefaultAddr
	}
	if o.TenantTTL <= 0 {
		o.TenantTTL = DefaultTenantTTL
	}
	if o.ReapInterval <= 0 {
		o.ReapInterval = DefaultReapInterval
	}
	if o.MaxTenants <= 0 {
		o.MaxTenants = DefaultMaxTenants
	}
	if o.MaxTasks <= 0 {
		o.MaxTasks = DefaultMaxTasks
	}
	if o.LeaseTTL <= 0 {
		o.LeaseTTL = DefaultLeaseTTL
	}
	if o.RatePerHour <= 0 {
		o.RatePerHour = DefaultRatePerHour
	}
	if o.RateBurst <= 0 {
		o.RateBurst = DefaultRateBurst
	}
	if o.IdleTimeout <= 0 {
		o.IdleTimeout = DefaultIdleTimeout
	}
	if o.KeepaliveInterval <= 0 {
		o.KeepaliveInterval = DefaultKeepaliveInterval
	}
	if o.KeepaliveMaxMissed <= 0 {
		o.KeepaliveMaxMissed = DefaultKeepaliveMaxMissed
	}
	if o.MaxSessions <= 0 {
		o.MaxSessions = DefaultMaxSessions
	}
	if o.MaxSessionsPerKey <= 0 {
		o.MaxSessionsPerKey = DefaultMaxSessionsPerKey
	}
	if o.MaxSessionsPerKey > o.MaxSessions {
		o.MaxSessionsPerKey = o.MaxSessions
	}
	return o
}

// rateInterval converts a per-hour allowance into the gap between refills.
func rateInterval(perHour int) time.Duration {
	if perHour <= 0 {
		return time.Hour
	}
	return time.Hour / time.Duration(perHour)
}

// Listen binds the configured address so it is reachable before Serve runs.
func (s *Server) Listen() error {
	if s.listener != nil {
		return nil
	}
	ln, err := net.Listen("tcp", s.opts.Addr)
	if err != nil {
		return core.Internal("listening on %q: %v", s.opts.Addr, err)
	}
	s.listener = ln
	return nil
}

// Addr reports the bound address, which is empty before Listen.
func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// Serve accepts connections and runs the reaper until ctx is cancelled.
func (s *Server) Serve(ctx context.Context) error {
	if err := s.Listen(); err != nil {
		return err
	}
	go s.reap(ctx)

	done := make(chan error, 1)
	go func() { done <- s.ssh.Serve(s.listener) }()

	select {
	case err := <-done:
		if errors.Is(err, ssh.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		_ = s.ssh.Close()
		<-done
		return nil
	}
}

// Close stops the listener.
func (s *Server) Close() error { return s.ssh.Close() }

// sourceOf reduces a remote address to the host the limit counts against, so
// a client reconnecting from a fresh ephemeral port is still the same source.
func sourceOf(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}
