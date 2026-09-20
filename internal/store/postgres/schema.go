package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/thereisnotime/tix/internal/clock"
	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/store/migrations"
	sqlb "github.com/thereisnotime/tix/internal/store/sql"
)

// appRoleName is the non-superuser role every transaction runs as, because
// row-level security has no effect on a superuser or on a table's owner.
const appRoleName = "tix_app"

// tenantSetting is the per-transaction setting the isolation policies read.
const tenantSetting = "tix.tenant_id"

// isolationPolicy is the name given to every tenant isolation policy.
const isolationPolicy = "tix_tenant_isolation"

// partitionMonthsBack and partitionMonthsAhead bound the partitions created up
// front, so writes never wait on partition creation.
const (
	partitionMonthsBack  = 12
	partitionMonthsAhead = 24
)

// boolColumns are stored as INTEGER in the portable schema and BOOLEAN here.
var boolColumns = map[string]bool{
	"workflows.builtin":        true,
	"field_defs.required":      true,
	"field_defs.indexed":       true,
	"webhook_endpoints.active": true,
}

// jsonColumns are stored as TEXT in the portable schema and JSONB here.
var jsonColumns = map[string]bool{
	"workflows.definition":          true,
	"field_defs.enum_options":       true,
	"field_defs.default_value":      true,
	"tasks.custom_fields":           true,
	"api_tokens.scopes":             true,
	"artifacts.payload":             true,
	"events.payload":                true,
	"audit_entries.before_state":    true,
	"audit_entries.after_state":     true,
	"webhook_endpoints.event_types": true,
	"sync_sources.config":           true,
}

// partitionedTables maps a high-volume table to the column it is ranged on, so
// retention drops a month rather than deleting rows one by one.
var partitionedTables = map[string]string{
	"events":        "occurred_at",
	"audit_entries": "occurred_at",
}

// constraintKeywords open a table constraint rather than a column definition.
var constraintKeywords = map[string]bool{
	"PRIMARY": true, "UNIQUE": true, "CHECK": true, "FOREIGN": true, "CONSTRAINT": true,
}

// runMigrations applies every migration above the current version, translating
// the portable schema to PostgreSQL types and adding the engine-specific
// partitioning, search vector and row-level security within the same step.
func runMigrations(ctx context.Context, db *sql.DB, clk clock.Clock) error {
	current, err := migrations.Current(ctx, db)
	if err != nil {
		return core.Internal("reading schema version").Wrap(err)
	}
	all, err := migrations.All()
	if err != nil {
		return core.Internal("reading migrations").Wrap(err)
	}
	for _, m := range all {
		if m.Version <= current {
			continue
		}
		if err := applyMigration(ctx, db, m, clk); err != nil {
			return err
		}
	}
	return nil
}

