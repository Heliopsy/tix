// SPDX-License-Identifier: AGPL-3.0-or-later

package presence_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/presence"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/sqlite"
)

// fixture is a real temporary database, because the whole behaviour under test
// is what survives in a row and what does not.
type fixture struct {
	store *sqlite.Store
	clk   *clock.Fake
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	clk := clock.NewFakeAt()
	s, err := sqlite.Open(filepath.Join(t.TempDir(), "tix.db"), clk)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	return fixture{store: s, clk: clk}
}

// registrar builds one over the fixture, on the fixture's clock.
func (f fixture) registrar(addr string, surfaces ...core.ServerSurface) *presence.Registrar {
	return presence.New(presence.Config{
		Store:    f.store,
		Address:  func() string { return addr },
		Version:  "9.9.9",
		Surfaces: surfaces,
	}, presence.WithClock(f.clk))
}

// servers reads every registered row.
func (f fixture) servers(t *testing.T) []core.Server {
	t.Helper()
	var out []core.Server
	err := f.store.Unscoped(context.Background(), func(u store.UnscopedTx) error {
		var err error
		out, err = u.ListServers(context.Background(), core.Page{Limit: 100})
		return err
	})
	if err != nil {
		t.Fatalf("listing servers: %v", err)
	}
	return out
}

func TestRegisterWritesTheRow(t *testing.T) {
	f := newFixture(t)
	r := f.registrar("10.0.0.4:8080", core.ServerSurfaceAPI, core.ServerSurfaceSSH)

	if err := r.Register(context.Background()); err != nil {
		t.Fatalf("registering: %v", err)
	}

	got := f.servers(t)
	if len(got) != 1 {
		t.Fatalf("registered %d servers, want 1", len(got))
	}
	s := got[0]
	if s.ID != r.ServerID() {
		t.Errorf("row id = %q, registrar says %q", s.ID, r.ServerID())
	}
	if s.Address != "10.0.0.4:8080" {
		t.Errorf("address = %q", s.Address)
	}
	if s.Version != "9.9.9" {
		t.Errorf("version = %q", s.Version)
	}
	if len(s.Surfaces) != 2 || s.Surfaces[0] != core.ServerSurfaceAPI || s.Surfaces[1] != core.ServerSurfaceSSH {
		t.Errorf("surfaces = %v", s.Surfaces)
	}
	if !s.StartedAt.Equal(f.clk.Now()) || !s.LastSeenAt.Equal(f.clk.Now()) {
		t.Errorf("started %v, last seen %v, clock %v", s.StartedAt, s.LastSeenAt, f.clk.Now())
	}
}

func TestHeartbeatMovesOnlyTheLastSeenInstant(t *testing.T) {
	f := newFixture(t)
	r := f.registrar("127.0.0.1:1")
	ctx := context.Background()
	if err := r.Register(ctx); err != nil {
		t.Fatalf("registering: %v", err)
	}
	started := f.servers(t)[0].StartedAt

	f.clk.Advance(core.ServerHeartbeatInterval)
	if err := r.Beat(ctx); err != nil {
		t.Fatalf("beating: %v", err)
	}

	got := f.servers(t)[0]
	if !got.StartedAt.Equal(started) {
		t.Errorf("the start instant moved: %v then %v", started, got.StartedAt)
	}
	if !got.LastSeenAt.Equal(f.clk.Now()) {
		t.Errorf("last seen = %v, want %v", got.LastSeenAt, f.clk.Now())
	}
}

// TestStalenessNeedsNoSweeper is the point of the whole design. Nothing runs
// between the beat and the judgement: no sweeper, no pruner, no second
// registrar. The reader compares the row to its own clock and is right.
func TestStalenessNeedsNoSweeper(t *testing.T) {
	f := newFixture(t)
	r := f.registrar("127.0.0.1:1")
	if err := r.Register(context.Background()); err != nil {
		t.Fatalf("registering: %v", err)
	}
	row := f.servers(t)[0]

	tests := []struct {
		name     string
		after    time.Duration
		attached bool
	}{
		{"just registered", 0, true},
		{"one missed beat", core.ServerHeartbeatInterval, true},
		{"two missed beats", 2 * core.ServerHeartbeatInterval, true},
		{"exactly at the threshold", core.ServerStaleAfter, true},
		{"past the threshold", core.ServerStaleAfter + time.Second, false},
		{"long gone", 30 * 24 * time.Hour, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			at := row.LastSeenAt.Add(tt.after)
			if got := row.Attached(at); got != tt.attached {
				t.Errorf("Attached(%v after last seen) = %v, want %v", tt.after, got, tt.attached)
			}
		})
	}

	// And the row is still there to be judged, which is what a crash leaves.
	if len(f.servers(t)) != 1 {
		t.Fatal("the row vanished without anybody deleting it")
	}
}

