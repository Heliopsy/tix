// SPDX-License-Identifier: AGPL-3.0-or-later

package migrations

import (
	"cmp"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/heliopsy/tix/internal/testenv"
)

// TestMain reports what the environment did not let this package check.
func TestMain(m *testing.M) { testenv.Main(m) }

func TestParseName(t *testing.T) {
	cases := []struct {
		file    string
		version int
		name    string
		wantErr string
	}{
		{"0001_init.sql", 1, "init", ""},
		{"0034_sync_source_secrets.sql", 34, "sync_source_secrets", ""},
		{"20261107153000_add_widgets.sql", 20261107153000, "add_widgets", ""},
		{"0035_next.sql", 0, "", "numbered migrations end at 0034"},
		{"9999_next.sql", 0, "", "numbered migrations end at 0034"},
		{"20261332153000_bad_month.sql", 0, "", "not a UTC time"},
		{"20261107256000_bad_hour.sql", 0, "", "not a UTC time"},
		{"202611071530_short.sql", 0, "", "must be named"},
		{"20261107153000_Upper.sql", 0, "", "must be named"},
		{"20261107153000.sql", 0, "", "must be named"},
		{"init.sql", 0, "", "must be named"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			v, name, err := ParseName(tc.file)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("ParseName(%q) error = %v, want one containing %q", tc.file, err, tc.wantErr)
				}
				return
			}
			if err != nil || v != tc.version || name != tc.name {
				t.Fatalf("ParseName(%q) = %d, %q, %v; want %d, %q", tc.file, v, name, err, tc.version, tc.name)
			}
		})
	}
}

func TestAllFromRejectsTwoMigrationsWithOneTimestamp(t *testing.T) {
	_, err := allFrom(fstest.MapFS{
		"20261107153000_a.sql": &fstest.MapFile{Data: []byte("SELECT 1;")},
		"20261107153000_b.sql": &fstest.MapFile{Data: []byte("SELECT 2;")},
	})
	if err == nil || !strings.Contains(err.Error(), "share version") {
		t.Fatalf("allFrom() error = %v, want the shared version named", err)
	}
}

