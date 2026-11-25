// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"bytes"
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// tenantIconStored reports whether a tenant's image bytes are on its row.
func tenantIconStored(t *testing.T, l *Local, tenantID string) bool {
	t.Helper()
	found := false
	if err := l.store.ViewUnscoped(context.Background(), func(u store.UnscopedTx) error {
		_, err := u.GetTenantIcon(context.Background(), tenantID)
		found = err == nil
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return found
}

// patIn is pat signed in to scope as actorID, through a session or, with
// scopes, a token carrying them.
func patIn(scope core.TenantScope, actorID string, scopes ...core.Scope) context.Context {
	actor := &core.Actor{ID: actorID, TenantID: scope.TenantID, Kind: core.ActorUser, Role: core.RoleMember}
	if len(scopes) > 0 {
		actor.TokenID, actor.Scopes, actor.Role = "tok", scopes, ""
	}
	return authContext(actor)
}

func TestUpdateTenantSetsEachIconKindAndReplacesTheLast(t *testing.T) {
	f := newTwoTenants(t)
	ctx := authContext(f.otherAdmin)
	steps := []struct {
		name  string
		in    core.UpdateTenantInput
		kind  core.ProjectIconKind
		image bool
	}{
		{"text", core.UpdateTenantInput{Icon: strp(" O ")}, core.IconKindText, false},
		{"image", core.UpdateTenantInput{IconFile: &core.ProjectIconFile{ContentType: "text/plain", Data: iconPNG}}, core.IconKindImage, true},
		{"same image again", core.UpdateTenantInput{IconFile: &core.ProjectIconFile{Data: iconPNG}}, core.IconKindImage, true},
		{"built-in replaces the image", core.UpdateTenantInput{IconName: strp(" Rocket ")}, core.IconKindBuiltin, false},
		{"name only leaves it", core.UpdateTenantInput{Name: strp("Other Ltd")}, core.IconKindBuiltin, false},
		{"empty clears it", core.UpdateTenantInput{Icon: strp("")}, core.IconKindNone, false},
	}
	for _, s := range steps {
		got, err := f.l.UpdateTenant(ctx, "", s.in)
		if err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		if got.IconKind() != s.kind {
			t.Errorf("%s: kind %q (%+v), want %q", s.name, got.IconKind(), got, s.kind)
		}
		if stored := tenantIconStored(t, f.l, f.other.TenantID); stored != s.image {
			t.Errorf("%s: image stored = %v, want %v", s.name, stored, s.image)
		}
	}
	read, err := f.l.GetTenant(ctx, "")
	if err != nil || read.Name != "Other Ltd" || read.IconKind() != core.IconKindNone {
		t.Fatalf("tenant read back as %+v, %v", read, err)
	}
	if got, err := f.l.UpdateTenant(ctx, "", core.UpdateTenantInput{IconName: strp("rocket")}); err != nil || got.IconName != "rocket" {
		t.Fatalf("normalised built-in = %+v, %v", got, err)
	}

	refusals := []struct {
		name string
		ctx  context.Context
		in   core.UpdateTenantInput
		kind core.Kind
	}{
		{"two kinds", ctx, core.UpdateTenantInput{Icon: strp("O"), IconName: strp("bug")}, core.KindInvalid},
		{"unknown built-in", ctx, core.UpdateTenantInput{IconName: strp("nope")}, core.KindInvalid},
		{"script in an svg", ctx, core.UpdateTenantInput{IconFile: &core.ProjectIconFile{
			Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)}}, core.KindInvalid},
		{"a member", patIn(f.other, f.otherPat.ID), core.UpdateTenantInput{Icon: strp("X")}, core.KindForbidden},
	}
	for _, r := range refusals {
		_, err := f.l.UpdateTenant(r.ctx, "", r.in)
		if !core.IsKind(err, r.kind) {
			t.Errorf("%s: %v, want %s", r.name, err, r.kind)
		}
	}
	if read, _ := f.l.GetTenant(ctx, ""); read.IconName != "rocket" {
		t.Errorf("a refused update changed the icon: %+v", read)
	}
}

// TestATenantIconIsReadByItsMembersFromAnyOfTheirTenants is the switcher's
// case: pat, signed in to home, draws the icon of the other tenant they
// belong to. A global admin sets it from outside the tenant. Somebody who
// does not belong there, and a token that may not list its owner's tenants,
// get not found.
func TestATenantIconIsReadByItsMembersFromAnyOfTheirTenants(t *testing.T) {
	f := newTwoTenants(t)
	if _, err := f.l.UpdateTenant(f.global, f.otherKey, core.UpdateTenantInput{
		IconFile: &core.ProjectIconFile{Data: iconOther}}); err != nil {
		t.Fatalf("a global admin setting another tenant's icon: %v", err)
	}

	readers := []struct {
		name string
		ctx  context.Context
		ref  string
		ok   bool
	}{
		{"its admin, own tenant by default", authContext(f.otherAdmin), "", true},
		{"a member signed in to another of their tenants", patIn(f.home, f.homePat.ID), f.otherKey, true},
		{"by identifier", patIn(f.home, f.homePat.ID), f.other.TenantID, true},
		{"a token carrying account:read", patIn(f.home, f.homePat.ID, core.ScopeTaskRead, core.ScopeAccountRead), f.otherKey, true},
		{"a global admin", f.global, f.otherKey, true},
		{"a token without account:read", patIn(f.home, f.homePat.ID, core.ScopeAll), f.otherKey, false},
		{"somebody who is no member", authContext(f.homeAdmin), f.otherKey, false},
		{"a tenant that does not exist", patIn(f.home, f.homePat.ID), "nowhere", false},
	}
	for _, r := range readers {
		img, err := f.l.GetTenantIcon(r.ctx, r.ref)
		switch {
		case r.ok && (err != nil || !bytes.Equal(img.Data, iconOther) || img.ContentType != "image/png"):
			t.Errorf("%s: %+v, %v; want the image", r.name, img, err)
		case !r.ok && !core.IsKind(err, core.KindNotFound):
			t.Errorf("%s: %v, want not found", r.name, err)
		}
	}
	if _, err := f.l.GetTenantIcon(patIn(f.home, f.homePat.ID), ""); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("home has no image: %v, want not found", err)
	}
}

func TestWhoAmICarriesTheTenantIcons(t *testing.T) {
	f := newTwoTenants(t)
	other, err := f.l.UpdateTenant(authContext(f.otherAdmin), "", core.UpdateTenantInput{IconName: strp("rocket")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.l.UpdateTenant(f.global, "", core.UpdateTenantInput{Icon: strp("H")}); err != nil {
		t.Fatal(err)
	}

	me, err := f.l.WhoAmI(patIn(f.home, f.homePat.ID))
	if err != nil {
		t.Fatal(err)
	}
	if me.TenantIcon != "H" || me.TenantIconName != "" || me.TenantIconImage != "" {
		t.Errorf("whoami's own tenant icon = %q %q %q, want H", me.TenantIcon, me.TenantIconName, me.TenantIconImage)
	}
	seen := map[string]core.AccountMembership{}
	for _, m := range me.Memberships {
		seen[m.TenantID] = m
	}
	if m := seen[other.ID]; m.IconName != "rocket" || m.Icon != "" {
		t.Errorf("the other membership = %+v, want the rocket", m)
	}
	if m := seen[f.home.TenantID]; m.Icon != "H" {
		t.Errorf("the home membership = %+v, want H", m)
	}
}
