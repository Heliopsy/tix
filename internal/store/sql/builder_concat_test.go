// SPDX-License-Identifier: AGPL-3.0-or-later

package sql

import "testing"

// concat copies both runs in order into a slice aliasing neither.
func TestConcatCopiesBothRuns(t *testing.T) {
	first, second := []any{1, 2}, []any{3}
	out := concat(first, second)
	if len(out) != 3 || out[0] != 1 || out[2] != 3 {
		t.Fatalf("concat = %v", out)
	}
	out[0] = 9
	if first[0] != 1 {
		t.Fatal("concat aliased its first argument")
	}
}
