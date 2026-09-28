// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// registerServers writes n rows a minute apart and returns their identifiers in
// the order the listing should produce them.
func registerServers(t *testing.T, s *Store, base time.Time, n int) []string {
	t.Helper()
	ctx := context.Background()
	ids := make([]string, 0, n)
	for i := range n {
		srv := core.Server{
			ID:         fmt.Sprintf("srv%023d", i),
			Address:    fmt.Sprintf("10.0.0.%d:8080", i+1),
			Version:    "9.9.9",
			Surfaces:   []core.ServerSurface{core.ServerSurfaceAPI, core.ServerSurfaceWeb},
			StartedAt:  base.Add(time.Duration(i) * time.Minute),
			LastSeenAt: base.Add(time.Duration(i) * time.Minute),
		}
		if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
			return u.RegisterServer(ctx, &srv)
		}); err != nil {
			t.Fatalf("registering %q: %v", srv.ID, err)
		}
		ids = append(ids, srv.ID)
	}
	return ids
}

func listServers(t *testing.T, s *Store, page core.Page) []core.Server {
	t.Helper()
	ctx := context.Background()
	var out []core.Server
	if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
		var err error
		out, err = u.ListServers(ctx, page)
		return err
	}); err != nil {
		t.Fatalf("listing servers: %v", err)
	}
	return out
}

func TestServerRoundTrips(t *testing.T) {
	s, clk := newStore(t)
	base := clk.Now()
	registerServers(t, s, base, 1)

	got := listServers(t, s, core.Page{Limit: 10})
	if len(got) != 1 {
		t.Fatalf("listed %d servers, want 1", len(got))
	}
	srv := got[0]
	if srv.Address != "10.0.0.1:8080" || srv.Version != "9.9.9" {
		t.Errorf("read back %+v", srv)
	}
	if len(srv.Surfaces) != 2 ||
		srv.Surfaces[0] != core.ServerSurfaceAPI || srv.Surfaces[1] != core.ServerSurfaceWeb {
		t.Errorf("surfaces = %v, want the two that were written in that order", srv.Surfaces)
	}
	if !srv.StartedAt.Equal(base) || !srv.LastSeenAt.Equal(base) {
		t.Errorf("instants = %v, %v, want %v", srv.StartedAt, srv.LastSeenAt, base)
	}
}

// TestServerListingPagesByKeyset walks the listing a page at a time and checks
// that nothing repeats and nothing is skipped, which is the property OFFSET
// would not have under a concurrent write.
func TestServerListingPagesByKeyset(t *testing.T) {
	s, clk := newStore(t)
	want := registerServers(t, s, clk.Now(), 7)

	var got []string
	page := core.Page{Limit: 3, Sort: "started_at"}
	for range 10 {
		batch := listServers(t, s, page)
		for _, srv := range batch {
			got = append(got, srv.ID)
		}
		if len(batch) < page.Limit {
			break
		}
		last := batch[len(batch)-1]
		page.Cursor = core.Cursor{
			SortValue: sqlb.TimeText(last.StartedAt),
			ID:        last.ID,
			Sort:      "started_at",
		}.Encode()
	}

	if len(got) != len(want) {
		t.Fatalf("paged %d servers, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("page order = %v, want %v", got, want)
		}
	}
}

func TestServerHeartbeatAndRemoval(t *testing.T) {
	s, clk := newStore(t)
	ctx := context.Background()
	ids := registerServers(t, s, clk.Now(), 2)
	later := clk.Now().Add(time.Hour)

	if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
		return u.HeartbeatServer(ctx, ids[0], later)
	}); err != nil {
		t.Fatalf("heartbeating: %v", err)
	}
	for _, srv := range listServers(t, s, core.Page{Limit: 10}) {
		if srv.ID == ids[0] && !srv.LastSeenAt.Equal(later) {
			t.Errorf("last seen = %v, want %v", srv.LastSeenAt, later)
		}
		if srv.ID == ids[1] && srv.LastSeenAt.Equal(later) {
			t.Error("a heartbeat for one server moved another's")
		}
	}

	if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
		return u.DeregisterServer(ctx, ids[1])
	}); err != nil {
		t.Fatalf("deregistering: %v", err)
	}
	got := listServers(t, s, core.Page{Limit: 10})
	if len(got) != 1 || got[0].ID != ids[0] {
		t.Fatalf("after one removal the survivors are %v", got)
	}
}

// TestRegisteringTwiceReplacesTheRow: a process that registers again has
// restarted, so its start instant is the new one rather than the old.
func TestRegisteringTwiceReplacesTheRow(t *testing.T) {
	s, clk := newStore(t)
	ctx := context.Background()
	registerServers(t, s, clk.Now(), 1)

	restarted := core.Server{
		ID:         "srv00000000000000000000000",
		Address:    "10.0.0.99:8080",
		Version:    "9.9.9",
		StartedAt:  clk.Now().Add(time.Hour),
		LastSeenAt: clk.Now().Add(time.Hour),
	}
	if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
		return u.RegisterServer(ctx, &restarted)
	}); err != nil {
		t.Fatalf("re-registering: %v", err)
	}

	got := listServers(t, s, core.Page{Limit: 10})
	if len(got) != 1 {
		t.Fatalf("re-registering left %d rows, want 1", len(got))
	}
	if got[0].Address != "10.0.0.99:8080" {
		t.Errorf("address = %q, want the new one", got[0].Address)
	}
}

func TestForgetServersBefore(t *testing.T) {
	s, clk := newStore(t)
	ctx := context.Background()
	base := clk.Now()
	ids := registerServers(t, s, base, 3)

	var removed int
	if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
		var err error
		removed, err = u.ForgetServersBefore(ctx, base.Add(90*time.Second))
		return err
	}); err != nil {
		t.Fatalf("forgetting: %v", err)
	}
	if removed != 2 {
		t.Errorf("forgot %d rows, want 2", removed)
	}
	got := listServers(t, s, core.Page{Limit: 10})
	if len(got) != 1 || got[0].ID != ids[2] {
		t.Fatalf("survivors = %v, want only %q", got, ids[2])
	}
}
