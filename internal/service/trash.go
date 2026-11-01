// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// trashOff reports whether the tenant the transaction is scoped to deletes
// tasks permanently.
func trashOff(ctx context.Context, tx store.Tx) (bool, error) {
	t, err := tx.GetTenant(ctx)
	if err != nil {
		return false, err
	}
	return t.TrashOff, nil
}

// TrashStatus says whether the tenant keeps deleted tasks and how many it
// holds. Any reader may ask, because every surface that draws a Delete needs
// to know whether it can be undone.
func (l *Local) TrashStatus(ctx context.Context) (*core.TrashStatus, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	out := &core.TrashStatus{}
	err = l.read(ctx, actor, func(tx store.Tx) error {
		off, err := trashOff(ctx, tx)
		if err != nil {
			return err
		}
		out.Enabled = !off
		project := ""
		if actor.ScopedToProject() {
			project = actor.ProjectID
		}
		out.Held, err = tx.CountTrashed(ctx, project)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// trashWord is how an audit entry names the trash setting.
func trashWord(on bool) string {
	if on {
		return "on"
	}
	return "off"
}
