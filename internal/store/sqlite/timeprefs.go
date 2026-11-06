// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"

	"github.com/heliopsy/tix/internal/core"
)

// UserTimePrefs returns the date format and timezone a user chose for
// themselves, empty where they inherit.
func (t *tx) UserTimePrefs(ctx context.Context, userID string) (core.TimePrefs, error) {
	b := t.builder("users").Select("time_format", "timezone").Where("id = ?", userID).Limit(1)
	q, args := b.SelectQuery()
	var p core.TimePrefs
	if err := t.ex.QueryRowContext(ctx, q, args...).Scan(&p.TimeFormat, &p.Timezone); err != nil {
		err = mapErr(err, "reading time preferences")
		if core.IsKind(err, core.KindNotFound) {
			return core.TimePrefs{}, core.NotFound("user %q", userID)
		}
		return core.TimePrefs{}, err
	}
	return p, nil
}

// SetUserTimePrefs replaces a user's date format and timezone.
func (t *tx) SetUserTimePrefs(ctx context.Context, userID string, p core.TimePrefs) error {
	b := t.builder("users").
		Where("id = ?", userID).
		Set("time_format", p.TimeFormat).
		Set("timezone", p.Timezone)
	n, err := t.execUpdate(ctx, b, "updating time preferences of user %q", userID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("user %q", userID)
	}
	return nil
}
