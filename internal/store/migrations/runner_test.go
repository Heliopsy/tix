// SPDX-License-Identifier: AGPL-3.0-or-later

package migrations

import (
	"context"
	"database/sql"
	"slices"
	"testing"
)

// Two migrations as two branches would write them: the second depends on the
// first, so applying them out of order fails rather than passing quietly.
var (
	widgets = Migration{Version: 20261107100000, Name: "widgets",
		SQL: "CREATE TABLE widgets (id TEXT PRIMARY KEY);"}
	widgetColour = Migration{Version: 20261107110000, Name: "widget_colour",
		SQL: "ALTER TABLE widgets ADD COLUMN colour TEXT NOT NULL DEFAULT '';"}
)

func withStamped(t *testing.T, extra ...Migration) []Migration {
	t.Helper()
	all, err := All()
	if err != nil {
		t.Fatalf("All() error = %v", err)
	}
	return append(all, extra...)
}

func versions(all []Migration) []int {
	out := make([]int, len(all))
	for i, m := range all {
		out[i] = m.Version
	}
	return out
}

// recorded returns the versions in the order they were applied.
func recorded(ctx context.Context, t *testing.T, db *sql.DB) []int {
	t.Helper()
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations ORDER BY rowid`)
	if err != nil {
		t.Fatalf("reading schema_migrations: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scanning a version: %v", err)
		}
		out = append(out, v)
	}
	return out
}

func TestRunRecordsEveryVersionOnAFreshDatabase(t *testing.T) {
	ctx := context.Background()
	db := open(t)
	if _, err := Run(ctx, db); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	all, err := All()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := recorded(ctx, t, db), versions(all); !slices.Equal(got, want) {
		t.Fatalf("recorded %v, want every migration %v", got, want)
	}
}

func TestNumberedThenStampedOnAFreshDatabase(t *testing.T) {
	ctx := context.Background()
	db := open(t)
	all := withStamped(t, widgetColour, widgets)
	slices.SortFunc(all, func(a, b Migration) int { return a.Version - b.Version })
	if _, err := run(ctx, db, all); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	got := recorded(ctx, t, db)
	if want := versions(all); !slices.Equal(got, want) {
		t.Fatalf("applied %v, want %v", got, want)
	}
	if got[len(got)-3] != LastNumbered {
		t.Fatalf("the migration before the stamped ones is %d, want %d", got[len(got)-3], LastNumbered)
	}
}

func TestStampedOnADatabaseAtTheLastNumbered(t *testing.T) {
	ctx := context.Background()
	db := open(t)
	runThrough(ctx, t, db, LastNumbered)
	if _, err := db.ExecContext(ctx, `INSERT INTO tenants (id, key, name, created_at, updated_at)
		VALUES ('t1', 'acme', 'Acme', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	applied, err := run(ctx, db, withStamped(t, widgets, widgetColour))
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if applied != 2 {
		t.Fatalf("applied %d migrations to a database at %04d, want the 2 stamped ones", applied, LastNumbered)
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tenants`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("tenants after the upgrade = %d, %v; want the 1 already there", n, err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO widgets (id, colour) VALUES ('w1', 'red')`); err != nil {
		t.Fatalf("the stamped migrations did not take effect: %v", err)
	}
}

// A database that already ran the later of two parallel branches' migrations
// still gets the earlier one when its branch merges second.
func TestAnEarlierStampMergedLaterStillApplies(t *testing.T) {
	ctx := context.Background()
	db := open(t)
	other := Migration{Version: 20261107120000, Name: "gadgets", SQL: "CREATE TABLE gadgets (id TEXT PRIMARY KEY);"}
	if _, err := run(ctx, db, withStamped(t, other)); err != nil {
		t.Fatalf("run() with the first branch error = %v", err)
	}

	applied, err := run(ctx, db, withStamped(t, widgets, other))
	if err != nil {
		t.Fatalf("run() with both branches error = %v", err)
	}
	if applied != 1 {
		t.Fatalf("applied %d, want the one migration from the branch that merged second", applied)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO widgets (id) VALUES ('w1')`); err != nil {
		t.Fatalf("the earlier stamp was skipped: %v", err)
	}
	if again, err := run(ctx, db, withStamped(t, widgets, other)); err != nil || again != 0 {
		t.Fatalf("a second run applied %d, %v; want nothing", again, err)
	}
}
