// SPDX-License-Identifier: AGPL-3.0-or-later

package sql

import "testing"

// concat copies both runs in order into a slice aliasing neither.
func TestConcatCopiesBothRuns(t *testing.T) {
	first, second := []any{1, 2}, []any{3}
	out := concat(first, second)
	if len(out) != 3 || out[0] != 1 || out[1] != 2 || out[2] != 3 {
		t.Fatalf("concat = %v", out)
	}
	out[0], out[2] = 9, 9
	if first[0] != 1 || second[0] != 3 {
		t.Fatalf("concat aliased its arguments: first %v, second %v", first, second)
	}
}

// Appending to the result must not write into spare capacity of first, which
// another builder may still hold.
func TestConcatDoesNotAliasSpareCapacity(t *testing.T) {
	backing := make([]any, 1, 4)
	backing[0] = "a"
	out := concat(backing, nil)
	out = append(out, "b")
	if extended := backing[:2]; extended[1] != nil {
		t.Fatalf("appending to concat's result wrote %v into first's backing array", extended[1])
	}
	out[0] = "z"
	if backing[0] != "a" {
		t.Fatal("concat's result shares first's backing array")
	}
	if len(concat(nil, nil)) != 0 {
		t.Fatal("concat of nothing is not empty")
	}
}
