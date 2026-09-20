package bench

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

// Operation names, which are also the budget keys.
const (
	opListFirst   = "list_first_page"
	opListDeep    = "list_deep_page"
	opBoard       = "board_grouped"
	opSearch      = "search_body"
	opUnblocked   = "list_unblocked"
	opClaimSerial = "claim_next_serial"
	opClaimRace   = "claim_next_contended"
)

// budgets are p95 ceilings keyed "<profile>/<engine>". Each is roughly three
// times the p95 measured on the reference machine, recorded in README.md, which
// absorbs a loaded worker without absorbing a lost index. A profile with no
// entry is measured and reported but not asserted.
var budgets = map[string]map[string]time.Duration{
	"small/sqlite": {
		opListFirst:   6 * time.Millisecond,
		opListDeep:    9 * time.Millisecond,
		opBoard:       45 * time.Millisecond,
		opSearch:      6 * time.Millisecond,
		opUnblocked:   300 * time.Millisecond,
		opClaimSerial: 1500 * time.Millisecond,
		opClaimRace:   9 * time.Second,
	},
	"small/postgres": {
		opListFirst:   25 * time.Millisecond,
		opListDeep:    25 * time.Millisecond,
		opBoard:       70 * time.Millisecond,
		opSearch:      25 * time.Millisecond,
		opUnblocked:   30 * time.Millisecond,
		opClaimSerial: 40 * time.Millisecond,
		opClaimRace:   150 * time.Millisecond,
	},
	"medium/sqlite": {
		opListFirst: 35 * time.Millisecond,
		opListDeep:  50 * time.Millisecond,
		opBoard:     600 * time.Millisecond,
		opSearch:    175 * time.Millisecond,
	},
}

// DeepFraction is how far into the listing the deep-pagination case starts. It
// leaves more than one page of matching rows beyond the cursor.
const DeepFraction = 0.8

// ClaimWorkers is how many goroutines race for the head of the queue.
const ClaimWorkers = 4

// ClaimsPerWorker is how many claims each racing goroutine takes.
const ClaimsPerWorker = 2

// ClaimTimeout fails a claim that has stopped being a bounded operation instead
// of letting the suite hang.
const ClaimTimeout = 90 * time.Second

// BudgetsEnv opts a run into enforcing the committed ceilings. They are
// wall-clock numbers measured on one machine against local disk, so they do not
// survive a container bind mount or a loaded host: enforcing them everywhere
// would make the suite fail for reasons that have nothing to do with the code.
// Unset, the run still measures and logs, which is what catches a plan
// regression in review.
const BudgetsEnv = "TIX_BENCH_BUDGETS"

func TestPercentileBudgets(t *testing.T) {
	ctx := context.Background()
	enforce := os.Getenv(BudgetsEnv) != ""
	for _, e := range engines(t) {
		t.Run(e.name, func(t *testing.T) {
			f := fixtureFor(t, e)
			key := f.Spec.Name + "/" + e.name
			budget := budgets[key]
			switch {
			case !enforce:
				t.Logf("%s is unset; measuring only", BudgetsEnv)
				budget = nil
			case budget == nil:
				t.Logf("no committed budget for %q; measuring only", key)
			}
			check := func(name string, stats Stats) {
				t.Logf("%s %s %s", key, name, stats)
				if ceiling, ok := budget[name]; ok && stats.P95 > ceiling {
					t.Errorf("%s p95 = %s, over the %s budget", name, stats.P95, ceiling)
				}
			}

			for _, c := range budgetCases(ctx, f) {
				// The costly cases are stress measurements, not assertions about
				// correctness, and claim_next_contended cannot finish in the time
				// it is given on slower storage (see internal/bench/README.md).
				// They run when a measurement is actually being taken.
				if c.costly && (testing.Short() || !enforce) {
					t.Logf("%s: skipped; set %s to measure it", c.name, BudgetsEnv)
					continue
				}
				stats, err := Measure(c.iterations, c.run)
				if err != nil {
					t.Fatalf("%s: %v", c.name, err)
				}
				check(c.name, stats)
			}

			if testing.Short() || !enforce {
				t.Logf("%s: skipped; set %s to measure it", opClaimRace, BudgetsEnv)
				return
			}
			stats, err := contendedClaim(ctx, f)
			if err != nil {
				t.Fatalf("%s: %v", opClaimRace, err)
			}
			check(opClaimRace, stats)
		})
	}
}

type budgetCase struct {
	name       string
	iterations int
	// costly marks an operation whose cost is superlinear in the fixture, so
	// -short leaves it out.
	costly bool
	run    func() error
}

func budgetCases(ctx context.Context, f *Fixture) []budgetCase {
	t := f.Primary()
	deep := f.CursorAt(int(float64(t.Tasks) * DeepFraction))
	project := t.Projects[0].ID
	claims := 0

	return []budgetCase{
		{name: opListFirst, iterations: 60, run: func() error {
			_, err := f.FilteredPage(ctx, "", PageLimit)
			return err
		}},
		{name: opListDeep, iterations: 60, run: func() error {
			_, err := f.FilteredPage(ctx, deep, PageLimit)
			return err
		}},
		{name: opBoard, iterations: 40, run: func() error {
			_, err := f.Board(ctx, project, PageLimit)
			return err
		}},
		{name: opSearch, iterations: 40, run: func() error {
			_, err := f.Search(ctx, SearchTerm, PageLimit)
			return err
		}},
		{name: opUnblocked, iterations: 20, costly: true, run: func() error {
			_, err := f.UnblockedPage(ctx, PageLimit)
			return err
		}},
		{name: opClaimSerial, iterations: 5, costly: true, run: func() error {
			claims++
			return claimOnce(ctx, f, fmt.Sprintf("serial-%d", claims))
		}},
	}
}

