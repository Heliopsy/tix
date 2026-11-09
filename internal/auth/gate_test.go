// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"context"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// installGate makes g the process-wide gate for the duration of one test.
func installGate(t *testing.T, g *gate) {
	t.Helper()
	previous := swapHashGate(g)
	t.Cleanup(func() { swapHashGate(previous) })
}

// hold occupies slots of g until the returned release is called.
func hold(t *testing.T, g *gate, slots int) func() {
	t.Helper()
	held := make(chan struct{})
	var wg sync.WaitGroup
	admitted := make(chan struct{}, slots)
	for range slots {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = g.do(context.Background(), func() {
				admitted <- struct{}{}
				<-held
			})
		}()
	}
	for range slots {
		<-admitted
	}
	return func() {
		close(held)
		wg.Wait()
	}
}

func TestGateBoundsConcurrency(t *testing.T) {
	g := newGate(3, 64)

	var (
		inFlight atomic.Int64
		peak     atomic.Int64
		wg       sync.WaitGroup
	)
	release := make(chan struct{})
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = g.do(context.Background(), func() {
				now := inFlight.Add(1)
				for {
					was := peak.Load()
					if now <= was || peak.CompareAndSwap(was, now) {
						break
					}
				}
				<-release
				inFlight.Add(-1)
			})
		}()
	}
	for peak.Load() < 3 {
		runtime.Gosched()
	}
	close(release)
	wg.Wait()

	if got := peak.Load(); got > 3 {
		t.Fatalf("peak concurrency = %d, want at most 3", got)
	}
}

// watchDerivation wraps every derivation for one test.
func watchDerivation(t *testing.T, wrap func(func()) func()) {
	t.Helper()
	previous := derivation
	derivation = func(password string, salt []byte, p Params) []byte {
		var key []byte
		wrap(func() { key = previous(password, salt, p) })()
		return key
	}
	t.Cleanup(func() { derivation = previous })
}

// TestVerifyHoldsAtMostNInFlight runs N+1 verifications against a gate of N
// and counts how many derivations run at once while all of them are waiting.
func TestVerifyHoldsAtMostNInFlight(t *testing.T) {
	const n = 2
	encoded, err := NewHasherWithParams(TestParams()).Hash(t.Context(), testPassword)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	g := newGate(n, 8)
	installGate(t, g)

	var inFlight, peak atomic.Int64
	open := make(chan struct{})
	watchDerivation(t, func(fn func()) func() {
		return func() {
			now := inFlight.Add(1)
			for was := peak.Load(); now > was && !peak.CompareAndSwap(was, now); was = peak.Load() {
			}
			<-open
			fn()
			inFlight.Add(-1)
		}
	})

	var wg sync.WaitGroup
	errs := make(chan error, n+1)
	for range n + 1 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- Verify(t.Context(), encoded, testPassword)
		}()
	}
	for (peak.Load() < n || g.waiting.Load() < n+1) && peak.Load() <= n {
		runtime.Gosched()
	}
	if got := inFlight.Load(); got != n {
		t.Fatalf("with %d callers in, %d derivations ran at once, want %d", n+1, got, n)
	}
	close(open)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("a queued verification failed: %v", err)
		}
	}
	if got := peak.Load(); got != n {
		t.Fatalf("peak concurrency = %d, want %d", got, n)
	}
}

func TestGateRefusesPastQueue(t *testing.T) {
	g := newGate(1, 0)
	release := hold(t, g, 1)

	refused := g.do(context.Background(), func() { t.Error("a refused caller still ran") })
	if !IsHashBusy(refused) {
		t.Fatalf("refusal = %v, want ErrHashBusy", refused)
	}

	release()
	if err := g.do(context.Background(), func() {}); err != nil {
		t.Fatalf("the gate stayed closed after draining: %v", err)
	}
}

// TestGateGivesUpWhenTheContextEnds pins the timeout path: a queued caller
// whose request ends is refused and never derives.
func TestGateGivesUpWhenTheContextEnds(t *testing.T) {
	g := newGate(1, 4)
	release := hold(t, g, 1)
	defer release()

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- g.do(ctx, func() { t.Error("a caller that gave up still derived") }) }()
	for g.waiting.Load() < 2 {
		runtime.Gosched()
	}
	cancel()
	if err := within(t, done); !IsHashBusy(err) {
		t.Fatalf("a cancelled wait answered %v, want ErrHashBusy", err)
	}
}

// within returns what done delivers, failing the test if it takes longer than
// a refusal ever should.
func within(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("the gate kept a caller that should have been refused")
		return nil
	}
}

// TestGateGivesUpAfterItsPatience pins that a caller with no deadline of its
// own still stops waiting.
func TestGateGivesUpAfterItsPatience(t *testing.T) {
	g := newGate(1, 4)
	g.patience = time.Millisecond
	release := hold(t, g, 1)
	defer release()

	done := make(chan error, 1)
	go func() { done <- g.do(t.Context(), func() { t.Error("a caller past its patience still derived") }) }()
	if err := within(t, done); !IsHashBusy(err) {
		t.Fatalf("an expired wait answered %v, want ErrHashBusy", err)
	}
}

// TestVerifyTimesOutWithoutDeriving pins that Verify, saturated, refuses
// with ErrHashBusy and never says the password was right or wrong.
func TestVerifyTimesOutWithoutDeriving(t *testing.T) {
	encoded, err := NewHasherWithParams(TestParams()).Hash(t.Context(), testPassword)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	g := newGate(1, 4)
	installGate(t, g)
	release := hold(t, g, 1)
	defer release()

	var ran atomic.Int64
	watchDerivation(t, func(fn func()) func() { return func() { ran.Add(1); fn() } })
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, password := range []string{testPassword, "not-the-password"} {
		if err := Verify(ctx, encoded, password); !IsHashBusy(err) {
			t.Fatalf("a saturated server answered %v, want ErrHashBusy", err)
		}
	}
	if ran.Load() != 0 {
		t.Fatalf("a refused verification derived %d times", ran.Load())
	}
}

