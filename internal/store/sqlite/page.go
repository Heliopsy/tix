// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"github.com/heliopsy/tix/internal/core"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// pageSpec is a normalized listing window. Listings page by keyset only; no
// statement in this package emits OFFSET.
type pageSpec struct {
	limit   int
	column  string
	column2 string // set only for a compound ordering; empty otherwise
	dir     core.SortDirection
	cursor  core.Cursor
}

// resolvePage normalizes a page against the sort columns a listing allows.
func resolvePage(p core.Page, defaultSort string, columns map[string]string) (pageSpec, error) {
	if p.Sort == "" {
		p.Sort = defaultSort
	}
	col, ok := columns[p.Sort]
	if !ok {
		return pageSpec{}, core.Invalid("cannot sort by %q", p.Sort)
	}
	norm, err := p.Normalize()
	if err != nil {
		return pageSpec{}, err
	}
	c, err := core.DecodeCursor(norm.Cursor)
	if err != nil {
		return pageSpec{}, err
	}
	return pageSpec{limit: norm.Limit, column: col, dir: norm.Direction, cursor: c}, nil
}

// apply adds the ordering, the keyset predicate and the limit to a statement.
// A compound ordering (column2 set) carries every key it orders by in the
// keyset predicate, or a page could skip or repeat rows whenever two rows
// share the first key.
func (p pageSpec) apply(b *sqlb.Builder, idColumn string) *sqlb.Builder {
	if p.column2 != "" {
		b.Keyset2(p.column, p.column2, idColumn, p.cursor, p.dir).
			OrderBy(p.column, p.dir).
			OrderBy(p.column2, p.dir)
	} else {
		b.Keyset(p.column, idColumn, p.cursor, p.dir).
			OrderBy(p.column, p.dir)
	}
	return b.OrderBy(idColumn, p.dir).Limit(p.limit)
}