// TestUptimeStopsWhenTheServerDid is a hand-verification finding turned into a
// guard. A dead server's uptime was measured to now, so the figure climbed for
// a process that had not run for a minute and a half.
func TestUptimeStopsWhenTheServerDid(t *testing.T) {
	started := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	srv := core.Server{StartedAt: started, LastSeenAt: started.Add(time.Hour)}

	tests := []struct {
		name string
		at   time.Time
		want time.Duration
	}{
		// While the reader still counts it attached, uptime runs to now: as far
		// as anybody knows the process is up and the beat is merely in flight.
		{"at the last beat", started.Add(time.Hour), time.Hour},
		{"one beat behind, still attached", started.Add(time.Hour + core.ServerHeartbeatInterval),
			time.Hour + core.ServerHeartbeatInterval},
		// Once it reads as gone, uptime stops at the last beat and stays there.
		{"just past the threshold", started.Add(time.Hour + core.ServerStaleAfter + time.Second), time.Hour},
		{"long dead", started.Add(72 * time.Hour), time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := time.Duration(srv.UptimeAt(tt.at)); got != tt.want {
				t.Errorf("UptimeAt = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeregisterRemovesTheRow(t *testing.T) {
	f := newFixture(t)
	r := f.registrar("127.0.0.1:1")
	ctx := context.Background()
	if err := r.Register(ctx); err != nil {
		t.Fatalf("registering: %v", err)
	}
	if err := r.Deregister(ctx); err != nil {
		t.Fatalf("deregistering: %v", err)
	}
	if got := f.servers(t); len(got) != 0 {
		t.Fatalf("a graceful shutdown left %d rows behind: %v", len(got), got)
	}
}

func TestTwoServersRegisterSeparately(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	one := f.registrar("10.0.0.4:8080")
	two := f.registrar("10.0.0.5:8080")
	for _, r := range []*presence.Registrar{one, two} {
		if err := r.Register(ctx); err != nil {
			t.Fatalf("registering: %v", err)
		}
	}
	if one.ServerID() == two.ServerID() {
		t.Fatal("two processes registered under one identifier")
	}

	got := f.servers(t)
	if len(got) != 2 {
		t.Fatalf("registered %d servers, want 2", len(got))
	}

	// One of them dies without a shutdown, which is a registrar that simply
	// stops beating.
	if err := one.Deregister(ctx); err != nil {
		t.Fatalf("stopping the first: %v", err)
	}
	if got := f.servers(t); len(got) != 1 || got[0].ID != two.ServerID() {
		t.Fatalf("after one graceful stop the survivors are %v", got)
	}
}

func TestRegisteringForgetsLongUnseenRows(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	dead := f.registrar("10.0.0.9:8080")
	if err := dead.Register(ctx); err != nil {
		t.Fatalf("registering: %v", err)
	}

	f.clk.Advance(core.ServerForgetAfter + time.Hour)
	live := f.registrar("10.0.0.4:8080")
	if err := live.Register(ctx); err != nil {
		t.Fatalf("registering: %v", err)
	}

	got := f.servers(t)
	if len(got) != 1 || got[0].ID != live.ServerID() {
		t.Fatalf("the long-dead row survived: %v", got)
	}
}

// TestAStillRecentRowIsNotForgotten holds the other side of the purge: a
// server that is merely stale is the thing an operator came to see, so it must
// survive somebody else registering.
func TestAStillRecentRowIsNotForgotten(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	stale := f.registrar("10.0.0.9:8080")
	if err := stale.Register(ctx); err != nil {
		t.Fatalf("registering: %v", err)
	}

	f.clk.Advance(core.ServerStaleAfter * 10)
	live := f.registrar("10.0.0.4:8080")
	if err := live.Register(ctx); err != nil {
		t.Fatalf("registering: %v", err)
	}

	got := f.servers(t)
	if len(got) != 2 {
		t.Fatalf("registered %d servers, want the stale one kept: %v", len(got), got)
	}
	now := f.clk.Now()
	for _, s := range got {
		switch s.ID {
		case stale.ServerID():
			if s.Attached(now) {
				t.Error("the stale server reads as attached")
			}
		case live.ServerID():
			if !s.Attached(now) {
				t.Error("the live server reads as stale")
			}
		}
	}
}

func TestRunRegistersBeatsAndRemoves(t *testing.T) {
	f := newFixture(t)
	r := f.registrar("127.0.0.1:1")
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()

	waitFor(t, func() bool { return len(f.servers(t)) == 1 })
	started := f.servers(t)[0].LastSeenAt

	// The ticker only exists once Run has built it, so the advance has to wait
	// for it rather than assume it.
	waitFor(t, func() bool { return f.clk.Tickers() > 0 })
	f.clk.Advance(core.ServerHeartbeatInterval)
	waitFor(t, func() bool { return f.servers(t)[0].LastSeenAt.After(started) })

	cancel()
	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() = %v", err)
	}
	if got := f.servers(t); len(got) != 0 {
		t.Fatalf("a stopped registrar left %d rows behind", len(got))
	}
}

// waitFor polls a condition another goroutine will satisfy. It is not a sleep
// standing in for coordination: the clock under test is fake, and what is
// being waited for is a goroutine reaching its next statement.
func waitFor(t *testing.T, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition was never met")
}

// busyOnce refuses the first unscoped write, which is what a database busy
// with another server's transaction looks like from in here. Two servers on
// one SQLite file meet exactly this on the way in.
type busyOnce struct {
	store.Store
	mu   sync.Mutex
	left int
}

func (s *busyOnce) Unscoped(ctx context.Context, fn func(store.UnscopedTx) error) error {
	s.mu.Lock()
	if s.left > 0 {
		s.left--
		s.mu.Unlock()
		return core.Conflict("database is busy")
	}
	s.mu.Unlock()
	return s.Store.Unscoped(ctx, fn)
}

// TestARegistrationRefusedAsBusyIsRetried holds the registrar to the one
// promise a status command depends on: a server that is running is listed.
//
// Returning the error instead ended the worker for the life of the process,
// so the server ran correctly and never appeared in `tix status`, which reads
// as the server being down. Found by running two servers against one SQLite
// file, where the second one's first write is refused as busy.
func TestARegistrationRefusedAsBusyIsRetried(t *testing.T) {
	f := newFixture(t)
	r := presence.New(presence.Config{
		Store:    &busyOnce{Store: f.store, left: 1},
		Address:  func() string { return "127.0.0.1:18299" },
		Version:  "9.9.9",
		Surfaces: []core.ServerSurface{core.ServerSurfaceAPI},
	}, presence.WithClock(f.clk))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopped := make(chan struct{})
	go func() { _ = r.Run(ctx); close(stopped) }()

	waitFor(t, func() bool { return f.clk.Tickers() > 0 })
	if got := f.servers(t); len(got) != 0 {
		t.Fatalf("a refused registration still wrote a row: %+v", got)
	}

	f.clk.Advance(core.ServerHeartbeatInterval)
	waitFor(t, func() bool { return len(f.servers(t)) == 1 })

	got := f.servers(t)
	if got[0].Address != "127.0.0.1:18299" {
		t.Errorf("address = %q, want the configured one", got[0].Address)
	}
	cancel()
	<-stopped
}

// TestStalenessFollowsTheServerSOwnInterval is the defect --heartbeat-interval
// had: the threshold was a compile-time three times the default cadence, so
// the flag moved the writing and nothing moved the reading. It was wrong in
// both directions and both are asserted here, because a fix that only widened
// the window would pass a test that checked one of them.
func TestStalenessFollowsTheServerSOwnInterval(t *testing.T) {
	const (
		slow = 5 * time.Minute
		fast = time.Second
	)
	last := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		interval time.Duration
		since    time.Duration
		attached bool
	}{
		// A server beating every five minutes is one beat behind here, and the
		// fixed threshold called it down at two and a half.
		{"a slow beater is not called down between its beats", slow, 6 * time.Minute, true},
		{"a slow beater is called down after three of its beats", slow, 15*time.Minute + time.Second, false},
		// A server that beat every second was called up for ninety after it
		// stopped, which is the direction an operator notices least.
		{"a fast beater is still up inside three of its beats", fast, 2 * time.Second, true},
		{"a fast beater is called down promptly", fast, 4 * time.Second, false},
		// A row written before the cadence was recorded keeps the meaning it
		// had: judged against the default interval.
		{"a row that declares no cadence is judged against the default", 0, core.ServerStaleAfter, true},
		{"a row that declares no cadence goes stale at the default", 0, core.ServerStaleAfter + time.Second, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := core.Server{LastSeenAt: last, HeartbeatInterval: core.Duration(tt.interval)}
			if got := srv.Attached(last.Add(tt.since)); got != tt.attached {
				t.Errorf("Attached(%v after last seen, beating every %v) = %v, want %v",
					tt.since, tt.interval, got, tt.attached)
			}
		})
	}
}

// TestTheRegistrarRecordsItsOwnCadence closes the other half: a reader can
// only judge against the writer's interval if the writer stored it.
func TestTheRegistrarRecordsItsOwnCadence(t *testing.T) {
	const interval = 5 * time.Minute
	f := newFixture(t)
	r := presence.New(presence.Config{
		Store:   f.store,
		Address: func() string { return "127.0.0.1:1" },
		Version: "9.9.9",
	}, presence.WithClock(f.clk), presence.WithInterval(interval))

	if err := r.Register(context.Background()); err != nil {
		t.Fatalf("registering: %v", err)
	}
	row := f.servers(t)[0]
	if got := time.Duration(row.HeartbeatInterval); got != interval {
		t.Fatalf("stored heartbeat interval = %v, want %v", got, interval)
	}
	// And the row a reader loads is judged against it, not against the default.
	if !row.Attached(row.LastSeenAt.Add(core.ServerStaleAfter + time.Minute)) {
		t.Error("a server beating every 5m read as down inside one of its own beats")
	}
}

// TestABeatAgainstAMissingRowRegistersAgain is the symmetric half of
// TestARegistrationRefusedAsBusyIsRetried. A registration that landed and was
// then deleted underneath the process left the loop beating against nothing
// for the rest of that process's life, and the store discarded the row count
// that would have said so.
func TestABeatAgainstAMissingRowRegistersAgain(t *testing.T) {
	f := newFixture(t)
	r := f.registrar("127.0.0.1:19001", core.ServerSurfaceAPI)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopped := make(chan struct{})
	go func() { _ = r.Run(ctx); close(stopped) }()

	waitFor(t, func() bool { return len(f.servers(t)) == 1 })
	waitFor(t, func() bool { return f.clk.Tickers() > 0 })

	// Somebody removes the row: an operator, another server's purge, a
	// database restored from a backup taken before this process started.
	if err := f.store.Unscoped(ctx, func(u store.UnscopedTx) error {
		return u.DeregisterServer(ctx, r.ServerID())
	}); err != nil {
		t.Fatalf("deleting the row: %v", err)
	}
	if got := f.servers(t); len(got) != 0 {
		t.Fatalf("the row survived its own deletion: %+v", got)
	}

	// The next beat finds nothing, and the one after it registers again.
	f.clk.Advance(core.ServerHeartbeatInterval)
	f.clk.Advance(core.ServerHeartbeatInterval)
	waitFor(t, func() bool { return len(f.servers(t)) == 1 })

	got := f.servers(t)[0]
	if got.ID != r.ServerID() {
		t.Errorf("the restored row is %q, want this process's %q", got.ID, r.ServerID())
	}
	if got.Address != "127.0.0.1:19001" {
		t.Errorf("address = %q, want the configured one", got.Address)
	}
	cancel()
	<-stopped
}

// TestABeatReportsAMissingRow states the store contract the loop above rests
// on, so a change that goes back to discarding the row count fails here too
// rather than only in a goroutine.
func TestABeatReportsAMissingRow(t *testing.T) {
	f := newFixture(t)
	r := f.registrar("127.0.0.1:19002")
	ctx := context.Background()
	if err := r.Register(ctx); err != nil {
		t.Fatalf("registering: %v", err)
	}
	if err := r.Beat(ctx); err != nil {
		t.Fatalf("beating against a live row: %v", err)
	}
	if err := r.Deregister(ctx); err != nil {
		t.Fatalf("deregistering: %v", err)
	}
	if err := r.Beat(ctx); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("beating against a deleted row = %v, want not found", err)
	}
}
