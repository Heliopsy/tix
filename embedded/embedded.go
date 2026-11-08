// SPDX-License-Identifier: AGPL-3.0-or-later

// Package embedded runs tix inside the calling process: the HTTP API, the
// browser interface and the background workers tix serve runs, on a loopback
// port, over a SQLite database file. An app that cannot start a process, on
// iOS or Android, talks to it over the ordinary HTTP API.
package embedded

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/server"
	"github.com/heliopsy/tix/internal/serving"
)

// DefaultListen is the address an instance binds when Options.Listen is empty:
// loopback, on a port the kernel picks.
const DefaultListen = "127.0.0.1:0"

// DefaultTokenTTL is how long a token lives when TokenRequest.TTL is zero, the
// default the CLI and the browser form offer.
const DefaultTokenTTL = 90 * 24 * time.Hour

// Options configure an instance.
type Options struct {
	// DBPath is the SQLite database file. Required.
	DBPath string
	// Listen is a loopback IP literal and port; empty means DefaultListen.
	Listen string
	// InitIfMissing creates and seeds the database when DBPath does not exist,
	// as tix init does. Without it a missing file is a not-found error.
	InitIfMissing bool
	// Logger receives the server's log; nil discards it.
	Logger *slog.Logger
}

// Tenant is a tenant of the embedded database.
type Tenant struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// TokenRequest describes the API token Token mints.
type TokenRequest struct {
	// Tenant is the tenant key; empty means the default tenant.
	Tenant string
	// Name labels the token, unique among the tenant's live tokens. Required.
	Name string
	// Scopes are the permissions granted; empty grants every scope.
	Scopes []string
	// TTL is the token's lifetime: zero means DefaultTokenTTL, negative means
	// the token never expires.
	TTL time.Duration
}

// Instance is a running embedded tix.
type Instance struct {
	conn *connect.Conn
	srv  *server.Server
	url  string

	cancel context.CancelFunc
	served chan error

	stopOnce sync.Once
	stopErr  error
}

// Start opens the database, starts the HTTP API and the background workers on
// a loopback address and returns once that address accepts connections. ctx
// bounds the start only; the instance runs until Stop.
func Start(ctx context.Context, opts Options) (*Instance, error) {
	listen, err := loopbackListen(opts.Listen)
	if err != nil {
		return nil, err
	}
	path := strings.TrimSpace(opts.DBPath)
	if path == "" {
		return nil, core.Invalid("a database path is required")
	}
	if err := checkExists(path, opts.InitIfMissing); err != nil {
		return nil, err
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	cfg := config.Defaulted()
	conn, err := connect.Dial(ctx, cfg, connect.Overrides{
		DB:        path,
		DrainMode: connect.DrainModeServer,
		Serving:   true,
	})
	if err != nil {
		return nil, err
	}
	params := serving.Defaults()
	params.Listen = listen
	params.Logger = logger
	srv, err := serving.Build(conn, cfg, params)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := srv.Listen(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	runCtx, cancel := context.WithCancel(context.Background())
	in := &Instance{conn: conn, srv: srv, url: "http://" + srv.Addr(), cancel: cancel, served: make(chan error, 1)}
	go func() { in.served <- srv.Serve(runCtx) }()
	return in, nil
}

// loopbackListen returns the address to bind, refusing anything but a
// loopback IP literal. A hostname is refused too: localhost may resolve
// anywhere a hosts file says.
func loopbackListen(listen string) (string, error) {
	listen = strings.TrimSpace(listen)
	if listen == "" {
		return DefaultListen, nil
	}
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return "", core.Invalid("listen address %q: %v", listen, err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return "", core.Invalid("listen address %q is not a loopback ip; an embedded tix serves this device alone", listen)
	}
	return listen, nil
}

// checkExists refuses a missing database unless creating one was asked for.
func checkExists(path string, create bool) error {
	_, err := os.Stat(path)
	switch {
	case err == nil, create && errors.Is(err, fs.ErrNotExist):
		return nil
	case errors.Is(err, fs.ErrNotExist):
		return core.NotFound("database %q does not exist; start with init_if_missing to create it", path)
	default:
		return core.Internal("reading database %q", path).Wrap(err)
	}
}

// URL is the base URL of the HTTP API, such as http://127.0.0.1:41234.
func (in *Instance) URL() string { return in.url }

// Stop stops accepting connections, drains in-flight requests, stops the
// workers and closes the database. It returns when that is done or ctx ends,
// and does nothing on a stopped instance.
func (in *Instance) Stop(ctx context.Context) error {
	in.stopOnce.Do(func() {
		in.cancel()
		select {
		case err := <-in.served:
			in.stopErr = err
		case <-ctx.Done():
			_ = in.srv.Close()
			<-in.served //nolint:errcheck // the deadline is the error worth reporting
			in.stopErr = ctx.Err()
		}
		if err := in.conn.Close(); err != nil && in.stopErr == nil {
			in.stopErr = err
		}
	})
	return in.stopErr
}

// Tenants lists the live tenants of the database.
func (in *Instance) Tenants(ctx context.Context) ([]Tenant, error) {
	ctx = in.conn.Context(ctx)
	var out []Tenant
	page := core.Page{Limit: 500}
	for {
		tenants, next, err := in.conn.Service.ListTenants(ctx, page)
		if err != nil {
			return nil, err
		}
		for _, t := range tenants {
			out = append(out, Tenant{Key: t.Key, Name: t.Name})
		}
		if next == "" {
			return out, nil
		}
		page.Cursor = next
	}
}

// CreateTenant creates a tenant with no members, as tix tenant create does.
func (in *Instance) CreateTenant(ctx context.Context, key, name string) (Tenant, error) {
	input := core.CreateTenantInput{Key: key, Name: name}
	if err := input.Validate(); err != nil {
		return Tenant{}, err
	}
	created, err := in.conn.Service.CreateTenant(in.conn.Context(ctx), input)
	if err != nil {
		return Tenant{}, err
	}
	return Tenant{Key: created.Key, Name: created.Name}, nil
}

// Token mints an API token for the local actor of the requested tenant, as
// tix token create --db does, and returns its secret.
func (in *Instance) Token(ctx context.Context, req TokenRequest) (string, error) {
	scopes := make([]core.Scope, 0, len(req.Scopes))
	for _, s := range req.Scopes {
		scopes = append(scopes, core.Scope(s))
	}
	if len(scopes) == 0 {
		scopes = []core.Scope{core.ScopeAll}
	}
	input := core.CreateTokenInput{Name: req.Name, Scopes: scopes}
	switch ttl := req.TTL; {
	case ttl == 0:
		input.ExpiresAt = expiry(DefaultTokenTTL)
	case ttl > 0:
		input.ExpiresAt = expiry(ttl)
	}
	if err := input.Validate(); err != nil {
		return "", err
	}
	owner, err := in.conn.InTenant(ctx, req.Tenant)
	if err != nil {
		return "", err
	}
	issued, err := owner.Service.CreateToken(owner.Context(ctx), input)
	if err != nil {
		return "", err
	}
	return issued.Token, nil
}

func expiry(ttl time.Duration) *time.Time {
	at := time.Now().Add(ttl).UTC()
	return &at
}

// ErrorCode names the kind of an error this package returned, as the HTTP API
// reports it: invalid, not_found, conflict, forbidden, internal and so on.
func ErrorCode(err error) string {
	if err == nil {
		return ""
	}
	return string(core.KindOf(err))
}

// ErrorMessage is the error's text without the kind ErrorCode already names.
func ErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimPrefix(err.Error(), ErrorCode(err)+": ")
}
