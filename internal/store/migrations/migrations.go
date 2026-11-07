// SPDX-License-Identifier: AGPL-3.0-or-later

// Package migrations holds the embedded schema and the forward-only runner.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed *.sql
var files embed.FS

// LastNumbered is the highest four-digit version. Every later migration is
// named by the UTC time it was created, which sorts after all of them, so a
// branch never has to reserve a number another branch might also take.
const LastNumbered = 34

// TimestampLayout is the version format of every migration after LastNumbered.
const TimestampLayout = "20060102150405"

var fileName = regexp.MustCompile(`^([0-9]+)_([a-z0-9_]+)\.sql$`)

// Migration is one schema step, identified by its version.
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// Created returns the UTC time a timestamped migration was created, and false
// for one of the numbered migrations that predate timestamps.
func (m Migration) Created() (time.Time, bool) {
	if m.Version <= LastNumbered {
		return time.Time{}, false
	}
	t, err := time.Parse(TimestampLayout, strconv.Itoa(m.Version))
	return t, err == nil
}

// ParseName reads the version and name out of a migration's file name. A
// numbered version is accepted only up to LastNumbered; anything newer must
// be a valid UTC timestamp in TimestampLayout.
func ParseName(file string) (int, string, error) {
	m := fileName.FindStringSubmatch(file)
	if m == nil {
		return 0, "", fmt.Errorf("migration %q must be named YYYYMMDDHHMMSS_name.sql in lowercase", file)
	}
	digits, name := m[1], m[2]
	switch len(digits) {
	case 4:
		v, _ := strconv.Atoi(digits)
		if v > LastNumbered {
			return 0, "", fmt.Errorf("migration %q: numbered migrations end at %04d, name it by its UTC creation time "+
				"(just migration %s)", file, LastNumbered, name)
		}
		return v, name, nil
	case len(TimestampLayout):
		if _, err := time.Parse(TimestampLayout, digits); err != nil {
			return 0, "", fmt.Errorf("migration %q: %s is not a UTC time in the form YYYYMMDDHHMMSS", file, digits)
		}
		v, err := strconv.Atoi(digits)
		if err != nil {
			return 0, "", fmt.Errorf("migration %q: %w", file, err)
		}
		return v, name, nil
	}
	return 0, "", fmt.Errorf("migration %q must be named YYYYMMDDHHMMSS_name.sql", file)
}

// All returns every migration in version order.
func All() ([]Migration, error) { return allFrom(files) }

func allFrom(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("reading migrations: %w", err)
	}

	var out []Migration
	seen := map[int]string{}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		v, rest, err := ParseName(name)
		if err != nil {
			return nil, err
		}
		if prev, dup := seen[v]; dup {
			return nil, fmt.Errorf("migrations %q and %q share version %d", prev, name, v)
		}
		seen[v] = name

		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("reading %q: %w", name, err)
		}
		out = append(out, Migration{Version: v, Name: rest, SQL: string(body)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// Latest returns the highest migration version.
func Latest() (int, error) {
	all, err := All()
	if err != nil {
		return 0, err
	}
	if len(all) == 0 {
		return 0, nil
	}
	return all[len(all)-1].Version, nil
}

const createVersionTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version    INTEGER PRIMARY KEY,
  applied_at TEXT NOT NULL
)`

// Current returns the highest applied version, or zero on an empty database.
func Current(ctx context.Context, db *sql.DB) (int, error) {
	if _, err := db.ExecContext(ctx, createVersionTable); err != nil {
		return 0, fmt.Errorf("creating schema_migrations: %w", err)
	}
	var v sql.NullInt64
	if err := db.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&v); err != nil {
		return 0, fmt.Errorf("reading schema version: %w", err)
	}
	return int(v.Int64), nil
}

// Applied returns every version the database has recorded.
func Applied(ctx context.Context, db *sql.DB) (map[int]bool, error) {
	if _, err := db.ExecContext(ctx, createVersionTable); err != nil {
		return nil, fmt.Errorf("creating schema_migrations: %w", err)
	}
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("reading applied migrations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[int]bool{}
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("reading applied migrations: %w", err)
		}
		out[int(v)] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading applied migrations: %w", err)
	}
	return out, nil
}

// Pending returns every migration the database has not recorded, in version
// order. It looks at each version rather than at the highest, so a migration
// that merged with an earlier timestamp than one already applied still runs.
func Pending(ctx context.Context, db *sql.DB) ([]Migration, error) {
	all, err := All()
	if err != nil {
		return nil, err
	}
	return pendingOf(ctx, db, all)
}

func pendingOf(ctx context.Context, db *sql.DB, all []Migration) ([]Migration, error) {
	applied, err := Applied(ctx, db)
	if err != nil {
		return nil, err
	}
	var out []Migration
	for _, m := range all {
		if !applied[m.Version] {
			out = append(out, m)
		}
	}
	return out, nil
}

// Run applies every pending migration, each in its own transaction.
// Migrations are forward-only; there is no down path.
func Run(ctx context.Context, db *sql.DB) (applied int, err error) {
	all, err := All()
	if err != nil {
		return 0, err
	}
	return run(ctx, db, all)
}

func run(ctx context.Context, db *sql.DB, all []Migration) (applied int, err error) {
	pending, err := pendingOf(ctx, db, all)
	if err != nil {
		return 0, err
	}
	for _, m := range pending {
		if err := apply(ctx, db, m); err != nil {
			return applied, err
		}
		applied++
	}
	return applied, nil
}

func apply(ctx context.Context, db *sql.DB, m Migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migration %d: begin: %w", m.Version, err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, stmt := range Split(m.SQL) {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migration %d (%s): %w\nstatement: %s", m.Version, m.Name, err, truncate(stmt))
		}
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
		m.Version, time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("migration %d: recording version: %w", m.Version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migration %d: commit: %w", m.Version, err)
	}
	return nil
}

// Split separates a migration into statements, ignoring semicolons inside
// string literals and line comments.
func Split(body string) []string {
	var (
		out    []string
		cur    strings.Builder
		inStr  bool
		inLine bool
	)
	for i := 0; i < len(body); i++ {
		c := body[i]
		switch {
		case inLine:
			if c == '\n' {
				inLine = false
				cur.WriteByte(c)
			}
			continue
		case inStr:
			cur.WriteByte(c)
			if c == '\'' {
				inStr = false
			}
			continue
		case c == '-' && i+1 < len(body) && body[i+1] == '-':
			inLine = true
			continue
		case c == '\'':
			inStr = true
			cur.WriteByte(c)
			continue
		case c == ';':
			if s := strings.TrimSpace(cur.String()); s != "" {
				out = append(out, s)
			}
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	if s := strings.TrimSpace(cur.String()); s != "" {
		out = append(out, s)
	}
	return out
}

func truncate(s string) string {
	const max = 200
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
