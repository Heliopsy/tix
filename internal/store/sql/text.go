// SPDX-License-Identifier: AGPL-3.0-or-later

package sql

import (
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// likeEscape is the escape character every text predicate declares, so a value
// holding a percent sign or an underscore filters for that character rather
// than for the wildcard it spells.
const likeEscape = `\`

// ApplyTextTerms folds explicit title and body predicates into a statement.
//
// It lives here, shared by both engines, on purpose. core.TaskFilter.Query is
// answered natively by each engine -- a tsquery on PostgreSQL, a LIKE on
// SQLite -- and so already means two different things depending on where the
// data sits. These terms are the opposite bargain: the same LOWER/LIKE SQL on
// both, so a weak match selects the same tasks whichever engine answers it.
// The one residue is case folding outside ASCII, where SQLite's LOWER is
// ASCII-only and PostgreSQL's is not; ASCII, which every status, tag and
// English title is, folds identically.
func ApplyTextTerms(b *Builder, terms []core.TextTerm, titleCol, bodyCol string) error {
	for _, term := range terms {
		if err := term.Validate(); err != nil {
			return err
		}
		cond, args := textPredicate(term, titleCol, bodyCol)
		if term.Negate {
			cond = "NOT (" + cond + ")"
		}
		b.Where(cond, args...)
	}
	return nil
}

// textPredicate renders one term over the columns it addresses.
func textPredicate(term core.TextTerm, titleCol, bodyCol string) (string, []any) {
	cols := []string{titleCol}
	switch term.Field {
	case core.TextBody:
		cols = []string{bodyCol}
	case core.TextAny:
		cols = []string{titleCol, bodyCol}
	case core.TextTitle:
	}
	parts := make([]string, 0, len(cols))
	args := make([]any, 0, len(cols))
	for _, col := range cols {
		if term.Mode == core.MatchContains {
			parts = append(parts, "LOWER("+col+") LIKE LOWER(?) ESCAPE '"+likeEscape+"'")
			args = append(args, "%"+escapeLike(term.Value)+"%")
			continue
		}
		parts = append(parts, "LOWER("+col+") = LOWER(?)")
		args = append(args, term.Value)
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}

// escapeLike neutralises the wildcards inside a user-supplied value.
func escapeLike(v string) string {
	r := strings.NewReplacer(
		likeEscape, likeEscape+likeEscape,
		"%", likeEscape+"%",
		"_", likeEscape+"_",
	)
	return r.Replace(v)
}
