// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

const changedPassword = "a-brand-new-password"

// seedFlaggedUser creates a member who must change their password.
func seedFlaggedUser(t *testing.T, l *Local, admin *core.Actor, email string) *core.User {
	t.Helper()
	u, err := l.CreateUser(authContext(admin), core.CreateUserInput{
		Email: email, Password: testPassword, Role: core.RoleMember, RequirePasswordChange: true,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if !u.RequirePasswordChange {
		t.Fatal("the created user does not carry the flag")
	}
	return u
}

// subjectAudit lists the actions recorded against one subject, oldest first.
func subjectAudit(t *testing.T, l *Local, scope core.TenantScope, subjectID string) []string {
	t.Helper()
	ctx := context.Background()
	var out []string
	if err := l.store.View(ctx, scope, func(tx store.Tx) error {
		entries, err := tx.ListAudit(ctx, core.AuditFilter{Page: core.Page{Limit: 1000}})
		for _, e := range entries {
			if e.SubjectID == subjectID {
				out = append(out, e.Action)
			}
		}
		return err
	}); err != nil {
		t.Fatalf("reading audit: %v", err)
	}
	return out
}

func TestAFlaggedAccountIsChallengedAndGetsNoSession(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	seedFlaggedUser(t, l, admin, "ada@example.com")

	result, err := l.Login(loginContext(scope), core.LoginInput{
		Email: "ada@example.com", Password: testPassword, Surface: core.SurfaceWeb,
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if result.Session != nil {
		t.Fatal("a flagged account was handed a session without changing its password")
	}
	if result.Challenge == nil || result.Challenge.Kind != core.ChallengePasswordChange {
		t.Fatalf("challenge = %+v, want password_change", result.Challenge)
	}

	_, wrongErr := signIn(l, scope, "ada@example.com", "not-the-password-at-all")
	_, unknownErr := signIn(l, scope, "nobody@example.com", "not-the-password-at-all")
	if wrongErr == nil || unknownErr == nil || wrongErr.Error() != unknownErr.Error() {
		t.Errorf("a wrong password on a flagged account answered %v, an unknown one %v; want the same", wrongErr, unknownErr)
	}
}

func TestTheReplacementPasswordIsCheckedThenClearsTheFlag(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	user := seedFlaggedUser(t, l, admin, "ada@example.com")
	login := func(newPassword string) (*core.LoginResult, error) {
		return l.Login(loginContext(scope), core.LoginInput{
			Email: "ada@example.com", Password: testPassword, NewPassword: newPassword, Surface: core.SurfaceCLI,
		})
	}

	for name, bad := range map[string]string{"too short": "short", "the same": testPassword} {
		if _, err := login(bad); !core.IsKind(err, core.KindInvalid) {
			t.Errorf("%s new password: err = %v, want invalid", name, err)
		}
	}
	if got, _ := l.GetUser(authContext(admin), user.ID); !got.RequirePasswordChange {
		t.Fatal("a refused new password cleared the flag")
	}

	result, err := login(changedPassword)
	if err != nil {
		t.Fatalf("Login with a new password: %v", err)
	}
	if result.Session == nil {
		t.Fatal("changing the password did not sign the person in")
	}
	if got, _ := l.GetUser(authContext(admin), user.ID); got.RequirePasswordChange {
		t.Error("the flag survived the change")
	}
	if _, err := signIn(l, scope, "ada@example.com", testPassword); err == nil {
		t.Error("the old password still signs in")
	}
	session, err := signIn(l, scope, "ada@example.com", changedPassword)
	if err != nil || session == nil {
		t.Fatalf("the new password does not sign in on its own: %v", err)
	}
	actions := strings.Join(subjectAudit(t, l, scope, user.ID), ",")
	if !strings.Contains(actions, auditUserPasswordChange) {
		t.Errorf("audit for the user = %s, want a %s entry", actions, auditUserPasswordChange)
	}
}

func TestANewPasswordIsRefusedWhenNoChangeIsPending(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	seedUser(t, l, admin, "ada@example.com")
	_, err := l.Login(loginContext(scope), core.LoginInput{
		Email: "ada@example.com", Password: testPassword, NewPassword: changedPassword,
	})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("err = %v, want invalid", err)
	}
}

func TestAnAdministratorSetsAndClearsTheFlagOnEdit(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	user := seedUser(t, l, admin, "ada@example.com")
	on := true
	if _, err := l.UpdateUser(authContext(admin), user.ID, core.UpdateUserInput{RequirePasswordChange: &on}); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	result, err := l.Login(loginContext(scope), core.LoginInput{Email: "ada@example.com", Password: testPassword})
	if err != nil || result.Challenge == nil || result.Challenge.Kind != core.ChallengePasswordChange {
		t.Fatalf("after setting the flag Login = %+v, %v; want a password_change challenge", result, err)
	}
	if !strings.Contains(historyText(t, l, scope), `"require_password_change":true`) {
		t.Error("setting the flag is not in the audit trail")
	}
	off := false
	if _, err := l.UpdateUser(authContext(admin), user.ID, core.UpdateUserInput{RequirePasswordChange: &off}); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if session, err := signIn(l, scope, "ada@example.com", testPassword); err != nil || session == nil {
		t.Fatalf("after clearing the flag sign-in = %v, %v", session, err)
	}
}

func TestRequiringAChangeNeedsAPassword(t *testing.T) {
	l, _, _, admin := newLocal(t)
	_, err := l.CreateUser(authContext(admin), core.CreateUserInput{
		Email: "ada@example.com", RequirePasswordChange: true,
	})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("err = %v, want invalid", err)
	}
}

func invite(t *testing.T, l *Local, admin *core.Actor, in core.CreateInviteInput) *core.IssuedInvite {
	t.Helper()
	issued, err := l.CreateInvite(authContext(admin), in)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	return issued
}

func TestAnInviteIsAcceptedOnceAndSignsIn(t *testing.T) {
	l, clk, scope, admin := newLocal(t)
	issued := invite(t, l, admin, core.CreateInviteInput{Email: "Ada@Example.com", Role: core.RoleMember})

	if !strings.HasPrefix(issued.Token, auth.InviteTokenPrefix) || issued.Path != "/invite/"+issued.Token {
		t.Fatalf("issued = %+v, want a tix_inv_ token at /invite/TOKEN", issued)
	}
	if want := clk.Now().Add(core.DefaultInviteExpiry); !issued.ExpiresAt.Equal(want) {
		t.Errorf("expiry = %s, want %s", issued.ExpiresAt, want)
	}
	if hash := storedPasswordHash(t, l, "ada@example.com"); hash != "" {
		t.Fatal("the invited account was given a password")
	}
	listed, err := l.ListInvites(authContext(admin))
	if err != nil || len(listed) != 1 || listed[0].Email != "ada@example.com" || listed[0].InvitedByHandle != admin.Handle {
		t.Fatalf("ListInvites = %+v, %v", listed, err)
	}

	ctx := loginContext(scope)
	preview, err := l.PreviewInvite(ctx, issued.Token)
	if err != nil || preview.Email != "ada@example.com" || preview.ExistingAccount {
		t.Fatalf("PreviewInvite = %+v, %v", preview, err)
	}
	if _, err := l.AcceptInvite(ctx, issued.Token, core.AcceptInviteInput{Password: "short"}); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("a short password: err = %v, want invalid", err)
	}
	result, err := l.AcceptInvite(ctx, issued.Token, core.AcceptInviteInput{
		Password: changedPassword, DisplayName: "Ada Lovelace", Surface: core.SurfaceWeb,
	})
	if err != nil || result.Session == nil {
		t.Fatalf("AcceptInvite = %+v, %v; want a session", result, err)
	}
	if _, err := l.AcceptInvite(ctx, issued.Token, core.AcceptInviteInput{Password: changedPassword}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("a used link: err = %v, want not found", err)
	}
	if session, err := signIn(l, scope, "ada@example.com", changedPassword); err != nil || session == nil {
		t.Fatalf("the chosen password does not sign in: %v", err)
	}
	user, err := l.GetUser(authContext(admin), issued.UserID)
	if err != nil || user.DisplayName != "Ada Lovelace" {
		t.Errorf("user = %+v, %v; want the chosen display name", user, err)
	}
	if listed, _ := l.ListInvites(authContext(admin)); len(listed) != 0 {
		t.Errorf("an accepted invite is still listed: %+v", listed)
	}
}

func TestADeadLinkAnswersLikeAnUnknownOne(t *testing.T) {
	l, clk, scope, admin := newLocal(t)
	ctx := loginContext(scope)
	_, unknownErr := l.PreviewInvite(ctx, auth.InviteTokenPrefix+strings.Repeat("a", 52))
	if !core.IsKind(unknownErr, core.KindNotFound) {
		t.Fatalf("unknown token: err = %v, want not found", unknownErr)
	}
	_, malformedErr := l.PreviewInvite(ctx, "nonsense")

	expired := invite(t, l, admin, core.CreateInviteInput{Email: "old@example.com", ExpiresIn: core.Duration(time.Hour)})
	revoked := invite(t, l, admin, core.CreateInviteInput{Email: "gone@example.com"})
	if err := l.RevokeInvite(authContext(admin), revoked.ID); err != nil {
		t.Fatalf("RevokeInvite: %v", err)
	}
	clk.Advance(2 * time.Hour)
	_, expiredErr := l.AcceptInvite(ctx, expired.Token, core.AcceptInviteInput{Password: changedPassword})
	_, revokedErr := l.PreviewInvite(ctx, revoked.Token)

	for name, err := range map[string]error{"malformed": malformedErr, "expired": expiredErr, "revoked": revokedErr} {
		if err == nil || err.Error() != unknownErr.Error() || core.KindOf(err) != core.KindNotFound {
			t.Errorf("%s token answered %v, want %v", name, err, unknownErr)
		}
	}
	if hash := storedPasswordHash(t, l, "gone@example.com"); hash != "" {
		t.Error("revoking set a password")
	}
}

func TestReissuingStopsTheOldLink(t *testing.T) {
	l, clk, scope, admin := newLocal(t)
	first := invite(t, l, admin, core.CreateInviteInput{Email: "ada@example.com"})
	clk.Advance(time.Minute)
	second, err := l.ReissueInvite(authContext(admin), first.ID, core.ReissueInviteInput{ExpiresIn: core.Duration(48 * time.Hour)})
	if err != nil {
		t.Fatalf("ReissueInvite: %v", err)
	}
	if second.Token == first.Token || second.UserID != first.UserID {
		t.Fatalf("re-issued = %+v, want a new token for the same user", second)
	}
	ctx := loginContext(scope)
	if _, err := l.PreviewInvite(ctx, first.Token); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("the replaced link: err = %v, want not found", err)
	}
	if _, err := l.PreviewInvite(ctx, second.Token); err != nil {
		t.Errorf("the new link: %v", err)
	}
	if _, err := l.ReissueInvite(authContext(admin), first.ID, core.ReissueInviteInput{}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("re-issuing a replaced invite: err = %v, want not found", err)
	}
}

