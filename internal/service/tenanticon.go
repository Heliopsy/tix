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
	global, err := l.globalAllowed(ctx)
	if err != nil {
		return nil, err
	}
	var out *core.TenantIconImage
	err = l.read(ctx, actor, func(tx store.Tx) error {
		u, err := asUnscoped(tx)
		if err != nil {
			return err
		}
		t, err := lookupTenant(ctx, u, ref)
		if err != nil {
			return err
		}
		if t.DeletedAt != nil {
			return core.NotFound("tenant %q", ref)
		}
		if t.ID != actor.TenantID && !global {
			member, err := memberOf(ctx, tx, u, actor, t.ID)
			if err != nil {
				return err
			}
			if !member {
				return core.NotFound("tenant %q", ref)
			}
		}
		if t.IconImage == "" {
			return core.NotFound("tenant %q has no icon image", t.Key)
		}
		out, err = u.GetTenantIcon(ctx, t.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// memberOf reports whether the account behind actor belongs to tenantID. A
// token answers only when it carries account:read, the scope that lets it
// list its owner's tenants at all, so a token cannot probe them one by one.
func memberOf(ctx context.Context, tx store.Tx, u store.UnscopedTx, actor *core.Actor, tenantID string) (bool, error) {
	if actor.TokenID != "" && !actor.HasScope(core.ScopeAccountRead) {
		return false, nil
	}
	account, err := accountOfActor(ctx, tx, actor.ID)
	if core.IsKind(err, core.KindNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	found, err := u.ListMemberships(ctx, account.ID)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(found, func(m core.AccountMembership) bool { return m.TenantID == tenantID }), nil
}
