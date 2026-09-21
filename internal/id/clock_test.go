package id

import (
	"sync"
	"testing"
	"time"
)

// stoppedClock never advances, which is exactly what a test using
// clock.FakeClock hands the generator.
type stoppedClock struct{ at time.Time }

func (c stoppedClock) Now() time.Time { return c.at }

// timestampOf returns the characters of an identifier that encode the
// millisecond, which is the first 48 bits and so the first ten characters.
func timestampOf(s string) string { return s[:10] }

func TestGeneratorTimestampsFromInjectedClock(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	g := NewGenerator(stoppedClock{at: at})

	want := timestampOf(NewGenerator(nil).NewAt(at))
	for i := range 5 {
		got := g.New()
		if timestampOf(got) != want {
			t.Fatalf("identifier %d = %q, timestamp %q, want %q from the injected clock",
				i, got, timestampOf(got), want)
		}
	}
}

func TestGeneratorIsUniqueAndSortedUnderAStoppedClock(t *testing.T) {
	g := NewGenerator(stoppedClock{at: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)})

	seen := map[string]bool{}
	prev := ""
	for i := range 2000 {
		got := g.New()
		if seen[got] {
			t.Fatalf("identifier %d is a duplicate under a stopped clock: %q", i, got)
		}
		seen[got] = true
		if prev != "" && got <= prev {
			t.Fatalf("identifier %d does not sort after its predecessor: %q then %q", i, prev, got)
		}
		prev = got
	}
}

func TestGeneratorWithInjectedClockIsRaceFree(t *testing.T) {
	g := NewGenerator(stoppedClock{at: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)})

	var mu sync.Mutex
	seen := map[string]bool{}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 200 {
				v := g.New()
				mu.Lock()
				if seen[v] {
					t.Errorf("duplicate under concurrency with a stopped clock: %q", v)
				}
				seen[v] = true
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
}

func TestIncrReportsOverflow(t *testing.T) {
	var b [10]byte
	if incr(&b) {
		t.Errorf("incr on a zero value reported an overflow")
	}
	for i := range b {
		b[i] = 0xFF
	}
	if !incr(&b) {
		t.Errorf("incr on an exhausted value did not report an overflow")
	}
}

func TestExhaustedMillisecondStillSorts(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	g := &generator{clock: stoppedClock{at: at}}

	first := g.New()
	g.mu.Lock()
	for i := range g.lastRand {
		g.lastRand[i] = 0xFF
	}
	g.mu.Unlock()

	second := g.New()
	if second <= first {
		t.Fatalf("after exhausting a millisecond the next identifier must still sort later: %q then %q",
			first, second)
	}
}
