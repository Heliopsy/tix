// SPDX-License-Identifier: AGPL-3.0-or-later

// Package migrations holds the embedded schema and the forward-only runner.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed *.sql
var files embed.FS

// Migration is one numbered schema step.
type Migration struct {
	Version int
	Name    string
	SQL     string
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
		numPart, rest, ok := strings.Cut(strings.TrimSuffix(name, ".sql"), "_")
		if !ok {
			return nil, fmt.Errorf("migration %q must be named NNNN_name.sql", name)
		}
		v, err := strconv.Atoi(numPart)
		if err != nil {
			return nil, fmt.Errorf("migration %q has a non-numeric version: %w", name, err)
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

// Run applies every migration above the current version, each in its own
// transaction. Migrations are forward-only; there is no down path.
func Run(ctx context.Context, db *sql.DB) (applied int, err error) {
	current, err := Current(ctx, db)
	if err != nil {
		return 0, err
	}
	all, err := All()
	if err != nil {
		return 0, err
	}

	for _, m := range all {
		if m.Version <= current {
			continue
		}
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
