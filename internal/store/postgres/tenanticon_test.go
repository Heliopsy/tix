// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"bytes"
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// TestTenantIconImagesRoundTripAcrossTenants stores a tenant's icon from a
// transaction scoped to another tenant, as a global admin's update does, and
// reads it back from outside both: the row, the image and the membership
// listing a switcher reads all carry it, and removing it leaves nothing.
func TestTenantIconImagesRoundTripAcrossTenants(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	acme, globex := seed(t, s, clk, "acme"), seed(t, s, clk, "globex")
	data := []byte("\x89PNG\r\n\x1a\nglobex\x00\xff")
	digest := core.ProjectIconDigest(data)

	pat := core.User{Email: "pat@example.com"}
	if err := s.Update(ctx, globex.scope, func(tx store.Tx) error {
		if err := tx.CreateUser(ctx, &pat, ""); err != nil {
			return err
		}
		actor := core.Actor{Kind: core.ActorUser, Handle: "pat", UserID: pat.ID}
		if err := tx.CreateActor(ctx, &actor); err != nil {
			return err
		}
		return tx.AddMember(ctx, &core.Membership{ActorID: actor.ID, Role: core.RoleMember})
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.Update(ctx, acme.scope, func(tx store.Tx) error {
		u := tx.(store.UnscopedTx)
		tenant, err := u.GetTenantByID(ctx, globex.tenant.ID)
		if err != nil {
			return err
		}
		tenant.Icon, tenant.IconName, tenant.IconImage = "", "", digest
		if err := u.UpdateTenant(ctx, tenant); err != nil {
			return err
		}
		return u.PutTenantIcon(ctx, &core.TenantIconImage{TenantID: globex.tenant.ID, ContentType: "image/png",
			Digest: digest, Data: data})
	}); err != nil {
		t.Fatalf("storing globex's icon from acme: %v", err)
	}

	if err := s.ViewUnscoped(ctx, func(u store.UnscopedTx) error {
		img, err := u.GetTenantIcon(ctx, globex.tenant.ID)
		if err != nil {
			return err
		}
		if !bytes.Equal(img.Data, data) || img.ContentType != "image/png" || img.Digest != digest ||
			img.TenantID != globex.tenant.ID || img.UpdatedAt.IsZero() {
			t.Errorf("read back %+v", img)
		}
		if _, err := u.GetTenantIcon(ctx, acme.tenant.ID); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("acme's icon = %v, want not found", err)
		}
		back, err := u.GetTenantByID(ctx, globex.tenant.ID)
		if err != nil || back.IconImage != digest {
			t.Errorf("globex read back with icon_image %q (%v), want %q", back.IconImage, err, digest)
		}
		found, err := u.ListMemberships(ctx, pat.ID)
		if err != nil || len(found) != 1 || found[0].IconImage != digest {
			t.Errorf("memberships = %+v, %v; want globex carrying its icon", found, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
		tenant, err := u.GetTenantByID(ctx, globex.tenant.ID)
		if err != nil {
			return err
		}
		tenant.IconName, tenant.IconImage = "rocket", ""
		if err := u.UpdateTenant(ctx, tenant); err != nil {
			return err
		}
		return u.DeleteTenantIcon(ctx, globex.tenant.ID)
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.ViewUnscoped(ctx, func(u store.UnscopedTx) error {
		if _, err := u.GetTenantIcon(ctx, globex.tenant.ID); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("a removed icon = %v, want not found", err)
		}
		found, err := u.ListMemberships(ctx, pat.ID)
		if err != nil || len(found) != 1 || found[0].IconName != "rocket" || found[0].IconImage != "" {
			t.Errorf("memberships = %+v, %v; want globex with the rocket", found, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
