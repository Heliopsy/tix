// SPDX-License-Identifier: AGPL-3.0-or-later

package serving_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/server"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/serving"
	"github.com/heliopsy/tix/internal/wire"
)

// TestMain drops password hashing to test cost, as every package building a
// service does.
func TestMain(m *testing.M) {
	service.UseCheapHashingInTests()
	os.Exit(m.Run())
}

func TestBuildRefusesWithoutALocalConnection(t *testing.T) {
	for name, conn := range map[string]*connect.Conn{"nil": nil, "no store": {}} {
		if _, err := serving.Build(conn, config.Defaulted(), serving.Defaults()); !core.IsKind(err, core.KindInvalid) {
			t.Errorf("%s: got %v, want invalid", name, err)
		}
	}
}

func TestBuildServesTheAPIAndTheBrowser(t *testing.T) {
	ctx := context.Background()
	cfg := config.Defaulted()
	conn, err := connect.Dial(ctx, cfg, connect.Overrides{DB: filepath.Join(t.TempDir(), "tix.db"), Serving: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	p := serving.Defaults()
	p.Listen = "127.0.0.1:0"
	p.DisableRegistry = true
	srv, err := serving.Build(conn, cfg, p)
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Listen(); err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- srv.Serve(runCtx) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("serve: %v", err)
		}
	}()
	for _, route := range []string{wire.RouteHealth, "/login"} {
		resp, err := http.Get("http://" + srv.Addr() + route)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s answered %d", route, resp.StatusCode)
		}
	}
}

func TestDefaultsAreServesFlagDefaults(t *testing.T) {
	d := serving.Defaults()
	if d.Listen != server.DefaultAddr || d.SweepInterval != serving.DefaultSweepInterval ||
		d.PruneInterval != serving.DefaultPruneInterval || d.Heartbeat != core.ServerHeartbeatInterval {
		t.Fatalf("defaults %+v", d)
	}
}

func TestTimeStyleFollowsTheConfiguration(t *testing.T) {
	utc := config.Defaulted()
	utc.Config.Output.Timezone = "utc"
	if got := serving.TimeStyle(utc).Location(); got != time.UTC {
		t.Fatalf("a utc configuration gave %v", got)
	}
	bad := config.Defaulted()
	bad.Config.Output.Timezone = "Not/AZone"
	zero := output.TimeStyle{}
	for name, cfg := range map[string]*config.Resolved{"nil": nil, "unreadable zone": bad} {
		got := serving.TimeStyle(cfg)
		if got.Name() != zero.Name() || got.Location() != zero.Location() {
			t.Errorf("%s gave %s in %v, want the zero style", name, got.Name(), got.Location())
		}
	}
}
