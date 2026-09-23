// SPDX-License-Identifier: AGPL-3.0-or-later

package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/server"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/sqlite"
	"github.com/heliopsy/tix/internal/wire"
)

// assembled is a serve process wired the way the command wires it.
type assembled struct {
	conn  *connect.Conn
	store *sqlite.Store
	srv   *server.Server
}

func assemble(t *testing.T, adjust func(*server.Options)) *assembled {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "tix.db")

	conn, err := connect.Dial(ctx, &config.Resolved{}, connect.Overrides{DB: path})
	if err != nil {
		t.Fatalf("dialling the local target: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	st, err := sqlite.Open(path, clock.New())
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	opts := server.Options{
		Service:       conn.Service,
		Store:         st,
		TenantID:      conn.Info.TenantID,
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
	return &assembled{conn: conn, store: st, srv: srv}
}

func TestAssembleRejectsIncompleteOptions(t *testing.T) {
	if _, err := server.Assemble(server.Options{}); err == nil {
		t.Error("assembling without a service was accepted")
	}
	a := assemble(t, nil)
	if _, err := server.Assemble(server.Options{Service: a.conn.Service}); err == nil {
		t.Error("assembling without a store was accepted")
	}
}

func TestAssembledServerServesHealthAndReadiness(t *testing.T) {
	a := assemble(t, nil)
	if err := a.srv.Listen(); err != nil {
		t.Fatalf("listening: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.srv.Serve(ctx) }()

	base := "http://" + a.srv.Addr()
	for _, path := range []string{wire.RouteHealth, wire.RouteReady} {
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatalf("requesting %s: %v", path, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s status = %d, want 200", path, resp.StatusCode)
		}
		_ = resp.Body.Close()
	}

	unauth, err := http.Get(base + wire.RouteTasks)
	if err != nil {
		t.Fatalf("requesting tasks: %v", err)
	}
	if unauth.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauthenticated status = %d, want 401", unauth.StatusCode)
	}
	_ = unauth.Body.Close()

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serving: %v", err)
	}
}

func TestAssembleHonoursDisabledWorkers(t *testing.T) {
	cases := []struct {
		name   string
		adjust func(*server.Options)
	}{
		{"both enabled", nil},
		{"sweeper disabled", func(o *server.Options) { o.DisableSweep = true }},
		{"pruner disabled", func(o *server.Options) { o.DisablePrune = true }},
		{"pruner without an interval", func(o *server.Options) { o.PruneInterval = 0 }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := assemble(t, tc.adjust)
			if err := a.srv.Listen(); err != nil {
				t.Fatalf("listening: %v", err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() { done <- a.srv.Serve(ctx) }()
			cancel()
			if err := <-done; err != nil {
				t.Fatalf("serving: %v", err)
			}
		})
	}
}

func TestStoreCredentialsResolveATokenWithinTheTenant(t *testing.T) {
	a := assemble(t, nil)
	ctx := context.Background()
	scope := core.TenantScope{TenantID: a.conn.Info.TenantID}
	clk := clock.New()

	minted, err := auth.MintAPIToken(clk, scope.TenantID, core.CreateTokenInput{
		Name: "agent", ActorID: a.conn.Actor.ID, Scopes: []core.Scope{core.ScopeTaskRead},
	})
	if err != nil {
		t.Fatalf("minting token: %v", err)
	}
	stored := minted.Issued.APIToken
	if err := a.store.Update(ctx, scope, func(tx store.Tx) error {
		return tx.CreateToken(ctx, &stored, minted.Hash)
	}); err != nil {
		t.Fatalf("storing token: %v", err)
	}

	authenticator := server.NewAuthenticator(a.store, clk)
	req := httptest.NewRequest(http.MethodGet, wire.RouteTasks, nil)
	req.Header.Set(wire.HeaderAuth, "Bearer "+minted.Issued.Token)

	scoped := core.WithTenant(ctx, scope)
	actor, err := authenticator.Authenticate(scoped, req)
	if err != nil {
		t.Fatalf("authenticating a valid token: %v", err)
	}
	if actor.TenantID != scope.TenantID {
		t.Errorf("tenant = %q, want %q", actor.TenantID, scope.TenantID)
	}

	if _, err := authenticator.Authenticate(ctx, req); err == nil {
		t.Error("a token resolved without a tenant in context")
	}

	other := core.WithTenant(ctx, core.TenantScope{TenantID: "01JJJJJJJJJJJJJJJJJJJJJJJJ"})
	if _, err := authenticator.Authenticate(other, req); err == nil {
		t.Error("a token resolved inside another tenant")
	}
}

func TestStoreCredentialsResolveASession(t *testing.T) {
	a := assemble(t, nil)
	ctx := context.Background()
	scope := core.TenantScope{TenantID: a.conn.Info.TenantID}
	clk := clock.New()

	minted, err := auth.MintSession(clk, auth.SessionInput{
		ActorID: a.conn.Actor.ID, TenantID: scope.TenantID, Handle: "local", TTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("minting session: %v", err)
	}
	if err := a.store.Update(ctx, scope, func(tx store.Tx) error {
		return tx.CreateSession(ctx, minted.Stored.ActorID, minted.Stored.TokenHash,
			minted.Stored.ExpiresAt)
	}); err != nil {
		t.Fatalf("storing session: %v", err)
	}

	authenticator := server.NewAuthenticator(a.store, clk)
	req := httptest.NewRequest(http.MethodGet, wire.RouteTasks, nil)
	req.AddCookie(&http.Cookie{Name: wire.SessionCookieName, Value: minted.Session.Token})

	actor, err := authenticator.Authenticate(core.WithTenant(ctx, scope), req)
	if err != nil {
		t.Fatalf("authenticating a live session: %v", err)
	}
	if actor.ID != a.conn.Actor.ID {
		t.Errorf("actor = %q, want %q", actor.ID, a.conn.Actor.ID)
	}

	unknown := httptest.NewRequest(http.MethodGet, wire.RouteTasks, nil)
	unknown.AddCookie(&http.Cookie{Name: wire.SessionCookieName, Value: "not-a-session"})
	if _, err := authenticator.Authenticate(core.WithTenant(ctx, scope), unknown); err == nil {
		t.Error("an unknown session token authenticated")
	}
}
