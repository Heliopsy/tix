// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"slices"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// tenantIcon is one validated tenant icon choice: at most one of the three
// kinds.
type tenantIcon struct {
	text  string
	name  string
	image *core.TenantIconImage
}

// resolveTenantIcon validates an icon choice and types an uploaded image by
// its bytes.
func resolveTenantIcon(text, name string, file *core.ProjectIconFile) (tenantIcon, error) {
	if err := core.ValidateTenantIconChoice(text, name, file); err != nil {
		return tenantIcon{}, err
	}
	var out tenantIcon
	out.text, _ = core.NormalizeTenantIcon(text)
	out.name, _ = core.NormalizeTenantIconName(name)
	if !file.Empty() {
		img, err := core.NewTenantIconImage(*file)
		if err != nil {
			return tenantIcon{}, err
		}
		out.image = &img
	}
	return out, nil
}

// apply replaces whichever icon the tenant carried with this one.
func (i tenantIcon) apply(t *core.Tenant) {
	t.Icon, t.IconName, t.IconImage = i.text, i.name, ""
	if i.image != nil {
		t.IconImage = i.image.Digest
	}
}

// store writes the image this choice carries, and removes the one the tenant
// had when the choice no longer names it.
func (i tenantIcon) store(ctx context.Context, u store.UnscopedTx, tenantID, previous string) error {
	if i.image == nil {
		if previous == "" {
			return nil
		}
		return u.DeleteTenantIcon(ctx, tenantID)
	}
	if i.image.Digest == previous {
		return nil
	}
	i.image.TenantID = tenantID
	return u.PutTenantIcon(ctx, i.image)
}

// GetTenantIcon returns a tenant's uploaded icon image. Any member of the
// tenant may read it, from a session or token in any tenant they belong to,
// so a switcher can draw the icon of every tenant it lists.
func (l *Local) GetTenantIcon(ctx context.Context, ref string) (*core.TenantIconImage, error) {
	actor, err := core.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(ref) == "" {
		ref = actor.TenantID
	}
	tenant, accountID, err := l.iconTenant(ctx, actor, ref)
	if err != nil {
		return nil, err
	}
	if tenant.ID != actor.TenantID {
		err := l.mayReadTenantIcon(ctx, actor, accountID, tenant.ID)
		if core.IsKind(err, core.KindForbidden) {
			return nil, core.NotFound("tenant %q", ref)
		}
		if err != nil {
			return nil, err
		}
	}
	if tenant.IconImage == "" {
		return nil, core.NotFound("tenant %q has no icon image", tenant.Key)
	}
	var out *core.TenantIconImage
	err = l.read(ctx, actor, func(tx store.Tx) error {
		u, err := asUnscoped(tx)
		if err != nil {
			return err
		}
		out, err = u.GetTenantIcon(ctx, tenant.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// iconTenant resolves the live tenant whose icon is asked for and the
// account behind the actor, empty when it has none.
func (l *Local) iconTenant(ctx context.Context, actor *core.Actor, ref string) (*core.Tenant, string, error) {
	var (
		tenant    *core.Tenant
		accountID string
	)
	err := l.read(ctx, actor, func(tx store.Tx) error {
		u, err := asUnscoped(tx)
		if err != nil {
			return err
		}
		if tenant, err = lookupTenant(ctx, u, ref); err != nil {
			return err
		}
		if tenant.DeletedAt != nil {
			return core.NotFound("tenant %q", ref)
		}
		account, err := accountOfActor(ctx, tx, actor.ID)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		accountID = account.ID
		return nil
	})
	return tenant, accountID, err
}

// mayReadTenantIcon allows a global admin, and an account holding a
// membership in tenantID, read through membershipsOf. A token answers only
// when it carries account:read, the scope that lets it list its owner's
// tenants at all, so a token cannot probe them one by one.
func (l *Local) mayReadTenantIcon(ctx context.Context, actor *core.Actor, accountID, tenantID string) error {
	global, err := l.globalAllowed(ctx)
	if err != nil || global {
		return err
	}
	refused := core.Forbidden("not a member of the tenant")
	if accountID == "" || (actor.TokenID != "" && !actor.HasScope(core.ScopeAccountRead)) {
		return refused
	}
	found, err := l.membershipsOf(ctx, accountID)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(found, func(m core.AccountMembership) bool { return m.TenantID == tenantID }) {
		return refused
	}
	return nil
}
