// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
)

// refreshRecorder is a service with one due SAML provider in every tenant,
// which reports who refreshed it.
type refreshRecorder struct {
	core.Service
	refreshed chan *core.Actor
}

func (r refreshRecorder) ListSSOProviders(context.Context) ([]core.SSOProvider, error) {
	return []core.SSOProvider{{ID: "p1", Key: "okta", Kind: core.SSOKindSAML, Enabled: true,
		IdPMetadataURL: "https://idp.example/md"}}, nil
}

func (r refreshRecorder) RefreshSAMLMetadata(ctx context.Context, _ string) (*core.SSOProvider, error) {
	actor, _ := core.ActorFrom(ctx)
	r.refreshed <- actor
	return &core.SSOProvider{Key: "okta", MetadataRefresh: &core.SAMLMetadataRefresh{Result: core.SAMLRefreshUpdated}}, nil
}

func workerNamed(opts Options, name string) Worker {
	for _, w := range workersFor(opts) {
		if w.Name() == name {
			return w
		}
	}
	return nil
}

// The refresher runs only with an interval, visits every live tenant from
// the store, and refreshes as each tenant's system actor.
func TestTheSAMLRefresherVisitsEveryTenantAsTheSystemActor(t *testing.T) {
	conn, err := connect.Dial(context.Background(), &config.Resolved{},
		connect.Overrides{DB: filepath.Join(t.TempDir(), "tix.db")})
	if err != nil {
		t.Fatalf("dialling a scratch database: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	svc := refreshRecorder{refreshed: make(chan *core.Actor, 4)}
	opts := Options{Service: svc, Store: conn.Store, Clock: clock.NewFakeAt(), Logger: slog.Default(),
		DisableSweep: true, DisableDispatch: true}
	if workerNamed(opts, "saml-metadata-refresher") != nil {
		t.Fatal("a refresher runs with auth.saml_metadata_refresh at 0")
	}
	opts.SAMLRefreshInterval = 12 * time.Hour
	w := workerNamed(opts, "saml-metadata-refresher")
	if w == nil {
		t.Fatal("no refresher with an interval")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = w.Run(ctx) }()
	actor := <-svc.refreshed
	if actor == nil || actor.Kind != core.ActorSystem || actor.TenantID != conn.Info.TenantID {
		t.Fatalf("refreshed as %+v, want the system actor of tenant %s", actor, conn.Info.TenantID)
	}
}
