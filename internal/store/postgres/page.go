package postgres

import (
	"github.com/thereisnotime/tix/internal/core"
	sqlb "github.com/thereisnotime/tix/internal/store/sql"
)

// pageSpec is a normalized listing window. Listings page by keyset only; no
// statement in this package emits OFFSET.
type pageSpec struct {
	limit  int
	column string
	dir    core.SortDirection
	cursor core.Cursor
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
func (p pageSpec) apply(b *sqlb.Builder, idColumn string) *sqlb.Builder {
	return b.Keyset(p.column, idColumn, p.cursor, p.dir).
		OrderBy(p.column, p.dir).
		OrderBy(idColumn, p.dir).
		Limit(p.limit)
}
