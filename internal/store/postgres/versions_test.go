// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/store/migrations"
)

func appliedVersions(t *testing.T, s *Store) []int {
	t.Helper()
	rows, err := s.db.QueryContext(context.Background(), `SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatalf("reading schema_migrations: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []int
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scanning a version: %v", err)
		}
		out = append(out, int(v))
	}
	return out
}

// SQLite records exactly migrations.All, which the migrations package holds
// it to; this holds PostgreSQL to the same list.
func TestPostgresRecordsTheSameVersionsAsSQLite(t *testing.T) {
	s, _ := newStore(t)
	all, err := migrations.All()
	if err != nil {
		t.Fatal(err)
	}
	var want []int
	for _, m := range all {
		want = append(want, m.Version)
	}
	if got := appliedVersions(t, s); !slices.Equal(got, want) {
		t.Fatalf("recorded %v, want %v", got, want)
	}
}

// A database created before timestamped versions has a 32-bit version
// column; Migrate widens it so a version such as 20261107153000 fits.
func TestMigrateWidensAnIntegerVersionColumn(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, freshDSN(t), clock.NewFakeAt())
	if _, err := s.db.ExecContext(ctx,
		`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		t.Fatalf("creating the old table: %v", err)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	var kind string
	if err := s.db.QueryRowContext(ctx, `SELECT data_type FROM information_schema.columns
		WHERE table_name = 'schema_migrations' AND column_name = 'version'`).Scan(&kind); err != nil {
		t.Fatal(err)
	}
	if kind != "bigint" {
		t.Fatalf("schema_migrations.version is %s, want bigint", kind)
	}
	stamp := 20261107153000
	if _, err := s.db.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)`,
		stamp, time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatalf("recording a timestamped version: %v", err)
	}
}

// A table created by a later migration gets its row-level security with that
// migration, found by reading the migrations rather than from a list.
func TestIntroducedInReadsTheMigrations(t *testing.T) {
	for table, want := range map[string]int{
		"tenants":          1,
		"tenant_domains":   1,
		"ssh_keys":         5,
		"inbox_deliveries": 24,
		"no_such_table":    0,
	} {
		if got := introducedIn(table); got != want {
			t.Errorf("introducedIn(%q) = %d, want %d", table, got, want)
		}
	}
}