// TestHashIsGated pins that minting a hash, not only checking one, waits for
// a slot.
func TestHashIsGated(t *testing.T) {
	g := newGate(1, 0)
	installGate(t, g)
	release := hold(t, g, 1)
	defer release()

	if _, err := NewHasherWithParams(TestParams()).Hash(t.Context(), testPassword); !IsHashBusy(err) {
		t.Fatalf("a saturated server hashed anyway: %v", err)
	}
}

// TestVerifyRefusalCarriesNoAccountSignal pins that an overloaded server
// answers a wrong password and an account nobody holds identically, so a
// refusal cannot be used to enumerate users.
func TestVerifyRefusalCarriesNoAccountSignal(t *testing.T) {
	h := NewHasherWithParams(TestParams())
	account, err := h.Hash(t.Context(), "correct-horse-battery")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	dummy, err := h.Dummy()
	if err != nil {
		t.Fatalf("dummy: %v", err)
	}

	installGate(t, newGate(1, 0))
	release := hold(t, hashGate(), 1)
	defer release()

	wrong := Verify(t.Context(), account, "not-the-password")
	unknown := Verify(t.Context(), dummy, "not-the-password")
	if wrong == nil || unknown == nil {
		t.Fatalf("expected both calls to be refused, got %v and %v", wrong, unknown)
	}
	if wrong.Error() != unknown.Error() {
		t.Fatalf("refusals differ: %q vs %q", wrong, unknown)
	}
	if !core.IsKind(wrong, core.KindPrecondition) {
		t.Fatalf("refusal kind = %v", wrong)
	}
}

// TestDummyCostsADerivation pins the timing equaliser: verifying against the
// dummy decodes to the hasher's own parameters and goes through the gate,
// so it derives exactly as a real account's verification does.
func TestDummyCostsADerivation(t *testing.T) {
	p := TestParams()
	p.Memory, p.Time = p.Memory*2, p.Time+1
	h := NewHasherWithParams(p)
	dummy, err := h.Dummy()
	if err != nil {
		t.Fatalf("dummy: %v", err)
	}
	params, _, _, err := decode(dummy)
	if err != nil {
		t.Fatalf("decoding the dummy: %v", err)
	}
	if params != h.Params() {
		t.Fatalf("dummy params = %+v, want the hasher's %+v", params, h.Params())
	}

	var ran atomic.Int64
	watchDerivation(t, func(fn func()) func() { return func() { ran.Add(1); fn() } })
	if err := Verify(t.Context(), dummy, testPassword); !core.IsKind(err, core.KindUnauthenticated) {
		t.Fatalf("the dummy answered %v, want invalid credentials", err)
	}
	if ran.Load() != 1 {
		t.Fatalf("verifying the dummy derived %d times, want 1", ran.Load())
	}
}

// TestVerifyUnderLoadStaysCorrect pins that the queue, not the verdict, is
// what a burst changes.
func TestVerifyUnderLoadStaysCorrect(t *testing.T) {
	encoded, err := NewHasherWithParams(TestParams()).Hash(t.Context(), "correct-horse-battery")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	installGate(t, newGate(2, 256))

	var wg sync.WaitGroup
	bad := make(chan error, 64)
	for range 64 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := Verify(t.Context(), encoded, "correct-horse-battery"); err != nil {
				bad <- err
			}
		}()
	}
	wg.Wait()
	close(bad)
	for err := range bad {
		t.Fatalf("a queued verification failed: %v", err)
	}
}

func TestConcurrencyFor(t *testing.T) {
	const mib = 1 << 20
	mem := DefaultParams().Memory
	tests := []struct {
		name  string
		limit int64
		procs int
		want  int
	}{
		{"no limit", math.MaxInt64, 8, unboundedConcurrency},
		{"no limit, one core", math.MaxInt64, 1, 1},
		{"unset", 0, 8, unboundedConcurrency},
		{"256 MiB pod at 85%", 217 * mib, 8, 1},
		{"512 MiB pod at 85%", 435 * mib, 8, 2},
		{"1 GiB pod at 85%", 870 * mib, 8, 4},
		{"tiny limit still admits one", 32 * mib, 8, 1},
		{"capped at the cores", 64 << 30, 4, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ConcurrencyFor(tt.limit, mem, tt.procs); got != tt.want {
				t.Fatalf("ConcurrencyFor(%d, %d, %d) = %d, want %d", tt.limit, mem, tt.procs, got, tt.want)
			}
		})
	}
}

// TestSetMaxConcurrentHashes pins that an operator's number wins over the
// sizing, and 0 hands the choice back to it.
func TestSetMaxConcurrentHashes(t *testing.T) {
	previous := swapHashGate(nil)
	t.Cleanup(func() {
		SetMaxConcurrentHashes(0, 0)
		swapHashGate(previous)
	})

	SetMaxConcurrentHashes(3, DefaultParams().Memory)
	if got := MaxConcurrentHashes(); got != 3 {
		t.Fatalf("with an override of 3 the bound is %d", got)
	}
	SetMaxConcurrentHashes(0, DefaultParams().Memory)
	if got := MaxConcurrentHashes(); got < 1 {
		t.Fatalf("the sized bound is %d, want at least 1", got)
	}
}
