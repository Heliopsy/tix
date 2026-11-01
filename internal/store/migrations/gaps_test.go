// SPDX-License-Identifier: AGPL-3.0-or-later

package migrations

import (
	"slices"
	"testing"
)

// skippedVersions are the migration numbers that will never exist, each with
// the reason. The runner replays only versions above the highest one a
// database has recorded, so a number skipped and filled in later is never
// applied to a database that already passed it. A gap is therefore either
// listed here, deliberately and for good, or it is a mistake.
var skippedVersions = map[int]string{
	17: "reserved for a branch that then landed as 0019; databases already carry 0018 and 0019, so 0017 can never be applied",
	21: "reserved for saved filters, which merge before 0023; the branch that adds 0021 removes this line",
	22: "reserved for calendar feed tokens, which merge before 0023 if they need a table at all; the branch that adds 0022 removes this line",
}

// missingVersions lists the numbers between the first and the last version
// that no migration carries.
func missingVersions(versions []int) []int {
	var out []int
	for i := 1; i < len(versions); i++ {
		for v := versions[i-1] + 1; v < versions[i]; v++ {
			out = append(out, v)
		}
	}
	return out
}

func TestMissingVersions(t *testing.T) {
	cases := []struct {
		name     string
		versions []int
		want     []int
	}{
		{"none", nil, nil},
		{"contiguous", []int{1, 2, 3}, nil},
		{"one skipped", []int{1, 2, 4}, []int{3}},
		{"a run skipped", []int{1, 5, 6}, []int{2, 3, 4}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := missingVersions(tc.versions); !slices.Equal(got, tc.want) {
				t.Fatalf("missingVersions(%v) = %v, want %v", tc.versions, got, tc.want)
			}
		})
	}
}

func TestMigrationVersionsHaveNoUnlistedGap(t *testing.T) {
	all, err := All()
	if err != nil {
		t.Fatalf("All() error = %v", err)
	}
	versions := make([]int, len(all))
	for i, m := range all {
		versions[i] = m.Version
	}
	if versions[0] != 1 {
		t.Errorf("the first migration is %d, want 1", versions[0])
	}
	missing := missingVersions(versions)
	for _, v := range missing {
		if _, ok := skippedVersions[v]; !ok {
			t.Errorf("migration %04d is missing between %d and %d. A database migrated past it would never "+
				"apply it, so number the next migration contiguously, or list %d in skippedVersions with the reason",
				v, versions[0], versions[len(versions)-1], v)
		}
	}
	for v, reason := range skippedVersions {
		if !slices.Contains(missing, v) {
			t.Errorf("skippedVersions lists %d (%s), which is not a gap; remove it", v, reason)
		}
		if reason == "" {
			t.Errorf("skippedVersions lists %d without a reason", v)
		}
	}
}