func TestInviteExpiryIsBounded(t *testing.T) {
	l, _, _, admin := newLocal(t)
	for _, d := range []time.Duration{time.Minute, 31 * 24 * time.Hour} {
		_, err := l.CreateInvite(authContext(admin), core.CreateInviteInput{Email: "ada@example.com", ExpiresIn: core.Duration(d)})
		if !core.IsKind(err, core.KindInvalid) {
			t.Errorf("expiry %s: err = %v, want invalid", d, err)
		}
	}
}

func TestInvitingAnExistingAccountKeepsItsPassword(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	user := seedUser(t, l, admin, "ada@example.com")
	before := storedPasswordHash(t, l, "ada@example.com")
	if _, err := l.CreateInvite(authContext(admin), core.CreateInviteInput{Email: "ada@example.com"}); !core.IsKind(err, core.KindConflict) {
		t.Fatalf("inviting a member: err = %v, want conflict", err)
	}
	if err := l.RemoveMember(authContext(admin), user.ID); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}

	issued := invite(t, l, admin, core.CreateInviteInput{Email: "ada@example.com", Role: core.RoleMember})
	if !issued.ExistingAccount {
		t.Error("the invite does not say the account exists")
	}
	if role, err := l.memberRole(scope, user.ID); err != nil || role != core.RoleMember {
		t.Fatalf("role after the invite = %q, %v; want member", role, err)
	}
	if after := storedPasswordHash(t, l, "ada@example.com"); after != before {
		t.Fatal("inviting an existing account changed its password")
	}
	ctx := loginContext(scope)
	if _, err := l.AcceptInvite(ctx, issued.Token, core.AcceptInviteInput{Password: changedPassword}); !core.IsKind(err, core.KindUnauthenticated) {
		t.Fatalf("accepting with a password that is not theirs: err = %v, want unauthenticated", err)
	}
	result, err := l.AcceptInvite(ctx, issued.Token, core.AcceptInviteInput{Password: testPassword})
	if err != nil || result.Session == nil {
		t.Fatalf("AcceptInvite = %+v, %v", result, err)
	}
	if after := storedPasswordHash(t, l, "ada@example.com"); after != before {
		t.Fatal("accepting changed an existing account's password")
	}
}

