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
