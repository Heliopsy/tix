// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strconv"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// actingAccount is the account behind whoever makes a change, for the
// account log. It is empty for the system and for a shell holding the
// database, which act as no person.
type actingAccount struct {
	id, email string
	global    bool
}

// actingAs resolves the account behind an actor of its own tenant.
func (l *Local) actingAs(ctx context.Context, actor *core.Actor) (actingAccount, error) {
	out := actingAccount{global: core.DirectStoreFrom(ctx)}
	err := l.read(ctx, actor, func(tx store.Tx) error {
		by, err := accountBehind(ctx, tx, actor)
		if err == nil {
			out.id, out.email = by.id, by.email
			out.global = out.global || by.global
		}
		return err
	})
	return out, err
}

// accountBehind reads the account behind an actor in the transaction's
// tenant. An actor with no account, an agent, the system or the local actor,
// is nobody's account and not an error.
func accountBehind(ctx context.Context, tx store.Tx, actor *core.Actor) (actingAccount, error) {
	if actor == nil {
		return actingAccount{}, nil
	}
	u, err := accountOfActor(ctx, tx, actor.ID)
	switch {
	case core.IsKind(err, core.KindNotFound):
		return actingAccount{}, nil
	case err != nil:
		return actingAccount{}, err
	}
	return actingAccount{id: u.ID, email: u.Email, global: u.GlobalAdmin && u.DisabledAt == nil}, nil
}

// logAccount records a change to an account in the mutation's transaction,
// naming the tenant it was made from, who made it and through which channel.
// The detail must never carry a secret.
func (m *mutation) logAccount(ctx context.Context, userID string, action core.AccountAction,
	detail map[string]any,
) error {
	if userID == "" {
		return nil
	}
	tenant, err := m.tx.GetTenant(ctx)
	if err != nil {
		return err
	}
	by, ok := ctx.Value(globalStandingKey{}).(actingAccount)
	if !ok {
		if by, err = accountBehind(ctx, m.tx, m.actor); err != nil {
			return err
		}
		by.global = by.global || core.DirectStoreFrom(ctx)
	}
	return m.tx.AppendAccountLog(ctx, &core.AccountLogEntry{
		UserID: userID, Action: action, TenantKey: tenant.Key,
		ByUserID: by.id, ByEmail: by.email, ByGlobalAdmin: by.global,
		Via: m.via.Known().Kind, Detail: detail, OccurredAt: m.now,
	})
}

// GetAccountLog returns the caller's own account log, keyset paginated on
// seq, oldest first unless the page asks for desc. A browser session reads it; a token needs account:read, which
// "*" does not carry, so an agent's token learns nothing about the person's
// other tenants.
func (l *Local) GetAccountLog(ctx context.Context, page core.Page) ([]core.AccountLogEntry, string, error) {
	actor, err := core.RequireActor(ctx)
	if err != nil {
		return nil, "", err
	}
	if err := l.policy.CanReadAccount(actor); err != nil {
		return nil, "", err
	}
	var account *core.User
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		var err error
		account, err = accountOfActor(ctx, tx, actor.ID)
		return err
	}); err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, "", core.Invalid("an account log belongs to a person, and this caller is not one")
		}
		return nil, "", err
	}
	return l.accountLog(ctx, actor, account.ID, page)
}

// GetAccountLogFor returns any account's log, found by email, for a global
// admin. Anybody else is refused before the address is looked at.
func (l *Local) GetAccountLogFor(ctx context.Context, email string, page core.Page) ([]core.AccountLogEntry, string, error) {
	actor, err := l.authorizeGlobal(ctx)
	if err != nil {
		return nil, "", err
	}
	if err := core.ValidateEmail(email); err != nil {
		return nil, "", err
	}
	found, err := l.inviteeByEmail(ctx, core.NormalEmail(email))
	if err != nil {
		return nil, "", err
	}
	if found.user == nil {
		return nil, "", core.NotFound("no account has the address %s", core.NormalEmail(email))
	}
	return l.accountLog(ctx, actor, found.user.ID, page)
}

// accountLog reads one page of an account's log.
func (l *Local) accountLog(ctx context.Context, actor *core.Actor, userID string, page core.Page,
) ([]core.AccountLogEntry, string, error) {
	if page.Sort == "" {
		page.Sort = "seq"
	}
	if page.Sort != "seq" {
		return nil, "", core.Invalid("cannot sort an account log by %q", page.Sort)
	}
	page, err := page.Normalize()
	if err != nil {
		return nil, "", err
	}
	var out []core.AccountLogEntry
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		out, err = tx.ListAccountLog(ctx, userID, page)
		return err
	}); err != nil {
		return nil, "", err
	}
	if len(out) < page.Limit {
		return out, "", nil
	}
	seq := strconv.FormatInt(out[len(out)-1].Seq, 10)
	return out, core.Cursor{SortValue: seq, ID: seq, Sort: page.Sort, Direction: page.Direction}.Encode(), nil
}
