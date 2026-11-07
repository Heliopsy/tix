// SPDX-License-Identifier: AGPL-3.0-or-later

package migrationtest

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/heliopsy/tix/internal/store/migrations"
)

func TestRewindDerivesTheUndoOfEachStatement(t *testing.T) {
	all := []migrations.Migration{
		{Version: 1, Name: "init", SQL: `CREATE TABLE things (
  id    TEXT PRIMARY KEY,
  flag  INTEGER NOT NULL DEFAULT 0,
  label TEXT NOT NULL DEFAULT 'a, b' CHECK (label <> 'x')
);`},
		{Version: 2, Name: "more", SQL: `-- a comment; with a semicolon
ALTER TABLE things ADD COLUMN note TEXT;
CREATE UNIQUE INDEX idx_things_note ON things(note) WHERE note IS NOT NULL;
UPDATE things SET note = 'n';`},
		{Version: 20261107100000, Name: "drop", SQL: "ALTER TABLE things DROP COLUMN label;\nALTER TABLE things DROP COLUMN note;"},
		{Version: 20261107110000, Name: "other", SQL: "CREATE TABLE others (id TEXT);"},
	}
	got, err := rewind(all, 1)
	if err != nil {
		t.Fatalf("rewind() error = %v", err)
	}
	want := []string{
		"DROP TABLE others",
		"ALTER TABLE things ADD COLUMN note TEXT",
		"ALTER TABLE things ADD COLUMN label TEXT NOT NULL DEFAULT 'a, b' CHECK (label <> 'x')",
		"DROP INDEX idx_things_note",
		"ALTER TABLE things DROP COLUMN note",
		"DELETE FROM schema_migrations WHERE version > 1",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("rewind() =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestRewindStopsAtTheVersion(t *testing.T) {
	all := []migrations.Migration{
		{Version: 1, SQL: "CREATE TABLE a (id TEXT);"},
		{Version: 2, SQL: "CREATE TABLE b (id TEXT);"},
		{Version: 20261107100000, SQL: "CREATE TABLE c (id TEXT);"},
	}
	got, err := rewind(all, 2)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"DROP TABLE c", "DELETE FROM schema_migrations WHERE version > 2"}; !slices.Equal(got, want) {
		t.Fatalf("rewind() = %q, want %q", got, want)
	}
}

func TestRewindUsesTheMarkedStatements(t *testing.T) {
	all := []migrations.Migration{
		{Version: 1, SQL: "CREATE TABLE a (id TEXT, v TEXT);"},
		{Version: 2, SQL: `-- rewind: DROP VIEW a_view;
-- rewind: DROP TRIGGER a_touch
CREATE VIEW a_view AS SELECT id FROM a;
CREATE TRIGGER a_touch AFTER INSERT ON a BEGIN SELECT 1; END;`},
	}
	got, err := rewind(all, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"DROP VIEW a_view", "DROP TRIGGER a_touch", "DELETE FROM schema_migrations WHERE version > 1"}
	if !slices.Equal(got, want) {
		t.Fatalf("rewind() = %q, want %q", got, want)
	}
}

func TestRewindRefusesWhatItCannotDerive(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		want string
	}{
		{"a view", "CREATE VIEW v AS SELECT 1;", RewindMarker},
		{"a rename", "ALTER TABLE a RENAME TO b;", RewindMarker},
		{"a column nobody defined", "ALTER TABLE a DROP COLUMN ghost;", "which no earlier migration defines"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			all := []migrations.Migration{
				{Version: 1, SQL: "CREATE TABLE a (id TEXT);"},
				{Version: 20261107100000, Name: "hard", SQL: tc.sql},
			}
			_, err := rewind(all, 1)
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "20261107100000") {
				t.Fatalf("rewind() error = %v, want one naming the migration and %q", err, tc.want)
			}
		})
	}
}

// TestEveryMigrationRewindsToTheSchemaItStartedFrom derives the rewind to every
// version, runs some of them on a real database and migrates it forward again,
// and requires the schema each time to match one built fresh. A migration whose derived undo is wrong, or
// cannot be derived, fails here rather than in the test that first needs it.
func TestEveryMigrationRewindsToTheSchemaItStartedFrom(t *testing.T) {
	ctx := context.Background()
	all, err := migrations.All()
	if err != nil {
		t.Fatal(err)
	}
	fresh := open(t)
	if _, err := migrations.Run(ctx, fresh); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	want := shape(ctx, t, fresh)

	db := open(t)
	if _, err := migrations.Run(ctx, db); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	for _, m := range all {
		if _, err := Rewind(m.Version); err != nil {
			t.Fatalf("Rewind(%d) error = %v", m.Version, err)
		}
	}
	// Rewinding to the first version runs every derived undo once; the other
	// two cover a rewind that stops partway and one that undoes a single step.
	for _, v := range []int{all[0].Version, all[len(all)/2].Version, all[len(all)-2].Version} {
		stmts, err := Rewind(v)
		if err != nil {
			t.Fatalf("Rewind(%d) error = %v", v, err)
		}
		for _, stmt := range stmts {
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				t.Fatalf("rewinding to %d (%s): %v", v, stmt, err)
			}
		}
		if _, err := migrations.Run(ctx, db); err != nil {
			t.Fatalf("migrating forward from %d: %v", v, err)
		}
		if got := shape(ctx, t, db); got != want {
			t.Fatalf("after rewinding to %d and migrating, the schema differs from a fresh one:\n%s\nfresh:\n%s", v, got, want)
		}
	}
}

func open(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "tix.db")+
		"?_pragma=foreign_keys(1)&_pragma=synchronous(OFF)&_pragma=journal_mode(MEMORY)")
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// shape describes every table's columns and every index, independent of the
// order columns were added in.
func shape(ctx context.Context, t *testing.T, db *sql.DB) string {
	t.Helper()
	rows, err := db.QueryContext(ctx, `
		SELECT m.type, m.name, COALESCE(p.name, ''), COALESCE(p.type, ''), COALESCE(p."notnull", 0),
		       COALESCE(p.dflt_value, ''), COALESCE(p.pk, 0)
		FROM sqlite_master m LEFT JOIN pragma_table_info(m.name) p ON m.type = 'table'
		WHERE m.name NOT LIKE 'sqlite_%' AND m.name <> 'schema_migrations'
		ORDER BY m.type, m.name, p.name`)
	if err != nil {
		t.Fatalf("reading the schema: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var b strings.Builder
	for rows.Next() {
		var kind, name, col, typ, dflt string
		var notNull, pk int
		if err := rows.Scan(&kind, &name, &col, &typ, &notNull, &dflt, &pk); err != nil {
			t.Fatalf("scanning the schema: %v", err)
		}
		fmt.Fprintf(&b, "%s %s %s %s %d %s %d\n", kind, name, col, typ, notNull, dflt, pk)
	}
	return b.String()
}
