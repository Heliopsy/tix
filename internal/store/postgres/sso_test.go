// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// TestSSOProvidersAndIdentitiesRoundTripOnPostgres pins every column of the two
// single sign-on tables on this engine, and that deleting a provider takes
// its identities with it and leaves the account.
func TestSSOProvidersAndIdentitiesRoundTripOnPostgres(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	p := core.SSOProvider{Key: "kc", Kind: core.SSOKindOIDC, DisplayName: "Keycloak", Enabled: true, AutoCreate: true,
		DefaultRole: core.RoleViewer, TrustMFA: true, AllowedDomains: []string{"example.com"},
		RoleMapping: map[string]core.Role{"admins": core.RoleAdmin}, Issuer: "https://id.example.com",
		ClientID: "tix", Scopes: []string{"openid"}, GroupsClaim: "roles", MFAValues: []string{"otp"},
		SealedSecret: "sealed", SecretNonce: "nonce", SecretHint: "abcd"}
	user := core.User{Email: "ada@example.com"}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.CreateSSOProvider(ctx, &p); err != nil {
			return err
		}
		if err := tx.CreateUser(ctx, &user, ""); err != nil {
			return err
		}
		return tx.CreateUserIdentity(ctx, &core.UserIdentity{ProviderID: p.ID, Subject: "sub-1", UserID: user.ID,
			Email: user.Email})
	}); err != nil {
		t.Fatalf("writing: %v", err)
	}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		got, err := tx.GetSSOProvider(ctx, "kc")
		if err != nil {
			return err
		}
		if !got.Enabled || !got.AutoCreate || !got.TrustMFA || got.DefaultRole != core.RoleViewer || !got.SecretSet ||
			got.RoleMapping["admins"] != core.RoleAdmin || got.AllowedDomains[0] != "example.com" ||
			got.Issuer != p.Issuer || got.GroupsClaim != "roles" || got.MFAValues[0] != "otp" || got.SecretHint != "abcd" {
			t.Errorf("read back %+v", got)
		}
		got.Enabled, got.DisplayName = false, "KC"
		if err := tx.UpdateSSOProvider(ctx, got); err != nil {
			return err
		}
		ident, err := tx.GetUserIdentity(ctx, p.ID, "sub-1")
		if err != nil {
			return err
		}
		if err := tx.TouchUserIdentity(ctx, ident.ID, "new@example.com", clk.Now()); err != nil {
			return err
		}
		byUser, err := tx.GetUserIdentityByUser(ctx, p.ID, user.ID)
		if err != nil || byUser.Email != "new@example.com" || byUser.LastLoginAt == nil {
			t.Errorf("identity by user = %+v, %v", byUser, err)
		}
		listed, err := tx.ListSSOProviders(ctx)
		if err != nil || len(listed) != 1 || listed[0].Enabled || listed[0].DisplayName != "KC" {
			t.Errorf("listed = %+v, %v", listed, err)
		}
		return tx.DeleteSSOProvider(ctx, p.ID)
	}); err != nil {
		t.Fatalf("updating: %v", err)
	}
	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		if _, err := tx.GetUserIdentity(ctx, p.ID, "sub-1"); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("identity after the provider went = %v, want not found", err)
		}
		if _, err := tx.GetUser(ctx, user.ID); err != nil {
			t.Errorf("the account went with the provider: %v", err)
		}
		tenant, err := tx.GetTenant(ctx)
		if err != nil {
			return err
		}
		if tenant.PasswordLogin != core.PasswordLoginAllowed {
			t.Errorf("password_login = %q, want allowed", tenant.PasswordLogin)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading: %v", err)
	}
}
