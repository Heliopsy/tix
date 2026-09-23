// SPDX-License-Identifier: AGPL-3.0-or-later

package lease

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
)

func TestResolveTTLPrecedence(t *testing.T) {
	cases := []struct {
		name     string
		policy   Policy
		perClaim core.Duration
		workflow core.Duration
		want     time.Duration
		wantKind core.Kind
	}{
		{name: "package default", want: DefaultTTL},
		{name: "workflow override", workflow: core.Duration(time.Hour), want: time.Hour},
		{
			name:     "per claim beats workflow",
			perClaim: core.Duration(5 * time.Minute),
			workflow: core.Duration(time.Hour),
			want:     5 * time.Minute,
		},
		{
			name:   "global default applies",
			policy: Policy{Default: 2 * time.Minute, Max: time.Hour},
			want:   2 * time.Minute,
		},
		{
			name:     "negative rejected",
			perClaim: core.Duration(-time.Second),
			wantKind: core.KindInvalid,
		},
		{
			name:     "excessive rejected",
			perClaim: core.Duration(MaxTTL + time.Second),
			wantKind: core.KindInvalid,
		},
		{
			name:     "excessive against a tighter policy",
			policy:   Policy{Default: time.Minute, Max: 10 * time.Minute},
			perClaim: core.Duration(11 * time.Minute),
			wantKind: core.KindInvalid,
		},
		{
			name:     "negative workflow default rejected",
			workflow: core.Duration(-time.Hour),
			wantKind: core.KindInvalid,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.policy
			if p == (Policy{}) {
				p = DefaultPolicy()
			}
			got, err := p.Resolve(tc.perClaim, tc.workflow)
			if tc.wantKind != "" {
				if !core.IsKind(err, tc.wantKind) {
					t.Fatalf("Resolve error = %v, want %s", err, tc.wantKind)
				}
				if got != 0 {
					t.Errorf("rejected resolve returned %s, want zero", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if got != tc.want {
				t.Errorf("Resolve = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestResolveUsesTheDefaultPolicy(t *testing.T) {
	got, err := Resolve(0, 0)
	if err != nil || got != DefaultTTL {
		t.Fatalf("Resolve = %s, %v, want %s", got, err, DefaultTTL)
	}
}

func TestZeroPolicyFallsBackToPackageDefaults(t *testing.T) {
	var p Policy
	got, err := p.Resolve(0, 0)
	if err != nil || got != DefaultTTL {
		t.Fatalf("zero policy resolve = %s, %v, want %s", got, err, DefaultTTL)
	}
	if _, err := p.Resolve(core.Duration(MaxTTL+time.Minute), 0); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("zero policy accepted an excessive ttl: %v", err)
	}
}

func TestNewTokenIsOpaqueAndUnique(t *testing.T) {
	seen := make(map[string]bool, 512)
	for range 512 {
		tok, err := NewToken()
		if err != nil {
			t.Fatalf("NewToken: %v", err)
		}
		if len(tok) < 32 {
			t.Fatalf("token %q is too short to be unguessable", tok)
		}
		if seen[tok] {
			t.Fatalf("token %q was minted twice", tok)
		}
		seen[tok] = true
	}
}

func TestRenewInterval(t *testing.T) {
	cases := []struct {
		in, want time.Duration
	}{
		{in: DefaultTTL, want: DefaultTTL / RenewDivisor},
		{in: 3 * time.Second, want: time.Second},
		{in: 2, want: 2},
		{in: 0, want: 0},
		{in: -time.Second, want: 0},
	}
	for _, tc := range cases {
		if got := RenewInterval(tc.in); got != tc.want {
			t.Errorf("RenewInterval(%s) = %s, want %s", tc.in, got, tc.want)
		}
	}
}

func TestSweeperOnceUsesItsBatch(t *testing.T) {
	var gotLimit int
	s := NewSweeper(func(_ context.Context, limit int) (int, error) {
		gotLimit = limit
		return 4, nil
	}, WithBatch(7))

	n, err := s.Once(context.Background())
	if err != nil || n != 4 {
		t.Fatalf("Once = %d, %v", n, err)
	}
	if gotLimit != 7 {
		t.Errorf("sweep limit = %d, want 7", gotLimit)
	}
}

func TestSweeperOptionsIgnoreNonsense(t *testing.T) {
	s := NewSweeper(nil, WithBatch(0), WithInterval(-time.Second))
	if s.batch != DefaultBatch || s.Interval() != DefaultInterval {
		t.Errorf("batch = %d, interval = %s, want the defaults", s.batch, s.Interval())
	}
	if n, err := s.Once(context.Background()); n != 0 || err != nil {
		t.Errorf("a sweeper with no sweep function = %d, %v", n, err)
	}
}

func TestSweeperBoundedStopsAtItsBound(t *testing.T) {
	s := NewSweeper(func(ctx context.Context, _ int) (int, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	})

	if _, err := s.Bounded(context.Background(), time.Millisecond); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Bounded error = %v, want a deadline", err)
	}
}

func TestSweeperBoundedDefaultsItsBound(t *testing.T) {
	s := NewSweeper(func(ctx context.Context, _ int) (int, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Error("an opportunistic sweep ran without a time bound")
		}
		return 2, nil
	})
	if n, err := s.Bounded(context.Background(), 0); n != 2 || err != nil {
		t.Fatalf("Bounded = %d, %v", n, err)
	}
}

func TestSweeperRunsOnItsTicker(t *testing.T) {
	clk := clock.NewFakeAt()
	swept := make(chan int, 4)
	failed := make(chan error, 4)

	calls := 0
	s := NewSweeper(func(context.Context, int) (int, error) {
		calls++
		if calls == 1 {
			return 0, errors.New("transient sweep failure")
		}
		swept <- calls
		return 1, nil
	}, WithClock(clk), WithInterval(time.Minute), WithErrorHandler(func(err error) { failed <- err }))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	if err := <-tick(t, clk, time.Minute, failed); err == nil {
		t.Fatal("the error handler received no failure")
	}
	if got := <-tick(t, clk, time.Minute, swept); got != 2 {
		t.Fatalf("sweep call = %d, want the second pass", got)
	}

	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Run returned %v, want the cancellation", err)
	}
}

// tick advances the fake clock until the sweeper's ticker has been registered
// and has fired, so the test never depends on goroutine scheduling order.
func tick[T any](t *testing.T, clk *clock.Fake, d time.Duration, ch chan T) chan T {
	t.Helper()
	out := make(chan T, 1)
	deadline := time.After(5 * time.Second)
	for {
		clk.Advance(d)
		select {
		case v := <-ch:
			out <- v
			return out
		case <-deadline:
			t.Fatal("the sweeper never fired on its ticker")
		default:
			runtime.Gosched()
		}
	}
}
