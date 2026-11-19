// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"testing"
	"time"

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

// TestSAMLSettingsAndUsedAssertionsOnPostgres pins a SAML provider's settings
// and that an assertion id is taken once, until it expires.
func TestSAMLSettingsAndUsedAssertionsOnPostgres(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	p := core.SSOProvider{Key: "okta", Kind: core.SSOKindSAML, DisplayName: "Okta", Enabled: true,
		DefaultRole: core.RoleMember, IdPMetadataURL: "https://idp.example/md", IdPEntityID: "https://idp.example",
		IdPSSOURL: "https://idp.example/sso", IdPCertificates: []string{"PEM"}, SPEntityID: "urn:tix",
		NameIDFormat: core.NameIDPersistent, AllowIdPInitiated: true}
	now := clk.Now()
	consume := func(id string, expires time.Time, at time.Time) error {
		return s.Update(ctx, f.scope, func(tx store.Tx) error {
			return tx.ConsumeSSOAssertion(ctx, p.ID, id, expires, at)
		})
	}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error { return tx.CreateSSOProvider(ctx, &p) }); err != nil {
		t.Fatalf("creating: %v", err)
	}
	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		got, err := tx.GetSSOProvider(ctx, "okta")
		if err != nil {
			return err
		}
		if got.IdPMetadataURL != p.IdPMetadataURL || got.IdPEntityID != p.IdPEntityID || got.IdPSSOURL != p.IdPSSOURL ||
			len(got.IdPCertificates) != 1 || got.SPEntityID != "urn:tix" || got.NameIDFormat != core.NameIDPersistent ||
			!got.AllowIdPInitiated {
			t.Errorf("read back %+v", got)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading: %v", err)
	}
	if err := consume("_a1", now.Add(5*time.Minute), now); err != nil {
		t.Fatalf("first use: %v", err)
	}
	if err := consume("_a1", now.Add(5*time.Minute), now.Add(time.Minute)); !core.IsKind(err, core.KindConflict) {
		t.Errorf("second use = %v, want a conflict", err)
	}
	if err := consume("_a1", now.Add(20*time.Minute), now.Add(10*time.Minute)); err != nil {
		t.Errorf("use after the first expired = %v, want it pruned and taken", err)
	}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error { return tx.DeleteSSOProvider(ctx, p.ID) }); err != nil {
		t.Errorf("deleting a provider with used assertions: %v", err)
	}
}

// TestSAMLRefreshStateOnPostgres pins the refresh state a SAML provider keeps in
// its settings, and that recording a refresh leaves updated_at alone.
func TestSAMLRefreshStateOnPostgres(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	at := clk.Now().Truncate(time.Second)
	p := core.SSOProvider{Key: "okta", Kind: core.SSOKindSAML, DisplayName: "Okta", Enabled: true,
		DefaultRole: core.RoleMember, IdPMetadataURL: "https://idp.example/md", IdPEntityID: "https://idp.example",
		IdPSSOURL: "https://idp.example/sso", IdPCertificates: []string{"cert"}, IdPPinned: []string{core.PinIdPSSOURL}}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error { return tx.CreateSSOProvider(ctx, &p) }); err != nil {
		t.Fatalf("creating: %v", err)
	}
	clk.Advance(time.Hour)
	p.MetadataRefresh = &core.SAMLMetadataRefresh{At: at, Result: core.SAMLRefreshFailed, Error: "idp down"}
	p.IdPRetiring = []core.RetiringCertificate{{Fingerprint: "ab", Since: at, Until: at.Add(time.Hour)}}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error { return tx.RecordSSOProviderRefresh(ctx, &p) }); err != nil {
		t.Fatalf("recording: %v", err)
	}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		got, err := tx.GetSSOProvider(ctx, "okta")
		if err != nil {
			return err
		}
		if got.MetadataRefresh == nil || !got.MetadataRefresh.At.Equal(at) || got.MetadataRefresh.Error != "idp down" ||
			len(got.IdPRetiring) != 1 || got.IdPRetiring[0].Fingerprint != "ab" || !got.Pinned(core.PinIdPSSOURL) ||
			got.UpdatedAt.After(got.CreatedAt.Add(time.Second)) {
			t.Errorf("read back %+v, refresh %+v", got, got.MetadataRefresh)
		}
		gone := core.SSOProvider{ID: "nobody", Key: "nobody"}
		if err := tx.RecordSSOProviderRefresh(ctx, &gone); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("recording a missing provider = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading: %v", err)
	}
}
