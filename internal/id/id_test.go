// SPDX-License-Identifier: AGPL-3.0-or-later

package id

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewHasCorrectShape(t *testing.T) {
	got := New()
	if len(got) != Length {
		t.Errorf("New() = %q, length %d, want %d", got, len(got), Length)
	}
	if !Valid(got) {
		t.Errorf("New() = %q, which does not validate", got)
	}
}

func TestNewIsUnique(t *testing.T) {
	const n = 20000
	seen := make(map[string]bool, n)
	for range n {
		v := New()
		if seen[v] {
			t.Fatalf("New() produced a duplicate: %q", v)
		}
		seen[v] = true
	}
}

func TestIdentifiersSortInCreationOrder(t *testing.T) {
	base := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	var ids []string
	for i := range 200 {
		ids = append(ids, NewAt(base.Add(time.Duration(i)*time.Millisecond)))
	}

	sorted := make([]string, len(ids))
	copy(sorted, ids)
	sort.Strings(sorted)

	for i := range ids {
		if ids[i] != sorted[i] {
			t.Fatalf("identifiers do not sort in creation order at index %d: %q vs %q",
				i, ids[i], sorted[i])
		}
	}
}

func TestSameMillisecondIsMonotonic(t *testing.T) {
	at := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)

	var ids []string
	for range 500 {
		ids = append(ids, NewAt(at))
	}

	for i := 1; i < len(ids); i++ {
		if ids[i] <= ids[i-1] {
			t.Fatalf("identifiers within one millisecond are not increasing: %q then %q",
				ids[i-1], ids[i])
		}
	}
}

func TestTimestampOrderingDominatesRandomness(t *testing.T) {
	early := NewAt(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	late := NewAt(time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC))

	if early >= late {
		t.Errorf("an earlier identifier must sort before a later one: %q >= %q", early, late)
	}
}

func TestValid(t *testing.T) {
	valid := New()
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"generated", valid, true},
		{"lowercase is accepted", strings.ToLower(valid), true},
		{"empty", "", false},
		{"too short", valid[:Length-1], false},
		{"too long", valid + "0", false},
		{"excluded letter I", strings.Repeat("I", Length), false},
		{"excluded letter L", strings.Repeat("L", Length), false},
		{"excluded letter O", strings.Repeat("O", Length), false},
		{"excluded letter U", strings.Repeat("U", Length), false},
		{"punctuation", strings.Repeat("-", Length), false},
		{"path traversal", "../../../../../../etc/passwd"[:Length], false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Valid(tt.in); got != tt.want {
				t.Errorf("Valid(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestAlphabetExcludesAmbiguousLetters(t *testing.T) {
	for _, c := range "ILOU" {
		if strings.ContainsRune(crockford, c) {
			t.Errorf("alphabet must not contain %q", c)
		}
	}
	if len(crockford) != 32 {
		t.Errorf("alphabet length = %d, want 32", len(crockford))
	}
	seen := map[rune]bool{}
	for _, c := range crockford {
		if seen[c] {
			t.Errorf("alphabet contains %q twice", c)
		}
		seen[c] = true
	}
}

func TestConcurrentGenerationIsUniqueAndRaceFree(t *testing.T) {
	const workers, each = 16, 500

	var mu sync.Mutex
	seen := make(map[string]bool, workers*each)

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := make([]string, 0, each)
			for range each {
				local = append(local, New())
			}
			mu.Lock()
			defer mu.Unlock()
			for _, v := range local {
				if seen[v] {
					t.Errorf("duplicate identifier under concurrency: %q", v)
					return
				}
				seen[v] = true
			}
		}()
	}
	wg.Wait()

	if len(seen) != workers*each {
		t.Errorf("generated %d unique identifiers, want %d", len(seen), workers*each)
	}
}

func TestIncrCarries(t *testing.T) {
	var b [10]byte
	for i := range b {
		b[i] = 0xFF
	}
	incr(&b)
	for i, v := range b {
		if v != 0 {
			t.Errorf("after overflow byte %d = %d, want 0", i, v)
		}
	}

	b = [10]byte{}
	incr(&b)
	if b[9] != 1 {
		t.Errorf("least significant byte = %d, want 1", b[9])
	}

	b = [10]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0xFF}
	incr(&b)
	if b[9] != 0 || b[8] != 1 {
		t.Errorf("carry not propagated: %v", b)
	}
}

func TestFixedGenerator(t *testing.T) {
	f := NewFixed("a", "b")

	if got := f.New(); got != "a" {
		t.Errorf("first = %q, want a", got)
	}
	if got := f.New(); got != "b" {
		t.Errorf("second = %q, want b", got)
	}
	if got := f.New(); got != "a" {
		t.Errorf("third = %q, want a (cycled)", got)
	}
	if got := f.NewAt(time.Now()); got != "b" {
		t.Errorf("NewAt = %q, want b", got)
	}

	if got := NewFixed().New(); got != "" {
		t.Errorf("empty generator = %q, want an empty string", got)
	}
}

func TestFixedIsConcurrencySafe(t *testing.T) {
	f := NewFixed("a", "b", "c")
	errs := make(chan error, 8)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				got := f.New()
				// Relying on -race alone means this cannot fail under a plain
				// go test. Every value must be one the generator was given.
				if got != "a" && got != "b" && got != "c" {
					errs <- fmt.Errorf("New() = %q, want one of the declared values", got)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	// 800 calls over a three-value cycle must land back where it started.
	if got := f.New(); got != "c" {
		t.Errorf("after 800 calls New() = %q, want the cycle to have advanced exactly 800 times", got)
	}
}

func BenchmarkNew(b *testing.B) {
	for b.Loop() {
		_ = New()
	}
}
