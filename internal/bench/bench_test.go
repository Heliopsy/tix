// SPDX-License-Identifier: AGPL-3.0-or-later

package bench

import (
	"context"
	"fmt"
	"testing"
)

// BenchmarkListFirstPage measures the opening page of a filtered listing.
func BenchmarkListFirstPage(b *testing.B) {
	eachEngine(b, func(b *testing.B, f *Fixture) {
		ctx := context.Background()
		for b.Loop() {
			if _, err := f.FilteredPage(ctx, "", PageLimit); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkListDeepPage measures a page far into the same listing. Keyset
// paging makes it cost what the first page costs; OFFSET would not.
func BenchmarkListDeepPage(b *testing.B) {
	eachEngine(b, func(b *testing.B, f *Fixture) {
		ctx := context.Background()
		cursor := f.CursorAt(int(float64(f.Primary().Tasks) * DeepFraction))
		b.ResetTimer()
		for b.Loop() {
			if _, err := f.FilteredPage(ctx, cursor, PageLimit); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkBoard measures the grouped view: one page per workflow state.
func BenchmarkBoard(b *testing.B) {
	eachEngine(b, func(b *testing.B, f *Fixture) {
		ctx := context.Background()
		project := f.Primary().Projects[0].ID
		b.ResetTimer()
		for b.Loop() {
			if _, err := f.Board(ctx, project, PageLimit); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkSearch measures a substring match over titles and bodies.
func BenchmarkSearch(b *testing.B) {
	eachEngine(b, func(b *testing.B, f *Fixture) {
		ctx := context.Background()
		for b.Loop() {
			if _, err := f.Search(ctx, SearchTerm, PageLimit); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkUnblocked measures the dependency-aware listing ClaimNext shares a
// predicate with.
func BenchmarkUnblocked(b *testing.B) {
	eachEngine(b, func(b *testing.B, f *Fixture) {
		ctx := context.Background()
		for b.Loop() {
			if _, err := f.UnblockedPage(ctx, PageLimit); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkClaimNext measures one uncontended compare-and-swap claim.
func BenchmarkClaimNext(b *testing.B) {
	eachEngine(b, func(b *testing.B, f *Fixture) {
		ctx := context.Background()
		i := 0
		for b.Loop() {
			i++
			if _, err := f.ClaimNext(ctx, fmt.Sprintf("bench-%d", i)); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkClaimNextContended measures the same claim with every goroutine
// racing for the head of the same queue.
func BenchmarkClaimNextContended(b *testing.B) {
	eachEngine(b, func(b *testing.B, f *Fixture) {
		ctx := context.Background()
		var seq atomicCounter
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				token := fmt.Sprintf("bench-race-%d", seq.next())
				if _, err := f.ClaimNext(ctx, token); err != nil {
					b.Error(err)
					return
				}
			}
		})
	})
}

func eachEngine(b *testing.B, fn func(*testing.B, *Fixture)) {
	b.Helper()
	for _, e := range engines(b) {
		f := fixtureFor(b, e)
		b.Run(e.name, func(b *testing.B) { fn(b, f) })
	}
}
