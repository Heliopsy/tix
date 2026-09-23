// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"database/sql"
	"time"

	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// timeArg renders an instant for a TIMESTAMPTZ column.
func timeArg(t time.Time) any { return t.UTC() }

// nullTimeArg renders an optional instant, yielding NULL when absent.
func nullTimeArg(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return t.UTC()
}

// scanTime converts a possibly null TIMESTAMPTZ column to a value.
func scanTime(v sql.NullTime) time.Time {
	if !v.Valid {
		return time.Time{}
	}
	return v.Time.UTC()
}

// scanNullTime converts an optional TIMESTAMPTZ column to a pointer.
func scanNullTime(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time.UTC()
	return &t
}

// text returns the string held by an optional column, or the empty string.
func text(v sql.NullString) string { return sqlb.Text(v) }

// nullText renders an empty string as a NULL column value.
func nullText(s string) any { return sqlb.NullText(s) }
