package auth

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// swapVerifyGate installs g for the duration of one test.
func swapVerifyGate(t *testing.T, g *gate) {
	t.Helper()
	previous := verifyGate
	verifyGate = g
	t.Cleanup(func() { verifyGate = previous })
}

// hold occupies every slot of g until the returned release is called.
func hold(t *testing.T, g *gate, slots int) func() {
	t.Helper()
	held := make(chan struct{})
	var wg sync.WaitGroup
	admitted := make(chan struct{}, slots)
	for range slots {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = g.do(func() {
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
			_ = g.do(func() {
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

func TestGateRefusesPastQueue(t *testing.T) {
	g := newGate(1, 0)
	release := hold(t, g, 1)

	refused := g.do(func() { t.Error("a refused caller still ran") })
	if !core.IsKind(refused, core.KindPrecondition) {
		t.Fatalf("refusal = %v, want a precondition error", refused)
	}

	release()
	if err := g.do(func() {}); err != nil {
		t.Fatalf("the gate stayed closed after draining: %v", err)
	}
}

// TestVerifyIsGated pins that password verification runs under the process
// bound rather than allocating one argon2 arena per inbound request. Without
// the gate, Verify computes regardless of how many derivations are in flight.
func TestVerifyIsGated(t *testing.T) {
	h := NewHasherWithParams(TestParams())
	encoded, err := h.Hash("correct-horse-battery")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	swapVerifyGate(t, newGate(1, 0))
	release := hold(t, verifyGate, 1)
	defer release()

	if err := Verify(encoded, "correct-horse-battery"); !core.IsKind(err, core.KindPrecondition) {
		t.Fatalf("a saturated server verified anyway: %v", err)
	}
}

// TestVerifyRefusalCarriesNoAccountSignal pins that an overloaded server
// answers a wrong password and an account nobody holds identically, so a
// refusal cannot be used to enumerate users.
func TestVerifyRefusalCarriesNoAccountSignal(t *testing.T) {
	h := NewHasherWithParams(TestParams())
	account, err := h.Hash("correct-horse-battery")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	dummy, err := h.Hash("tix-absent-tix-absent")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	swapVerifyGate(t, newGate(1, 0))
	release := hold(t, verifyGate, 1)
	defer release()

	wrong := Verify(account, "not-the-password")
	unknown := Verify(dummy, "not-the-password")
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

// TestVerifyStillDecides pins that gating did not change the verdict.
func TestVerifyStillDecides(t *testing.T) {
	h := NewHasherWithParams(TestParams())
	encoded, err := h.Hash("correct-horse-battery")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if err := Verify(encoded, "correct-horse-battery"); err != nil {
		t.Fatalf("a matching password was rejected: %v", err)
	}
	if err := Verify(encoded, "wrong-horse-battery"); !core.IsKind(err, core.KindUnauthenticated) {
		t.Fatalf("a wrong password gave %v", err)
	}
}

// TestVerifyUnderLoadStaysCorrect pins that the queue, not the verdict, is
// what a burst changes.
func TestVerifyUnderLoadStaysCorrect(t *testing.T) {
	h := NewHasherWithParams(TestParams())
	encoded, err := h.Hash("correct-horse-battery")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	swapVerifyGate(t, newGate(2, 256))

	var wg sync.WaitGroup
	bad := make(chan error, 64)
	for range 64 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := Verify(encoded, "correct-horse-battery"); err != nil {
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
