// SPDX-License-Identifier: AGPL-3.0-or-later

package integration

import (
	"context"
	"slices"
	"testing"

	"github.com/heliopsy/tix/internal/client"
	"github.com/heliopsy/tix/internal/core"
)

// The path tix login --tenant and tix user key add --all-tenants take, against
// a served deployment on a host that serves several tenants: sign in naming
// the tenant in the header, trade the session for a token carrying
// account:read, list memberships with that token, and enrol one key in every
// tenant.
func TestATenantLoginOnASharedHostYieldsATokenThatReachesEveryMembership(t *testing.T) {
	w := newSharedHostWorld(t)
	ctx := context.Background()
	if _, err := w.h.local.CreateTenantUser(w.h.adminCtx, "default", core.CreateUserInput{
		Email: "ivy@example.test", Password: sharedPW, Role: core.RoleAdmin,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.h.local.AddTenantMember(w.h.adminCtx, "test", core.AddTenantMemberInput{
		Email: "ivy@example.test", Role: core.RoleAdmin,
	}); err != nil {
		t.Fatal(err)
	}

	anon, err := client.New(w.h.baseURL, "")
	if err != nil {
		t.Fatal(err)
	}
	result, err := anon.WithTenantHeader("test").Login(ctx, core.LoginInput{
		Email: "ivy@example.test", Password: sharedPW, Surface: core.SurfaceCLI,
	})
	if err != nil || result.Session == nil || result.Session.TenantID != w.testID {
		t.Fatalf("sign-in naming test = %+v, %v; want a session in test", result, err)
	}
	sess, err := client.New(w.h.baseURL, "", client.WithSessionCookie(result.Session.Token))
	if err != nil {
		t.Fatal(err)
	}
	issued, err := sess.WithTenantHeader("test").CreateToken(ctx, core.CreateTokenInput{
		Name: "cli", Scopes: []core.Scope{core.ScopeAll, core.ScopeAccountRead},
	})
	if err != nil {
		t.Fatalf("minting a token with the session = %v", err)
	}

	tok, err := client.New(w.h.baseURL, issued.Token)
	if err != nil {
		t.Fatal(err)
	}
	me, err := tok.WhoAmI(ctx)
	if err != nil || me.TenantKey != "test" || len(me.Memberships) != 2 {
		t.Fatalf("whoami with the token and no header = %+v, %v; want test and both memberships", me, err)
	}

	pub := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMBtWpANdXlq6pEw/ZGWCcsF7GLyPc5SjZ6y7/i0wZom"
	key, err := tok.EnrolSSHKey(ctx, core.EnrolSSHKeyInput{PublicKey: pub, AllTenants: true})
	if err != nil {
		t.Fatalf("enrolling in every tenant = %v", err)
	}
	if key.TenantID != w.testID || !slices.Contains(key.EnrolledIn, "default") || !slices.Contains(key.EnrolledIn, "test") {
		t.Fatalf("enrolment = %+v, want the test enrolment and both tenants listed", key)
	}

	plain, err := sess.WithTenantHeader("test").CreateToken(ctx, core.CreateTokenInput{
		Name: "plain", Scopes: []core.Scope{core.ScopeAll},
	})
	if err != nil {
		t.Fatal(err)
	}
	agent, err := client.New(w.h.baseURL, plain.Token)
	if err != nil {
		t.Fatal(err)
	}
	if me, err := agent.WhoAmI(ctx); err != nil || len(me.Memberships) != 0 {
		t.Errorf("a * token's whoami = %+v, %v; want no memberships", me, err)
	}
	if _, err := agent.EnrolSSHKey(ctx, core.EnrolSSHKeyInput{PublicKey: pub, AllTenants: true}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("an all-tenants enrolment with a * token = %v, want forbidden", err)
	}
}
