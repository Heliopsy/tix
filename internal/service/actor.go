// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// systemActorID is the identifier the importer, the sweeper and pruning act
// under. It owns no actor row, so it is answered from the vocabulary rather
// than from the directory.
const systemActorID = "system"

// GetActor resolves an actor identifier of this tenant to the identity behind
// it. Every reader of a record that names an actor asks the same question --
// who is this -- so the lookup needs no scope beyond being signed in, and an
// identifier belonging to another tenant is reported missing like any other
// record of another tenant.
//
// Only the naming fields come back. Scopes, role and token identify authority
// rather than identity, and returning them would turn a directory lookup into
// a privilege report any signed-in caller could run.
func (l *Local) GetActor(ctx context.Context, id string) (*core.Actor, error) {
	caller, err := core.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, core.Invalid("actor identifier is required")
	}
	if id == systemActorID {
		return &core.Actor{ID: systemActorID, TenantID: caller.TenantID,
			Kind: core.ActorSystem, Handle: systemActorID}, nil
	}
	var out *core.Actor
	if err := l.read(ctx, caller, func(tx store.Tx) error {
		found, err := lookupActor(ctx, tx, id)
		if err != nil {
			return err
		}
		out = &core.Actor{ID: found.ID, TenantID: found.TenantID, Kind: found.Kind,
			Handle: found.Handle, DisplayName: found.DisplayName}
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// lookupActor resolves an actor by identifier or by handle, accepting a
// handle in any case because references are typed by hand. It mirrors
// lookupProject.
func lookupActor(ctx context.Context, tx store.Tx, ref string) (*core.Actor, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, core.Invalid("actor reference is required")
	}
	a, err := tx.GetActor(ctx, ref)
	if err == nil {
		return a, nil
	}
	if !core.IsKind(err, core.KindNotFound) {
		return nil, err
	}
	a, err = tx.GetActorByHandle(ctx, strings.ToLower(ref))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("actor %q", ref)
		}
		return nil, err
	}
	return a, nil
}

// actorSort is the only ordering the directory offers. An actor row carries
// no timestamp in the columns the store reads back, so a cursor can address a
// position by handle and by nothing else.
const actorSort = "handle"

// ListActors returns this tenant's actors, keyset paginated by handle.
//
// It needs no scope beyond being signed in, for the reason GetActor needs
// none: the directory answers who is here, never what they may do. Agents are
// actors without a user, so a picker built from ListUsers would omit the
// actors most of the work in this product is assigned to.
func (l *Local) ListActors(ctx context.Context, page core.Page) ([]core.Actor, string, error) {
	caller, err := core.RequireActor(ctx)
	if err != nil {
		return nil, "", err
	}
	if page.Sort == "" {
		page.Sort = actorSort
	}
	if page.Sort != actorSort {
		return nil, "", core.Invalid("actors sort by %s only", actorSort)
	}
	if page, err = page.Normalize(); err != nil {
		return nil, "", err
	}
	if err := mustCursorMatch(page); err != nil {
		return nil, "", err
	}

	out := []core.Actor{}
	if err := l.read(ctx, caller, func(tx store.Tx) error {
		found, err := tx.ListActors(ctx, page)
		if err != nil {
			return err
		}
		for _, a := range found {
			out = append(out, core.Actor{ID: a.ID, TenantID: a.TenantID, Kind: a.Kind,
				Handle: a.Handle, DisplayName: a.DisplayName})
		}
		return nil
	}); err != nil {
		return nil, "", err
	}
	return out, nextActorCursor(page, out), nil
}

// actorCursor addresses the position just after this actor in the ordering.
func actorCursor(page core.Page, a core.Actor) core.Cursor {
	return core.Cursor{SortValue: a.Handle, ID: a.ID, Sort: page.Sort, Direction: page.Direction}
}

// nextActorCursor returns the cursor for the page after these actors. Like
// nextUserCursor it is built from the rows being returned, never from the
// page underneath them, so an opaque token can never describe a record the
// caller may not read.
func nextActorCursor(page core.Page, items []core.Actor) string {
	if len(items) == 0 || len(items) < page.Limit {
		return ""
	}
	return actorCursor(page, items[len(items)-1]).Encode()
}
