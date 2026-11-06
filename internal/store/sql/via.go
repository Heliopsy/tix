// SPDX-License-Identifier: AGPL-3.0-or-later

package sql

import (
	stdsql "database/sql"

	"github.com/heliopsy/tix/internal/core"
)

// ViaKindText is the via_kind column value for v, unknown when v has no kind.
func ViaKindText(v core.Via) string { return string(v.Known().Kind) }

// ViaDetailText renders v as the via_detail column, NULL when it carries
// nothing past its kind.
func ViaDetailText(v core.Via) (any, error) {
	if v.Known() == (core.Via{Kind: v.Known().Kind}) {
		return nil, nil
	}
	s, err := JSONText(v, "")
	if err != nil || s == "" {
		return nil, err
	}
	return s, nil
}

// ScanVia rebuilds a channel from its kind column and its detail column. The
// kind column wins over whatever the detail says, because it is what filters
// select on.
func ScanVia(kind string, detail stdsql.NullString) (core.Via, error) {
	var v core.Via
	if err := ParseJSON(Text(detail), &v); err != nil {
		return core.Via{}, core.Internal("decoding access channel").Wrap(err)
	}
	v.Kind = core.ViaKind(kind)
	return v.Known(), nil
}

// ApplyVia narrows an audit query to channels and token names.
func ApplyVia(b *Builder, vias []core.ViaKind, tokens []string) {
	if len(vias) > 0 {
		kinds := make([]string, len(vias))
		for i, k := range vias {
			kinds[i] = string(k)
		}
		b.WhereIn("via_kind", kinds)
	}
	if len(tokens) > 0 {
		b.WhereIn("via_token", tokens)
	}
}
