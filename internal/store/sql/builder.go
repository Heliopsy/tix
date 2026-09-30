// SPDX-License-Identifier: AGPL-3.0-or-later

// Package sql builds tenant-scoped statements shared by every engine.
package sql

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// Dialect selects placeholder syntax.
type Dialect int

// Dialects.
const (
	SQLite Dialect = iota
	Postgres
)

// TenantColumn is the column every tenant-owned table carries.
const TenantColumn = "tenant_id"

// unscoped names the tables that legitimately have no tenant column, each with
// the reason it has none. It is the only exemption from tenant scoping, and the
// schema guards read it as an allow-list rather than as the definition of what
// is scoped: a table carrying tenant_id and named here is a bug they fail on.
var unscoped = map[string]string{
	"tenants":           "a tenant row is the tenant, so it cannot belong to one",
	"users":             "a user is one global identity that memberships attach to tenants",
	"schema_migrations": "schema state belongs to the deployment, not to a tenant",
	"servers":           "a server process serves every tenant, so its row belongs to none of them",
}

// UnscopedTables returns the exemption list with its reasons, for the guards
// that enumerate the live schema and need to know what may hold no tenant_id.
func UnscopedTables() map[string]string {
	out := make(map[string]string, len(unscoped))
	for k, v := range unscoped {
		out[k] = v
	}
	return out
}

// Builder composes a statement for one tenant. There is no constructor that
// omits the scope, so an unscoped query cannot be built by accident.
type Builder struct {
	dialect Dialect
	scope   core.TenantScope
	table   string

	columns []string
	wheres  []string
	order   []string
	limit   int
	joins   []string
	sets    []string

	// A statement renders joins, then assignments, then predicates, and each
	// run of arguments is kept where it is bound rather than recovered from
	// one slice by counting placeholders.
	joinArgs []any
	setArgs  []any
	args     []any
}

// New starts a statement against table, scoped to one tenant.
func New(d Dialect, scope core.TenantScope, table string) (*Builder, error) {
	if !scope.Valid() && IsScoped(table) {
		return nil, core.Invalid("refusing to build a query on %q without a tenant scope", table)
	}
	b := &Builder{dialect: d, scope: scope, table: table}
	if IsScoped(table) {
		b.wheres = append(b.wheres, qualify(table, TenantColumn)+" = ?")
		b.args = append(b.args, scope.TenantID)
	}
	return b, nil
}

// MustNew is New for statements on tables known at compile time to be scoped.
func MustNew(d Dialect, scope core.TenantScope, table string) *Builder {
	b, err := New(d, scope, table)
	if err != nil {
		panic(err)
	}
	return b
}

// Select names the columns to read.
func (b *Builder) Select(cols ...string) *Builder {
	b.columns = append(b.columns, cols...)
	return b
}

// Join adds a join clause. The caller supplies the ON condition.
func (b *Builder) Join(clause string, args ...any) *Builder {
	b.joins = append(b.joins, clause)
	b.joinArgs = append(b.joinArgs, args...)
	return b
}

// Where adds a condition. The tenant predicate is always present already.
func (b *Builder) Where(cond string, args ...any) *Builder {
	b.wheres = append(b.wheres, cond)
	b.args = append(b.args, args...)
	return b
}

// WhereIn adds an IN condition, or a never-true condition when values is empty.
func (b *Builder) WhereIn(col string, values []string) *Builder {
	if len(values) == 0 {
		b.wheres = append(b.wheres, "1 = 0")
		return b
	}
	marks := make([]string, len(values))
	for i, v := range values {
		marks[i] = "?"
		b.args = append(b.args, v)
	}
	b.wheres = append(b.wheres, col+" IN ("+strings.Join(marks, ", ")+")")
	return b
}

// WhereNotIn adds a NOT IN condition, and nothing at all when values is empty,
// since excluding no value excludes no row. A NULL column is kept: SQL's NOT IN
// would drop it, and a task with no assignee is not a task assigned to somebody
// the filter excluded.
func (b *Builder) WhereNotIn(col string, values []string) *Builder {
	if len(values) == 0 {
		return b
	}
	marks := make([]string, len(values))
	for i, v := range values {
		marks[i] = "?"
		b.args = append(b.args, v)
	}
	b.wheres = append(b.wheres,
		"("+col+" IS NULL OR "+col+" NOT IN ("+strings.Join(marks, ", ")+"))")
	return b
}

// Set adds an assignment for an update.
func (b *Builder) Set(col string, value any) *Builder {
	b.sets = append(b.sets, col+" = ?")
	b.setArgs = append(b.setArgs, value)
	return b
}

// SetExpr adds an assignment whose value is an expression.
func (b *Builder) SetExpr(col, expr string, args ...any) *Builder {
	b.sets = append(b.sets, col+" = "+expr)
	b.setArgs = append(b.setArgs, args...)
	return b
}

// OrderBy appends an ordering. Callers pass validated column names only.
func (b *Builder) OrderBy(col string, dir core.SortDirection) *Builder {
	d := "ASC"
	if dir == core.Descending {
		d = "DESC"
	}
	b.order = append(b.order, col+" "+d)
	return b
}

// Limit caps the rows returned.
func (b *Builder) Limit(n int) *Builder {
	b.limit = n
	return b
}

// Keyset applies a cursor predicate, which is how listings page without OFFSET.
func (b *Builder) Keyset(sortCol, idCol string, c core.Cursor, dir core.SortDirection) *Builder {
	if c.Zero() {
		return b
	}
	op := ">"
	if dir == core.Descending {
		op = "<"
	}
	b.wheres = append(b.wheres,
		fmt.Sprintf("(%s, %s) %s (?, ?)", sortCol, idCol, op))
	b.args = append(b.args, c.SortValue, c.ID)
	return b
}

