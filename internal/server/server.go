// Package server runs the tix HTTP surface and its background workers.
package server

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/thereisnotime/tix/internal/core"
)

// Defaults for the process lifecycle.
const (
	DefaultShutdownTimeout = 15 * time.Second
	DefaultReadHeaderWait  = 10 * time.Second
	DefaultAddr            = "127.0.0.1:8080"
)

// ErrShutdownTimeout reports that work was still in flight when the shutdown
// deadline elapsed.
var ErrShutdownTimeout = errors.New("shutdown timeout reached with requests still in flight")

// Config assembles a Server.
type Config struct {
	Addr    string
	Handler http.Handler
	Logger  *slog.Logger

	CertFile string
	KeyFile  string

	// AllowInsecure permits binding a non-loopback address without TLS. It is
	// the only way past the bind guard, and it is never implied.
	AllowInsecure bool

	ShutdownTimeout   time.Duration
	ReadHeaderTimeout time.Duration

	Workers []Worker
}

// Server owns the listener, the HTTP server and the background workers.
type Server struct {
	cfg      Config
	http     *http.Server
	listener net.Listener
	tls      *tls.Config
	mu       sync.Mutex
}

// New validates the configuration, loads any certificate and returns a server
// that has not yet bound its address.
func New(cfg Config) (*Server, error) {
	if cfg.Handler == nil {
		return nil, core.Invalid("a server requires a handler")
	}
	if strings.TrimSpace(cfg.Addr) == "" {
		cfg.Addr = DefaultAddr
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = DefaultShutdownTimeout
	}
	if cfg.ReadHeaderTimeout <= 0 {
		cfg.ReadHeaderTimeout = DefaultReadHeaderWait
	}

	tlsConfig, err := loadTLS(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, err
	}
	if err := CheckBindSafety(cfg.Addr, tlsConfig != nil, cfg.AllowInsecure); err != nil {
		return nil, err
	}
	if tlsConfig == nil && cfg.AllowInsecure && !IsLoopbackAddr(cfg.Addr) {
		cfg.Logger.Warn("serving plain http on a non-loopback address; traffic is unencrypted",
			"addr", cfg.Addr)
	}

	return &Server{
		cfg: cfg,
		tls: tlsConfig,
		http: &http.Server{
			Addr:              cfg.Addr,
			Handler:           cfg.Handler,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			TLSConfig:         tlsConfig,
		},
	}, nil
}

// loadTLS reads the certificate pair, reporting which file failed.
func loadTLS(certFile, keyFile string) (*tls.Config, error) {
	switch {
	case certFile == "" && keyFile == "":
		return nil, nil
	case certFile == "":
		return nil, core.Invalid("a tls key was configured without a certificate")
	case keyFile == "":
		return nil, core.Invalid("a tls certificate was configured without a key")
	}
	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, core.Invalid("loading tls certificate %q and key %q: %v", certFile, keyFile, err)
	}
	return &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12}, nil
}

// CheckBindSafety refuses a non-loopback bind that is neither protected by TLS
// nor explicitly opted out of.
func CheckBindSafety(addr string, hasTLS, allowInsecure bool) error {
	if IsLoopbackAddr(addr) || hasTLS || allowInsecure {
		return nil
	}
	return core.Invalid(
		"refusing to bind non-loopback address %q without tls: configure a certificate and key, or pass the explicit insecure opt-out",
		addr)
}

// IsLoopbackAddr reports whether addr binds only the loopback interface.
func IsLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

// Listen binds the configured address so it is reachable before Serve runs.
func (s *Server) Listen() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return nil
	}
	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return core.Internal("listening on %q: %v", s.cfg.Addr, err)
	}
	s.listener = ln
	return nil
}

// Addr reports the bound address, which is empty before Listen.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// TLSEnabled reports whether the server serves HTTPS.
func (s *Server) TLSEnabled() bool { return s.tls != nil }

// Serve accepts connections and runs the workers until ctx is cancelled, then
// drains in-flight requests within the shutdown timeout.
func (s *Server) Serve(ctx context.Context) error {
	if err := s.Listen(); err != nil {
		return err
	}
	workers := s.startWorkers(ctx)

	served := make(chan error, 1)
	go func() { served <- s.accept() }()

	select {
	case err := <-served:
		workers.Wait()
		return err
	case <-ctx.Done():
	}

	err := s.shutdown()
	workers.Wait()
	<-served
	return err
}

// accept serves until the listener closes.
func (s *Server) accept() error {
	s.mu.Lock()
	ln := s.listener
	s.mu.Unlock()

	var err error
	if s.tls != nil {
		err = s.http.ServeTLS(ln, "", "")
	} else {
		err = s.http.Serve(ln)
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// shutdown stops accepting and waits for in-flight requests.
func (s *Server) shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
	defer cancel()

	if err := s.http.Shutdown(ctx); err != nil {
		s.cfg.Logger.Error("shutdown did not drain in time",
			"timeout", s.cfg.ShutdownTimeout.String())
		_ = s.http.Close()
		return ErrShutdownTimeout
	}
	return nil
}

// Close stops the server immediately, dropping connections.
func (s *Server) Close() error { return s.http.Close() }
