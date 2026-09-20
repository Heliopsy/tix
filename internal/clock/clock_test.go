package clock

import (
	"sync"
	"testing"
	"time"
)

func TestRealClockReturnsUTC(t *testing.T) {
	// Times are stored as RFC3339 strings on SQLite, where a non-UTC offset
	// would break the lexicographic ordering lease comparisons depend on.
	got := New().Now()
	if got.Location() != time.UTC {
		t.Errorf("Now() location = %v, want UTC", got.Location())
	}
}

func TestRealClockSince(t *testing.T) {
	c := New()
	past := c.Now().Add(-time.Second)
	if d := c.Since(past); d < time.Second {
		t.Errorf("Since() = %v, want at least 1s", d)
	}
}

func TestRealTickerFires(t *testing.T) {
	c := New()
	tk := c.NewTicker(time.Millisecond)
	defer tk.Stop()

	select {
	case <-tk.C():
	case <-time.After(2 * time.Second):
		t.Fatal("real ticker did not fire")
	}
}

func TestRealAfterAndSleep(t *testing.T) {
	c := New()
	select {
	case <-c.After(time.Millisecond):
	case <-time.After(2 * time.Second):
		t.Fatal("After() did not fire")
	}

	start := time.Now()
	c.Sleep(time.Millisecond)
	if time.Since(start) < time.Millisecond {
		t.Error("Sleep() returned too early")
	}
}

func TestFakeNowIsControlled(t *testing.T) {
	start := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	f := NewFake(start)

	if !f.Now().Equal(start) {
		t.Errorf("Now() = %v, want %v", f.Now(), start)
	}

	f.Advance(90 * time.Minute)
	want := start.Add(90 * time.Minute)
	if !f.Now().Equal(want) {
		t.Errorf("after Advance, Now() = %v, want %v", f.Now(), want)
	}

	f.Set(start)
	if !f.Now().Equal(start) {
		t.Errorf("after Set, Now() = %v, want %v", f.Now(), start)
	}
}

func TestFakeNormalizesToUTC(t *testing.T) {
	loc := time.FixedZone("test", 3*60*60)
	f := NewFake(time.Date(2026, 9, 20, 12, 0, 0, 0, loc))

	if f.Now().Location() != time.UTC {
		t.Errorf("Now() location = %v, want UTC", f.Now().Location())
	}
}

func TestFakeSince(t *testing.T) {
	start := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	f := NewFake(start)
	f.Advance(time.Hour)

	if got := f.Since(start); got != time.Hour {
		t.Errorf("Since() = %v, want 1h", got)
	}
}

func TestFakeAfterFiresOnAdvance(t *testing.T) {
	f := NewFakeAt()
	ch := f.After(15 * time.Minute)

	select {
	case <-ch:
		t.Fatal("After() fired before the clock advanced")
	default:
	}

	f.Advance(14 * time.Minute)
	select {
	case <-ch:
		t.Fatal("After() fired early")
	default:
	}

	f.Advance(time.Minute)
	select {
	case <-ch:
	default:
		t.Fatal("After() did not fire once the deadline was reached")
	}
}

func TestFakeAfterNonPositiveFiresImmediately(t *testing.T) {
	f := NewFakeAt()
	for _, d := range []time.Duration{0, -time.Second} {
		select {
		case <-f.After(d):
		default:
			t.Errorf("After(%v) should fire immediately", d)
		}
	}
}

func TestFakeAfterFiresOnSet(t *testing.T) {
	start := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	f := NewFake(start)
	ch := f.After(time.Hour)

	f.Set(start.Add(2 * time.Hour))
	select {
	case <-ch:
	default:
		t.Fatal("Set() past the deadline should fire the waiter")
	}
}

func TestFakeTickerFiresRepeatedly(t *testing.T) {
	f := NewFakeAt()
	tk := f.NewTicker(time.Minute)
	defer tk.Stop()

	f.Advance(time.Minute)
	select {
	case <-tk.C():
	default:
		t.Fatal("ticker did not fire after one interval")
	}

	// A ticker must keep firing, which is what drives lease renewal.
	f.Advance(time.Minute)
	select {
	case <-tk.C():
	default:
		t.Fatal("ticker did not fire on the second interval")
	}
}

func TestFakeTickerStopped(t *testing.T) {
	f := NewFakeAt()
	tk := f.NewTicker(time.Minute)
	tk.Stop()

	f.Advance(10 * time.Minute)
	select {
	case <-tk.C():
		t.Error("a stopped ticker must not fire")
	default:
	}
}

func TestFakeSleepUnblocksOnAdvance(t *testing.T) {
	f := NewFakeAt()

	done := make(chan struct{})
	go func() {
		f.Sleep(time.Hour)
		close(done)
	}()

	// Wait for the sleeper to register before advancing, otherwise the advance
	// races ahead of it and the test hangs.
	waitForWaiters(t, f, 1)
	f.Advance(time.Hour)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Sleep() did not return after the clock advanced")
	}
}

func TestFakeSleepNonPositiveReturnsImmediately(t *testing.T) {
	f := NewFakeAt()
	done := make(chan struct{})
	go func() {
		f.Sleep(0)
		f.Sleep(-time.Second)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Sleep() with a non-positive duration must return immediately")
	}
}

func TestFakeIsConcurrencySafe(t *testing.T) {
	// Lease tests advance the clock while the code under test reads it, so the
	// fake must tolerate that without a data race.
	f := NewFakeAt()

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				_ = f.Now()
			}
		}()
	}
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				f.Advance(time.Millisecond)
			}
		}()
	}
	wg.Wait()

	if got := f.Now(); got.IsZero() {
		t.Error("clock should still report a time after concurrent use")
	}
}

func TestFakeMultipleWaitersFireIndependently(t *testing.T) {
	f := NewFakeAt()
	short := f.After(time.Minute)
	long := f.After(time.Hour)

	f.Advance(time.Minute)
	select {
	case <-short:
	default:
		t.Fatal("the short waiter should have fired")
	}
	select {
	case <-long:
		t.Fatal("the long waiter must not fire early")
	default:
	}

	f.Advance(time.Hour)
	select {
	case <-long:
	default:
		t.Fatal("the long waiter should have fired")
	}
}

// waitForWaiters blocks until the fake clock has at least n registered waiters.
func waitForWaiters(t *testing.T, f *Fake, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		got := len(f.waiters)
		f.mu.Unlock()
		if got >= n {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d waiters", n)
}