// Keyset2 applies a two-key cursor predicate for a compound ordering, via a
// row-value comparison against both sort keys and the id tiebreaker in one
// go. A single-key Keyset cannot express this: paging on only the first key
// would repeat or skip rows whenever two tasks share it.
func (b *Builder) Keyset2(col1, col2, idCol string, c core.Cursor, dir core.SortDirection) *Builder {
	if c.Zero() {
		return b
	}
	op := ">"
	if dir == core.Descending {
		op = "<"
	}
	b.wheres = append(b.wheres,
		fmt.Sprintf("(%s, %s, %s) %s (?, ?, ?)", col1, col2, idCol, op))
	b.args = append(b.args, c.SortValue, c.SortValue2, c.ID)
	return b
}

// SelectQuery renders the SELECT and its arguments.
func (b *Builder) SelectQuery() (string, []any) {
	cols := "*"
	if len(b.columns) > 0 {
		cols = strings.Join(b.columns, ", ")
	}
	var sb strings.Builder
	sb.WriteString("SELECT " + cols + " FROM " + b.table)
	for _, j := range b.joins {
		sb.WriteString(" " + j)
	}
	b.writeWhere(&sb)
	if len(b.order) > 0 {
		sb.WriteString(" ORDER BY " + strings.Join(b.order, ", "))
	}
	if b.limit > 0 {
		sb.WriteString(" LIMIT " + strconv.Itoa(b.limit))
	}
	return b.rebind(sb.String()), concat(b.joinArgs, b.args)
}

// UpdateQuery renders the UPDATE and its arguments.
func (b *Builder) UpdateQuery() (string, []any, error) {
	if len(b.sets) == 0 {
		return "", nil, core.Invalid("update has no assignments")
	}
	var sb strings.Builder
	sb.WriteString("UPDATE " + b.table + " SET " + strings.Join(b.sets, ", "))
	b.writeWhere(&sb)
	return b.rebind(sb.String()), concat(b.setArgs, b.args), nil
}

// DeleteQuery renders the DELETE and its arguments.
func (b *Builder) DeleteQuery() (string, []any) {
	var sb strings.Builder
	sb.WriteString("DELETE FROM " + b.table)
	b.writeWhere(&sb)
	return b.rebind(sb.String()), b.args
}

// CountQuery renders a COUNT over the same predicates.
func (b *Builder) CountQuery() (string, []any) {
	var sb strings.Builder
	sb.WriteString("SELECT COUNT(*) FROM " + b.table)
	for _, j := range b.joins {
		sb.WriteString(" " + j)
	}
	b.writeWhere(&sb)
	return b.rebind(sb.String()), concat(b.joinArgs, b.args)
}

func (b *Builder) writeWhere(sb *strings.Builder) {
	if len(b.wheres) > 0 {
		sb.WriteString(" WHERE " + strings.Join(b.wheres, " AND "))
	}
}

// concat joins two argument runs into a fresh slice, so neither is aliased.
func concat(first, second []any) []any {
	out := make([]any, 0, len(first)+len(second))
	out = append(out, first...)
	return append(out, second...)
}

// rebind converts ? placeholders to $n for Postgres.
func (b *Builder) rebind(q string) string {
	if b.dialect != Postgres {
		return q
	}
	var sb strings.Builder
	n := 0
	for _, c := range q {
		if c == '?' {
			n++
			sb.WriteString("$" + strconv.Itoa(n))
			continue
		}
		sb.WriteRune(c)
	}
	return sb.String()
}

func qualify(table, col string) string { return table + "." + col }

// Insert builds an INSERT for one tenant-scoped row.
type Insert struct {
	dialect Dialect
	table   string
	cols    []string
	args    []any
}

// NewInsert starts an insert, setting the tenant column automatically.
func NewInsert(d Dialect, scope core.TenantScope, table string) (*Insert, error) {
	if !scope.Valid() && IsScoped(table) {
		return nil, core.Invalid("refusing to insert into %q without a tenant scope", table)
	}
	in := &Insert{dialect: d, table: table}
	if IsScoped(table) {
		in.Set(TenantColumn, scope.TenantID)
	}
	return in, nil
}

// Set adds a column and its value.
func (i *Insert) Set(col string, value any) *Insert {
	i.cols = append(i.cols, col)
	i.args = append(i.args, value)
	return i
}

// Query renders the INSERT and its arguments.
func (i *Insert) Query() (string, []any, error) {
	if len(i.cols) == 0 {
		return "", nil, core.Invalid("insert has no columns")
	}
	marks := make([]string, len(i.cols))
	for n := range i.cols {
		if i.dialect == Postgres {
			marks[n] = "$" + strconv.Itoa(n+1)
			continue
		}
		marks[n] = "?"
	}
	q := "INSERT INTO " + i.table +
		" (" + strings.Join(i.cols, ", ") + ") VALUES (" + strings.Join(marks, ", ") + ")"
	return q, i.args, nil
}

// IsScoped reports whether table carries a tenant column.
func IsScoped(table string) bool {
	_, exempt := unscoped[table]
	return !exempt
}

// ScopedTables returns the tables that must carry a tenant predicate.
func ScopedTables() []string {
	all := []string{
		"actors", "sessions", "api_tokens", "workflows", "projects", "field_defs",
		"tasks", "task_deps", "tags", "task_tags", "comments", "artifacts",
		"events", "audit_entries", "webhook_endpoints", "webhook_deliveries",
		"retention_policies", "external_refs", "sync_sources", "tenant_members",
		"ssh_keys", "tenant_domains",
	}
	return all
}