func TestTimestampsSortAfterEveryNumberedMigration(t *testing.T) {
	got, err := allFrom(fstest.MapFS{
		"20260101000000_first_stamped.sql": &fstest.MapFile{Data: []byte("SELECT 3;")},
		"0034_last_numbered.sql":           &fstest.MapFile{Data: []byte("SELECT 2;")},
		"0001_init.sql":                    &fstest.MapFile{Data: []byte("SELECT 1;")},
	})
	if err != nil {
		t.Fatalf("allFrom() error = %v", err)
	}
	var order []string
	for _, m := range got {
		order = append(order, m.Name)
	}
	if want := []string{"init", "last_numbered", "first_stamped"}; !slices.Equal(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
}

// futureTolerance absorbs clock skew between a laptop and CI. A timestamp
// further ahead than this was not taken in UTC, or was typed by hand, and it
// would jump ahead of migrations that merge after it.
const futureTolerance = time.Hour

// futureMigrations names the migrations created after now plus the tolerance.
func futureMigrations(all []Migration, now time.Time) []string {
	var out []string
	for _, m := range all {
		if created, ok := m.Created(); ok && created.After(now.Add(futureTolerance)) {
			out = append(out, fmt.Sprintf("%d_%s", m.Version, m.Name))
		}
	}
	return out
}

func TestFutureMigrations(t *testing.T) {
	now := time.Date(2026, 11, 7, 15, 30, 0, 0, time.UTC)
	all := []Migration{
		{Version: 34, Name: "numbered"},
		{Version: 20261107153000, Name: "now"},
		{Version: 20261107162900, Name: "within_tolerance"},
		{Version: 20261107163001, Name: "past_tolerance"},
		{Version: 20270101000000, Name: "next_year"},
	}
	want := []string{"20261107163001_past_tolerance", "20270101000000_next_year"}
	if got := futureMigrations(all, now); !slices.Equal(got, want) {
		t.Fatalf("futureMigrations() = %v, want %v", got, want)
	}
}

func TestNoMigrationIsFromTheFuture(t *testing.T) {
	all, err := All()
	if err != nil {
		t.Fatalf("All() error = %v", err)
	}
	for _, name := range futureMigrations(all, time.Now().UTC()) {
		t.Errorf("migration %s is dated after now; versions are the UTC time the file was created "+
			"(just migration NAME writes one)", name)
	}
}

// outOfOrder compares the migration files on a branch with those on the base
// it merges into. Every migration on the base must still be there under the
// same name, because databases have recorded it, and every migration the
// branch adds must sort after all of the base's, because a database built
// from the base before the merge would otherwise apply them in a different
// order from a fresh one.
func outOfOrder(base, head []string) []string {
	var problems []string
	baseMax := 0
	for _, f := range base {
		if v, _, err := ParseName(f); err == nil {
			baseMax = max(baseMax, v)
		}
		if !slices.Contains(head, f) {
			problems = append(problems, fmt.Sprintf("%s is on the base but not on this branch: rebase, "+
				"and never rename or delete a migration that has merged", f))
		}
	}
	for _, f := range head {
		if slices.Contains(base, f) {
			continue
		}
		v, name, err := ParseName(f)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		if v <= baseMax {
			problems = append(problems, fmt.Sprintf("%s sorts before %d, the newest migration on the base: "+
				"rename it to a current UTC timestamp, %s_%s.sql", f, baseMax, time.Now().UTC().Format(TimestampLayout), name))
		}
	}
	return problems
}

func TestOutOfOrder(t *testing.T) {
	base := []string{"0001_init.sql", "0034_last.sql", "20261107100000_merged.sql"}
	cases := []struct {
		name string
		head []string
		want []string
	}{
		{"nothing added", base, nil},
		{"added after the base", append(slices.Clone(base), "20261107110000_mine.sql"), nil},
		{"two added after the base", append(slices.Clone(base), "20261107110000_a.sql", "20261107120000_b.sql"), nil},
		{"added before the newest on the base", append(slices.Clone(base), "20261107090000_stale.sql"),
			[]string{"20261107090000_stale.sql sorts before 20261107100000"}},
		{"a number squeezed into a gap", append(slices.Clone(base), "0017_gap.sql"),
			[]string{"0017_gap.sql sorts before 20261107100000"}},
		{"a merged migration renamed", []string{"0001_init.sql", "0034_last.sql", "20261107110000_merged.sql"},
			[]string{"20261107100000_merged.sql is on the base but not on this branch"}},
		{"base ahead of a stale branch", base[:2],
			[]string{"20261107100000_merged.sql is on the base but not on this branch"}},
		{"a malformed name", append(slices.Clone(base), "0035_numbered.sql"),
			[]string{"numbered migrations end at 0034"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := outOfOrder(base, tc.head)
			if len(got) != len(tc.want) {
				t.Fatalf("outOfOrder() = %q, want %d problems like %q", got, len(tc.want), tc.want)
			}
			for i := range got {
				if !strings.Contains(got[i], tc.want[i]) {
					t.Errorf("problem %d = %q, want it to contain %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// baseEnv names the git ref new migrations are compared with, origin/main by
// default. CI checks out the full history, so the ref is there.
const baseEnv = "TIX_MIGRATION_BASE"

func TestNewMigrationsSortAfterTheBase(t *testing.T) {
	ref := cmp.Or(os.Getenv(baseEnv), "origin/main")
	out, err := exec.Command("git", "ls-tree", "--name-only", ref, "--", ".").Output()
	if err != nil {
		testenv.Skip(t, testenv.Capability{
			Name: "migration base",
			Why:  fmt.Sprintf("git cannot list %s here (%v), so new migrations were not compared with it", ref, err),
			How:  "git fetch origin main, or set " + baseEnv + " to the branch this one merges into",
		})
	}
	base := sqlFiles(strings.Fields(string(out)))
	if len(base) == 0 {
		t.Fatalf("git lists no migrations on %s, so nothing was compared", ref)
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("listing the migrations: %v", err)
	}
	var head []string
	for _, e := range entries {
		head = append(head, e.Name())
	}
	for _, p := range outOfOrder(base, sqlFiles(head)) {
		t.Errorf("against %s: %s", ref, p)
	}
}

func sqlFiles(names []string) []string {
	var out []string
	for _, n := range names {
		if strings.HasSuffix(n, ".sql") {
			out = append(out, n)
		}
	}
	return out
}