// An address of another tenant is invited, not added: the invite names a new
// actor here, and the person holds no membership and cannot sign in here until
// they accept with their own password.
func TestAnAddressOfAnotherTenantIsInvitedNotAdded(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	other := otherTenantActor(t, l)
	if _, err := l.CreateUser(authContext(other), core.CreateUserInput{Email: "carol@example.com", Password: testPassword}); err != nil {
		t.Fatalf("CreateUser in the other tenant: %v", err)
	}
	issued := invite(t, l, admin, core.CreateInviteInput{Email: "carol@example.com", Role: core.RoleMember})
	if !issued.GrantsMembership || !issued.ExistingAccount {
		t.Fatalf("issued = %+v, want an invite that grants its membership on acceptance", issued.Invite)
	}
	if _, err := signIn(l, scope, "carol@example.com", testPassword); err == nil {
		t.Fatal("carol signs in to a tenant she has only been invited to")
	}

	ctx := loginContext(scope)
	if _, err := l.AcceptInvite(ctx, issued.Token, core.AcceptInviteInput{Password: "not-her-password-at-all"}); !core.IsKind(err, core.KindUnauthenticated) {
		t.Fatalf("accepting with the wrong password = %v, want unauthenticated", err)
	}
	if _, err := signIn(l, scope, "carol@example.com", testPassword); err == nil {
		t.Fatal("a refused acceptance granted the membership")
	}
	result, err := l.AcceptInvite(ctx, issued.Token, core.AcceptInviteInput{Password: testPassword})
	if err != nil || result.Session == nil {
		t.Fatalf("accepting with her password = %+v, %v; want a session", result, err)
	}
	if result.Session.ActorID != issued.UserID || result.Session.TenantID != scope.TenantID {
		t.Fatalf("session = %+v, want her new actor %q in this tenant", result.Session, issued.UserID)
	}
	if _, err := signIn(l, scope, "carol@example.com", testPassword); err != nil {
		t.Fatalf("signing in after accepting = %v", err)
	}
}
func TestInvitesNeedUserAdmin(t *testing.T) {
	l, _, scope, _ := newLocal(t)
	viewer := &core.Actor{ID: "viewer", TenantID: scope.TenantID, Kind: core.ActorUser, Role: core.RoleViewer}
	if _, err := l.CreateInvite(authContext(viewer), core.CreateInviteInput{Email: "ada@example.com"}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("CreateInvite as a viewer: err = %v, want forbidden", err)
	}
	if _, err := l.ListInvites(authContext(viewer)); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("ListInvites as a viewer: err = %v, want forbidden", err)
	}
}

func TestNoInviteTokenReachesHistory(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	first := invite(t, l, admin, core.CreateInviteInput{Email: "ada@example.com"})
	second, err := l.ReissueInvite(authContext(admin), first.ID, core.ReissueInviteInput{})
	if err != nil {
		t.Fatalf("ReissueInvite: %v", err)
	}
	if _, err := l.AcceptInvite(loginContext(scope), second.Token, core.AcceptInviteInput{Password: changedPassword}); err != nil {
		t.Fatalf("AcceptInvite: %v", err)
	}
	history := historyText(t, l, scope)
	if !strings.Contains(history, auditInviteAccept) {
		t.Fatal("the history holds no acceptance; the test proves nothing")
	}
	for _, secret := range []string{first.Token, auth.HashToken(first.Token), second.Token, auth.HashToken(second.Token), changedPassword} {
		if strings.Contains(history, secret) {
			t.Errorf("%q appears in the audit log or an event payload", secret)
		}
	}
}

// memberRole reads an actor's membership role directly from the store.
func (l *Local) memberRole(scope core.TenantScope, actorID string) (core.Role, error) {
	var role core.Role
	err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		var err error
		role, err = userRole(context.Background(), tx, actorID)
		return err
	})
	return role, err
}