func applyMigration(ctx context.Context, db *sql.DB, m migrations.Migration, clk clock.Clock) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return core.Internal("migration %d: begin", m.Version).Wrap(err)
	}
	defer func() { _ = tx.Rollback() }()

	stmts, err := translate(m.SQL)
	if err != nil {
		return err
	}
	stmts = append(stmts, adjustments(clk.Now())...)
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return core.Internal("migration %d (%s): %s", m.Version, m.Name, truncate(stmt)).Wrap(err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)`,
		m.Version, time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		return core.Internal("migration %d: recording version", m.Version).Wrap(err)
	}
	if err := tx.Commit(); err != nil {
		return core.Internal("migration %d: commit", m.Version).Wrap(err)
	}
	return nil
}

// translate rewrites the portable schema into PostgreSQL DDL.
func translate(body string) ([]string, error) {
	var out []string
	for _, stmt := range migrations.Split(body) {
		if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(stmt)), "CREATE TABLE") {
			out = append(out, stmt)
			continue
		}
		translated, err := translateTable(stmt)
		if err != nil {
			return nil, err
		}
		out = append(out, translated)
	}
	return out, nil
}

func translateTable(stmt string) (string, error) {
	open := strings.Index(stmt, "(")
	shut := strings.LastIndex(stmt, ")")
	if open < 0 || shut < open {
		return "", core.Internal("cannot read the column list of %q", truncate(stmt))
	}
	head := strings.Fields(stmt[:open])
	if len(head) < 3 {
		return "", core.Internal("cannot read the table name of %q", truncate(stmt))
	}
	table := head[2]
	partCol := partitionedTables[table]

	var defs, extra []string
	for _, def := range splitTopLevel(stmt[open+1 : shut]) {
		def = strings.Join(strings.Fields(def), " ")
		if def == "" {
			continue
		}
		if constraintKeywords[strings.ToUpper(strings.Fields(def)[0])] {
			defs = append(defs, def)
			continue
		}
		col := strings.Fields(def)[0]
		rest := strings.TrimSpace(strings.TrimPrefix(def, col))
		if partCol != "" {
			if strings.Contains(strings.ToUpper(rest), "PRIMARY KEY AUTOINCREMENT") {
				defs = append(defs, col+" BIGSERIAL NOT NULL")
				extra = append(extra, "PRIMARY KEY ("+col+", "+partCol+")")
				continue
			}
			if before, found := dropWord(rest, "UNIQUE"); found {
				rest = before
				extra = append(extra, "UNIQUE ("+col+", "+partCol+")")
			}
		}
		defs = append(defs, col+" "+rewriteColumn(table, col, rest))
	}
	defs = append(defs, extra...)

	out := "CREATE TABLE " + table + " (\n  " + strings.Join(defs, ",\n  ") + "\n)"
	if partCol != "" {
		out += " PARTITION BY RANGE (" + partCol + ")"
	}
	return out, nil
}

func rewriteColumn(table, col, rest string) string {
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return rest
	}
	base := strings.ToUpper(fields[0])
	tail := strings.TrimSpace(strings.TrimPrefix(rest, fields[0]))
	if base == "INTEGER" && strings.HasPrefix(strings.ToUpper(tail), "PRIMARY KEY AUTOINCREMENT") {
		rem := strings.TrimSpace(tail[len("PRIMARY KEY AUTOINCREMENT"):])
		return strings.TrimSpace("BIGSERIAL PRIMARY KEY " + rem)
	}
	pg := columnType(table, col, base)
	if pg == "BOOLEAN" {
		tail = strings.ReplaceAll(tail, "DEFAULT 0", "DEFAULT FALSE")
		tail = strings.ReplaceAll(tail, "DEFAULT 1", "DEFAULT TRUE")
	}
	return strings.TrimSpace(pg + " " + tail)
}

func columnType(table, col, base string) string {
	switch {
	case isTimestampColumn(col):
		return "TIMESTAMPTZ"
	case boolColumns[table+"."+col]:
		return "BOOLEAN"
	case jsonColumns[table+"."+col]:
		return "JSONB"
	}
	switch base {
	case "INTEGER":
		return "BIGINT"
	case "BLOB":
		return "BYTEA"
	}
	return base
}

// isTimestampColumn names the convention the portable schema follows: every
// instant is a column ending in _at, plus the one lock expiry called _until.
func isTimestampColumn(col string) bool {
	return strings.HasSuffix(col, "_at") || strings.HasSuffix(col, "_until")
}

// dropWord removes a standalone keyword from a column definition.
func dropWord(def, word string) (string, bool) {
	fields := strings.Fields(def)
	out := make([]string, 0, len(fields))
	found := false
	for _, f := range fields {
		if strings.EqualFold(f, word) {
			found = true
			continue
		}
		out = append(out, f)
	}
	return strings.Join(out, " "), found
}

// splitTopLevel splits on commas that are outside parentheses and string literals.
func splitTopLevel(body string) []string {
	var (
		out   []string
		cur   strings.Builder
		depth int
		inStr bool
	)
	for i := 0; i < len(body); i++ {
		c := body[i]
		switch {
		case inStr:
			if c == '\'' {
				inStr = false
			}
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
		cur.WriteByte(c)
	}
	out = append(out, cur.String())
	return out
}

// adjustments are the statements that exist only on this engine: the monthly
// partitions, the search vector and the row-level security policies.
func adjustments(now time.Time) []string {
	var out []string
	for table := range partitionedTables {
		out = append(out, partitionStatements(table, now)...)
	}
	out = append(out,
		"ALTER TABLE tasks ADD COLUMN search_tsv tsvector GENERATED ALWAYS AS "+
			"(to_tsvector('simple', coalesce(title, '') || ' ' || coalesce(body, ''))) STORED",
		"CREATE INDEX idx_tasks_search ON tasks USING GIN (search_tsv)",
	)
	out = append(out, rowLevelSecurity()...)
	return out
}

func partitionStatements(table string, now time.Time) []string {
	start := time.Date(now.UTC().Year(), now.UTC().Month(), 1, 0, 0, 0, 0, time.UTC).
		AddDate(0, -partitionMonthsBack, 0)
	out := make([]string, 0, partitionMonthsBack+partitionMonthsAhead+2)
	for i := 0; i < partitionMonthsBack+partitionMonthsAhead; i++ {
		month := start.AddDate(0, i, 0)
		out = append(out, partitionStatement(table, month))
	}
	out = append(out, fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s_default PARTITION OF %s DEFAULT", table, table))
	return out
}

func partitionStatement(table string, month time.Time) string {
	return fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM ('%s') TO ('%s')",
		partitionName(table, month), table,
		month.Format(time.RFC3339), month.AddDate(0, 1, 0).Format(time.RFC3339))
}

func partitionName(table string, month time.Time) string {
	return fmt.Sprintf("%s_p%04d%02d", table, month.Year(), int(month.Month()))
}

func rowLevelSecurity() []string {
	var out []string
	predicate := fmt.Sprintf("%s = current_setting('%s', true)", sqlb.TenantColumn, tenantSetting)
	for _, table := range sqlb.ScopedTables() {
		out = append(out,
			"ALTER TABLE "+table+" ENABLE ROW LEVEL SECURITY",
			"ALTER TABLE "+table+" FORCE ROW LEVEL SECURITY",
			fmt.Sprintf("CREATE POLICY %s ON %s USING (%s) WITH CHECK (%s)",
				isolationPolicy, table, predicate, predicate),
		)
	}
	return out
}

// EnsurePartitions creates the monthly partitions covering every month from now
// through the given instant, so a write never finds its month missing.
func (s *Store) EnsurePartitions(ctx context.Context, through time.Time) error {
	now := s.clock.Now().UTC()
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	for !month.After(through.UTC()) {
		for table := range partitionedTables {
			if _, err := s.db.ExecContext(ctx, partitionStatement(table, month)); err != nil {
				return mapErr(err, "creating the %s partition for %s", table, month.Format("2006-01"))
			}
		}
		month = month.AddDate(0, 1, 0)
	}
	return nil
}

// DropPartitionsBefore drops whole monthly partitions of the high-volume tables
// that end at or before the cutoff, which is what makes retention cheap here.
func (s *Store) DropPartitionsBefore(ctx context.Context, cutoff time.Time) (int, error) {
	dropped := 0
	for table := range partitionedTables {
		rows, err := s.db.QueryContext(ctx,
			`SELECT c.relname FROM pg_class c
			   JOIN pg_inherits i ON i.inhrelid = c.oid
			   JOIN pg_class p ON p.oid = i.inhparent
			  WHERE p.relname = $1 AND c.relname <> $2`, table, table+"_default")
		if err != nil {
			return dropped, mapErr(err, "listing partitions of %q", table)
		}
		var names []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				_ = rows.Close()
				return dropped, mapErr(err, "listing partitions of %q", table)
			}
			names = append(names, name)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return dropped, mapErr(err, "listing partitions of %q", table)
		}
		_ = rows.Close()

		for _, name := range names {
			end, ok := partitionEnd(table, name)
			if !ok || end.After(cutoff.UTC()) {
				continue
			}
			if _, err := s.db.ExecContext(ctx, "DROP TABLE "+name); err != nil {
				return dropped, mapErr(err, "dropping partition %q", name)
			}
			dropped++
		}
	}
	return dropped, nil
}

func partitionEnd(table, name string) (time.Time, bool) {
	suffix := strings.TrimPrefix(name, table+"_p")
	if len(suffix) != 6 {
		return time.Time{}, false
	}
	month, err := time.Parse("200601", suffix)
	if err != nil {
		return time.Time{}, false
	}
	return month.AddDate(0, 1, 0), true
}

// grantAppRole creates the unprivileged role transactions run as. It is best
// effort: a deployment whose login role may not create roles keeps row-level
// security through FORCE ROW LEVEL SECURITY alone.
func (s *Store) grantAppRole(ctx context.Context) {
	stmts := []string{
		`DO $$ BEGIN
		   IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '` + appRoleName + `') THEN
		     CREATE ROLE ` + appRoleName + ` NOLOGIN NOBYPASSRLS;
		   END IF;
		 END $$`,
		`GRANT ` + appRoleName + ` TO CURRENT_USER`,
		`GRANT USAGE ON SCHEMA public TO ` + appRoleName,
		`GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO ` + appRoleName,
		`GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO ` + appRoleName,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return
		}
	}
}

// roleUsable reports whether transactions can drop into the application role.
func (s *Store) roleUsable(ctx context.Context) bool {
	var ok bool
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE((SELECT pg_has_role(current_user, oid, 'MEMBER') FROM pg_roles WHERE rolname = $1), false)`,
		appRoleName).Scan(&ok)
	return err == nil && ok
}

func truncate(s string) string {
	const max = 200
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// migrationsAll returns the portable schema as one body, for the translator's tests.
func migrationsAll() (string, error) {
	all, err := migrations.All()
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, m := range all {
		sb.WriteString(m.SQL)
		sb.WriteString("\n")
	}
	return sb.String(), nil
}