func claimOnce(ctx context.Context, f *Fixture, token string) error {
	ctx, cancel := context.WithTimeout(ctx, ClaimTimeout)
	defer cancel()
	_, err := f.ClaimNext(ctx, token)
	return err
}

// contendedClaim runs ClaimWorkers goroutines competing for the same queue
// head, which is the case the compare-and-swap exists for.
func contendedClaim(ctx context.Context, f *Fixture) (Stats, error) {
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		samples []time.Duration
		failure error
	)
	for w := range ClaimWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range ClaimsPerWorker {
				start := time.Now()
				err := claimOnce(ctx, f, fmt.Sprintf("race-%d-%d", w, i))
				elapsed := time.Since(start)
				mu.Lock()
				if err != nil {
					if failure == nil {
						failure = err
					}
					mu.Unlock()
					return
				}
				samples = append(samples, elapsed)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if failure != nil {
		return Stats{}, failure
	}
	return Summarize(samples), nil
}

// TestDeepPaginationDoesNotDegrade is the scale-free guard against an OFFSET
// creeping back in: a keyset page costs what the first page costs wherever it
// starts, while an OFFSET page costs more the deeper it is.
func TestDeepPaginationDoesNotDegrade(t *testing.T) {
	const tolerance = 4.0
	const iterations = 60
	ctx := context.Background()
	for _, e := range engines(t) {
		t.Run(e.name, func(t *testing.T) {
			f := fixtureFor(t, e)

			first, err := Measure(iterations, func() error {
				_, err := f.FilteredPage(ctx, "", PageLimit)
				return err
			})
			if err != nil {
				t.Fatalf("first page: %v", err)
			}
			cursor := f.CursorAt(int(float64(f.Primary().Tasks) * DeepFraction))
			deep, err := Measure(iterations, func() error {
				_, err := f.FilteredPage(ctx, cursor, PageLimit)
				return err
			})
			if err != nil {
				t.Fatalf("deep page: %v", err)
			}

			t.Logf("first %s", first)
			t.Logf("deep  %s", deep)
			if ratio := float64(deep.P95) / float64(first.P95); ratio > tolerance {
				t.Errorf("deep page p95 is %.1fx the first page (%s vs %s); a keyset page must not"+
					" get dearer with depth", ratio, deep.P95, first.P95)
			}
		})
	}
}

// TestDeepPageReturnsAFullPage keeps the timing honest: a page that returns
// nothing is fast for the wrong reason.
func TestDeepPageReturnsAFullPage(t *testing.T) {
	ctx := context.Background()
	for _, e := range engines(t) {
		t.Run(e.name, func(t *testing.T) {
			f := fixtureFor(t, e)
			cursor := f.CursorAt(int(float64(f.Primary().Tasks) * DeepFraction))
			n, err := f.FilteredPage(ctx, cursor, PageLimit)
			if err != nil {
				t.Fatalf("deep page: %v", err)
			}
			if n != PageLimit {
				t.Errorf("deep page returned %d tasks, want a full page of %d", n, PageLimit)
			}
		})
	}
}

// TestSearchFindsTheSeededToken guards the search case against measuring an
// empty result set.
func TestSearchFindsTheSeededToken(t *testing.T) {
	ctx := context.Background()
	for _, e := range engines(t) {
		t.Run(e.name, func(t *testing.T) {
			f := fixtureFor(t, e)
			n, err := f.Search(ctx, SearchTerm, PageLimit)
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			if n == 0 {
				t.Errorf("search for %q matched nothing", SearchTerm)
			}
		})
	}
}

// DepthSamples are the fractions of the listing the depth curve probes.
var DepthSamples = []float64{0, 0.1, 0.25, 0.5, 0.75, 0.9}

// TestPaginationDepthCurve reports what a page costs at several depths. It
// asserts nothing; TestDeepPaginationDoesNotDegrade owns the verdict. The curve
// is what distinguishes the two ways a listing can stop being O(1): a cost that
// rises with depth is an OFFSET, one that falls with depth is a plan that reads
// everything after the cursor and sorts.
func TestPaginationDepthCurve(t *testing.T) {
	const iterations = 20
	ctx := context.Background()
	for _, e := range engines(t) {
		t.Run(e.name, func(t *testing.T) {
			f := fixtureFor(t, e)
			total := f.Primary().Tasks
			for _, frac := range DepthSamples {
				cursor := f.CursorAt(int(float64(total) * frac))
				stats, err := Measure(iterations, func() error {
					_, err := f.FilteredPage(ctx, cursor, PageLimit)
					return err
				})
				if err != nil {
					t.Fatalf("depth %.0f%%: %v", frac*100, err)
				}
				t.Logf("depth=%3.0f%% %s", frac*100, stats)
			}
		})
	}
}
