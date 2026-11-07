// SPDX-License-Identifier: AGPL-3.0-or-later

// Package migrationtest takes a migrated database back to an earlier version,
// for tests of what a migration does to a database that already holds rows.
//
// The statements are derived from the migrations themselves. Tests used to
// carry the list by hand, and every pull request adding a migration had to
// extend it in each of them.
package migrationtest

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/heliopsy/tix/internal/store/migrations"
)

// RewindMarker starts a comment line in a migration that gives one statement
// undoing it, for a migration whose undo cannot be derived. When a migration
// has any, they replace the derived statements and run in the order written.
const RewindMarker = "-- rewind:"

var (
	createTable = regexp.MustCompile(`(?is)^\s*CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)\s*\((.*)\)\s*$`)
	createIndex = regexp.MustCompile(`(?is)^\s*CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)`)
	addColumn   = regexp.MustCompile(`(?is)^\s*ALTER\s+TABLE\s+(\w+)\s+ADD\s+COLUMN\s+(\w+)\s+(.*)$`)
	dropColumn  = regexp.MustCompile(`(?is)^\s*ALTER\s+TABLE\s+(\w+)\s+DROP\s+COLUMN\s+(\w+)\s*$`)
	dataChange  = regexp.MustCompile(`(?is)^\s*(UPDATE|INSERT|DELETE)\s`)
)

// Rewind returns the portable statements that undo every migration above
// version, newest first, followed by the removal of their schema_migrations
// rows, so the next migration run applies them again.
func Rewind(version int) ([]string, error) {
	all, err := migrations.All()
	if err != nil {
		return nil, err
	}
	return rewind(all, version)
}

func rewind(all []migrations.Migration, version int) ([]string, error) {
	var out []string
	for i := len(all) - 1; i >= 0; i-- {
		if all[i].Version <= version {
			break
		}
		stmts, err := undo(all[i], all[:i])
		if err != nil {
			return nil, err
		}
		out = append(out, stmts...)
	}
	return append(out, fmt.Sprintf("DELETE FROM schema_migrations WHERE version > %d", version)), nil
}

// undo returns the statements reversing one migration, given the ones before
// it, which hold the definition of any column it drops.
func undo(m migrations.Migration, earlier []migrations.Migration) ([]string, error) {
	if given := markedUndo(m.SQL); len(given) > 0 {
		return given, nil
	}
	stmts := migrations.Split(m.SQL)
	var out []string
	for i := len(stmts) - 1; i >= 0; i-- {
		stmt := stmts[i]
		switch {
		case createTable.MatchString(stmt):
			out = append(out, "DROP TABLE "+createTable.FindStringSubmatch(stmt)[1])
		case createIndex.MatchString(stmt):
			out = append(out, "DROP INDEX "+createIndex.FindStringSubmatch(stmt)[1])
		case addColumn.MatchString(stmt):
			c := addColumn.FindStringSubmatch(stmt)
			out = append(out, "ALTER TABLE "+c[1]+" DROP COLUMN "+c[2])
		case dropColumn.MatchString(stmt):
			c := dropColumn.FindStringSubmatch(stmt)
			def, ok := columnDef(earlier, c[1], c[2])
			if !ok {
				return nil, fmt.Errorf("migration %d (%s) drops %s.%s, which no earlier migration defines",
					m.Version, m.Name, c[1], c[2])
			}
			out = append(out, "ALTER TABLE "+c[1]+" ADD COLUMN "+c[2]+" "+def)
		case dataChange.MatchString(stmt):
		default:
			return nil, fmt.Errorf("migration %d (%s): cannot derive how to undo %q; "+
				"add %q comment lines giving the statements that do", m.Version, m.Name, firstLine(stmt), RewindMarker)
		}
	}
	return out, nil
}

func markedUndo(body string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), RewindMarker); ok {
			if stmt := strings.TrimSuffix(strings.TrimSpace(rest), ";"); stmt != "" {
				out = append(out, stmt)
			}
		}
	}
	return out
}

// columnDef finds the latest definition of a column among the migrations, as
// written after its name in a CREATE TABLE or an ADD COLUMN.
func columnDef(all []migrations.Migration, table, col string) (string, bool) {
	var def string
	found := false
	for _, m := range all {
		for _, stmt := range migrations.Split(m.SQL) {
			if c := addColumn.FindStringSubmatch(stmt); c != nil && strings.EqualFold(c[1], table) && strings.EqualFold(c[2], col) {
				def, found = strings.Join(strings.Fields(c[3]), " "), true
				continue
			}
			c := createTable.FindStringSubmatch(stmt)
			if c == nil || !strings.EqualFold(c[1], table) {
				continue
			}
			for _, part := range splitTopLevel(c[2]) {
				fields := strings.Fields(part)
				if len(fields) > 1 && strings.EqualFold(fields[0], col) {
					def, found = strings.Join(fields[1:], " "), true
				}
			}
		}
	}
	return def, found
}

// splitTopLevel splits a column list on the commas outside parentheses and
// string literals.
func splitTopLevel(body string) []string {
	var (
		out   []string
		cur   strings.Builder
		depth int
		inStr bool
	)
	for _, c := range body {
		switch {
		case inStr:
			inStr = c != '\''
		case c == '\'':
			inStr = true
		case c == '(':
			depth++
		case c == ')':
			depth--
		case c == ',' && depth == 0:
			out = append(out, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(c)
	}
	return append(out, cur.String())
}

func firstLine(stmt string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(stmt), "\n")
	return line
}
