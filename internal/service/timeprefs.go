// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

const auditUserTimePrefs = "user.time_prefs"

// WithTimeDefaults sets the deployment's date format and timezone, the level a
// tenant and a person each lay their own choice over.
func WithTimeDefaults(p core.TimePrefs) Option {
	return func(l *Local) { l.timeDefaults = p }
}

// GetTimeSettings returns the caller's date format and timezone at every level
// and the values in force.
func (l *Local) GetTimeSettings(ctx context.Context) (*core.TimeSettings, error) {
	actor, err := core.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	var out core.TimeSettings
	err = l.read(ctx, actor, func(tx store.Tx) error {
		var err error
		out, err = l.timeSettings(ctx, tx, actor)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// SetTimePrefs changes the caller's own date format and timezone.
func (l *Local) SetTimePrefs(ctx context.Context, in core.TimePrefsInput) (*core.TimeSettings, error) {
	actor, err := core.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	core.NormalizeTimePrefs(in.TimeFormat, in.Timezone)
	if err := in.Validate(); err != nil {
		return nil, err
	}
	var out core.TimeSettings
	err = l.write(ctx, actor, func(m *mutation) error {
		before, err := userTimePrefs(ctx, m.tx, actor)
		if err != nil {
			return err
		}
		after := before
		if in.TimeFormat != nil {
			after.TimeFormat = *in.TimeFormat
		}
		if in.Timezone != nil {
			after.Timezone = *in.Timezone
		}
		if after != before {
			if err := m.tx.SetUserTimePrefs(ctx, actor.ID, after); err != nil {
				return err
			}
			if err := m.Record(auditUserTimePrefs, eventUserUpdated, "user", actor.ID, "", before, after,
				map[string]any{"time_format": after.TimeFormat, "timezone": after.Timezone}); err != nil {
				return err
			}
		}
		out, err = l.timeSettings(ctx, m.tx, actor)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// timeSettings resolves the three levels for one caller. An automated actor
// has no users row and reads the tenant's choice.
func (l *Local) timeSettings(ctx context.Context, tx store.Tx, actor *core.Actor) (core.TimeSettings, error) {
	tenant, err := tx.GetTenant(ctx)
	if err != nil {
		return core.TimeSettings{}, err
	}
	user, err := tx.UserTimePrefs(ctx, actor.ID)
	personal := err == nil
	if err != nil && !core.IsKind(err, core.KindNotFound) {
		return core.TimeSettings{}, err
	}
	out := core.ResolveTimePrefs(l.timeDefaults,
		core.TimePrefs{TimeFormat: tenant.TimeFormat, Timezone: tenant.Timezone}, user)
	out.Personal = personal
	return out, nil
}

// userTimePrefs reads the caller's own choice, refusing a caller who is not a
// person.
func userTimePrefs(ctx context.Context, tx store.Tx, actor *core.Actor) (core.TimePrefs, error) {
	p, err := tx.UserTimePrefs(ctx, actor.ID)
	if core.IsKind(err, core.KindNotFound) {
		return core.TimePrefs{}, core.Invalid("a date preference belongs to a person, and this caller is not one; a tenant administrator sets the tenant's default with tix tenant edit")
	}
	return p, err
}
