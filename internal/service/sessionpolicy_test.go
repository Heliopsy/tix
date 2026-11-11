// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

func TestTenantSessionSettingsOnlyTighten(t *testing.T) {
	l, _, _, admin := newLocalWith(t, WithSessionPolicy(core.SessionPolicy{
		TTL: core.Duration(12 * time.Hour), IdleTimeout: core.Duration(time.Hour), RememberTTL: core.Duration(720 * time.Hour),
	}))
	ctx := authContext(admin)
	d := func(x time.Duration) *core.Duration { v := core.Duration(x); return &v }
	s := func(v string) *string { return &v }
	refused := []struct {
		name string
		in   core.UpdateTenantInput
	}{
		{"a longer lifetime", core.UpdateTenantInput{SessionTTL: d(24 * time.Hour)}},
		{"a longer idle timeout", core.UpdateTenantInput{SessionIdleTimeout: d(2 * time.Hour)}},
		{"a longer remember", core.UpdateTenantInput{SessionRememberTTL: s("60d")}},
	}
	for _, tt := range refused {
		if _, err := l.UpdateTenant(ctx, "", tt.in); !core.IsKind(err, core.KindInvalid) {
			t.Errorf("%s: err = %v, want invalid", tt.name, err)
		}
	}
	tenant, err := l.UpdateTenant(ctx, "", core.UpdateTenantInput{
		SessionTTL: d(8 * time.Hour), SessionIdleTimeout: d(30 * time.Minute), SessionRememberTTL: s("off"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if tenant.SessionTTL.D() != 8*time.Hour || tenant.SessionIdleTimeout.D() != 30*time.Minute || !tenant.SessionRememberOff {
		t.Errorf("stored %+v", tenant)
	}
	p, err := l.SessionPolicy(loginContext(core.TenantScope{TenantID: admin.TenantID}))
	if err != nil {
		t.Fatalf("SessionPolicy without a sign-in: %v", err)
	}
	if p.TTL.D() != 8*time.Hour || p.IdleTimeout.D() != 30*time.Minute || p.RememberTTL != 0 || p.RememberOffered() {
		t.Errorf("policy = %+v", p)
	}
	if p.ServerTTL.D() != 12*time.Hour {
		t.Errorf("server ttl = %s, want 12h", p.ServerTTL)
	}
}

func TestKeepMeSignedInLastsTheRememberLifetime(t *testing.T) {
	l, clk, scope, admin := newLocal(t)
	seedUser(t, l, admin, "kept@example.com")

	result, err := l.Login(loginContext(scope), core.LoginInput{
		Email: "kept@example.com", Password: testPassword, Surface: core.SurfaceWeb, Remember: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Session.Remembered || !result.Session.ExpiresAt.Equal(clk.Now().Add(core.DefaultSessionRememberTTL.D())) {
		t.Errorf("remembered sign-in: remembered %v, expires %s", result.Session.Remembered, result.Session.ExpiresAt)
	}
	sessions, _, err := l.ListSessions(authContext(&core.Actor{ID: result.Session.ActorID, TenantID: scope.TenantID,
		Kind: core.ActorUser, Role: core.RoleMember}), core.Page{})
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || !sessions[0].Remembered {
		t.Errorf("sessions = %+v, want the one remembered", sessions)
	}

	off := "off"
	if _, err := l.UpdateTenant(authContext(admin), "", core.UpdateTenantInput{SessionRememberTTL: &off}); err != nil {
		t.Fatal(err)
	}
	result, err = l.Login(loginContext(scope), core.LoginInput{
		Email: "kept@example.com", Password: testPassword, Surface: core.SurfaceWeb, Remember: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Session.Remembered || !result.Session.ExpiresAt.Equal(clk.Now().Add(core.DefaultSessionTTL.D())) {
		t.Errorf("with keep me signed in off: remembered %v, expires %s, want an ordinary session",
			result.Session.Remembered, result.Session.ExpiresAt)
	}
}

func TestANewLifetimeLeavesLiveSessionsAlone(t *testing.T) {
	l, clk, scope, admin := newLocal(t)
	seedUser(t, l, admin, "ada@example.com")
	before, err := signIn(l, scope, "ada@example.com", testPassword)
	if err != nil {
		t.Fatal(err)
	}
	ttl := core.Duration(time.Hour)
	if _, err := l.UpdateTenant(authContext(admin), "", core.UpdateTenantInput{SessionTTL: &ttl}); err != nil {
		t.Fatal(err)
	}
	after, err := signIn(l, scope, "ada@example.com", testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ExpiresAt.Equal(clk.Now().Add(core.DefaultSessionTTL.D())) {
		t.Errorf("the earlier session's expiry moved to %s", before.ExpiresAt)
	}
	if !after.ExpiresAt.Equal(clk.Now().Add(time.Hour)) {
		t.Errorf("a sign-in after the change expires %s, want an hour from now", after.ExpiresAt)
	}
}
