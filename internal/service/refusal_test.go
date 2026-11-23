// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
)

// noted runs fn with a refusal note in its context and returns the reason
// noted, or "".
func noted(ctx context.Context, fn func(context.Context)) auth.Refusal {
	ctx, note := auth.WithRefusalNote(ctx)
	fn(ctx)
	r, _ := note.Get()
	return r
}

func TestARefusedPasswordSignInNotesWhy(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	seedUser(t, l, admin, "ada@example.com")
	disabled := seedUser(t, l, admin, "grace@example.com")
	yes := true
	if _, err := l.UpdateUser(authContext(admin), disabled.ID, core.UpdateUserInput{Disabled: &yes}); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ email, password, want string }{
		{"ada@example.com", "not-the-password", auth.RefusedBadPassword},
		{"nobody@example.com", testPassword, auth.RefusedUnknownAccount},
		{"grace@example.com", testPassword, auth.RefusedUnknownAccount},
		{"ada@example.com", testPassword, ""},
	}
	for _, tc := range cases {
		t.Run(tc.email+"/"+tc.want, func(t *testing.T) {
			got := noted(loginContext(scope), func(ctx context.Context) {
				_, _ = l.Login(ctx, core.LoginInput{Email: tc.email, Password: tc.password, Surface: core.SurfaceCLI})
			})
			if got.Reason != tc.want {
				t.Errorf("noted %q, want %q", got.Reason, tc.want)
			}
		})
	}
}

func TestARefusedSecondFactorNotesWhy(t *testing.T) {
	l, clk, scope, admin := newTOTPLocal(t)
	user := seedUser(t, l, admin, "ada@example.com")
	actor := userActor(t, l, user, core.RoleMember)
	enrol(t, l, clk, actor)
	requireTOTP(t, l, scope, core.SurfaceCLI)

	attempt := func(ticket string) string {
		t.Helper()
		if ticket == "" {
			first, err := l.Login(loginContext(scope), core.LoginInput{
				Email: user.Email, Password: testPassword, Surface: core.SurfaceCLI,
			})
			if err != nil {
				t.Fatal(err)
			}
			ticket = first.Challenge.Ticket
		}
		return noted(loginContext(scope), func(ctx context.Context) {
			_, _ = l.Login(ctx, core.LoginInput{Surface: core.SurfaceCLI, Ticket: ticket, Code: "000000"})
		}).Reason
	}
	if got := attempt("not-a-ticket"); got != auth.RefusedSignInExpired {
		t.Errorf("an unreadable ticket noted %q, want %q", got, auth.RefusedSignInExpired)
	}
	if got := attempt(""); got != auth.RefusedTOTPInvalid {
		t.Errorf("a wrong code noted %q, want %q", got, auth.RefusedTOTPInvalid)
	}
	for range auth.TOTPMaxFailures {
		attempt("")
	}
	if got := attempt(""); got != auth.RefusedTOTPLocked {
		t.Errorf("a code while locked noted %q, want %q", got, auth.RefusedTOTPLocked)
	}
}

func TestASecondFactorThatCouldNotBeCheckedNotesNoRefusal(t *testing.T) {
	l, clk, scope, admin := newTOTPLocal(t)
	user := seedUser(t, l, admin, "ada@example.com")
	enrol(t, l, clk, userActor(t, l, user, core.RoleMember))
	requireTOTP(t, l, scope, core.SurfaceCLI)
	first, err := l.Login(loginContext(scope), core.LoginInput{Email: user.Email, Password: testPassword, Surface: core.SurfaceCLI})
	if err != nil {
		t.Fatal(err)
	}
	l.totpSealer = nil
	var loginErr error
	got := noted(loginContext(scope), func(ctx context.Context) {
		_, loginErr = l.Login(ctx, core.LoginInput{Surface: core.SurfaceCLI, Ticket: first.Challenge.Ticket, Code: "000000"})
	})
	if loginErr == nil || core.IsKind(loginErr, core.KindUnauthenticated) {
		t.Fatalf("login with no sealer = %v, want an error that is not a refused code", loginErr)
	}
	if got.Reason != "" {
		t.Errorf("a code that could not be checked noted %q, want no refusal", got.Reason)
	}
}

func TestAPolicyRefusalNotesScopeOrProject(t *testing.T) {
	policy := authz.New()
	reader := &core.Actor{ID: "a", TenantID: "t", TokenID: "tok-1", TokenName: "reader",
		Scopes: []core.Scope{core.ScopeTaskRead, core.ScopeProjectRead}}
	pinned := *reader
	pinned.ProjectID = "p1"
	cases := []struct {
		name   string
		actor  *core.Actor
		action authz.Action
		res    authz.Resource
		want   string
	}{
		{"missing scope", reader, authz.ActionProjectWrite, authz.Resource{TenantID: "t"}, auth.RefusedScopeDenied},
		{"another project", &pinned, authz.ActionProjectRead, authz.Resource{TenantID: "t", ProjectID: "p2"},
			auth.RefusedProjectDenied},
		{"pinned and missing scope", &pinned, authz.ActionProjectWrite, authz.Resource{TenantID: "t", ProjectID: "p1"},
			auth.RefusedScopeDenied},
		{"another tenant", reader, authz.ActionProjectRead, authz.Resource{TenantID: "u"}, ""},
		{"allowed", reader, authz.ActionProjectRead, authz.Resource{TenantID: "t"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := noted(context.Background(), func(ctx context.Context) {
				noteDenied(ctx, tc.actor, policy.Can(tc.actor, tc.action, tc.res))
			})
			if got.Reason != tc.want {
				t.Errorf("noted %q, want %q", got.Reason, tc.want)
			}
			if tc.want != "" && (got.TokenID != "tok-1" || got.TokenName != "reader") {
				t.Errorf("noted %+v, want the token named", got)
			}
		})
	}
}
