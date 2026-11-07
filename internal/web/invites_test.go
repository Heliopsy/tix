// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
)

// Invite links and the required password change in the browser, end to end,
// as plain form posts with a cookie jar and no script.

const newPassword = "a-brand-new-password"

// adminCtx is the fixture's administrator acting on its tenant directly.
func (f *fixture) adminCtx() context.Context {
	return core.WithTenant(core.WithActor(context.Background(),
		copyActor(f.actorA, []core.Scope{core.ScopeAll}, core.RoleAdmin)),
		core.TenantScope{TenantID: f.tenantA.ID})
}

// flaggedPerson creates an account that must change its password.
func (f *fixture) flaggedPerson(email, password string) core.User {
	f.t.Helper()
	user, err := f.svc.CreateUser(f.adminCtx(), core.CreateUserInput{
		Email: email, Password: password, Role: core.RoleMember, RequirePasswordChange: true})
	if err != nil {
		f.t.Fatalf("creating %s: %v", email, err)
	}
	return *user
}

// createDialog returns the create-a-user form, and only it.
func createDialog(t *testing.T, page string) string {
	t.Helper()
	return totpForm(t, page, "/admin/users")
}

// inviteLinkOn reads the link out of the one block that shows it.
func inviteLinkOn(t *testing.T, page string) string {
	t.Helper()
	return html.UnescapeString(between(t, page, `<pre id="invite-url" class="wraps"><code>`, "</code>"))
}

// issueInvite submits the create dialog in invite mode and follows the
// redirect to the screen that shows the link once.
func issueInvite(t *testing.T, b *browser, email string) (*http.Response, string) {
	t.Helper()
	resp := b.post("/admin/users", url.Values{
		"mode": {"invite"}, "email": {email}, "role": {"member"}, "expires": {"7d"},
		"password": {"ignored-in-invite-mode"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	shown := b.get(resp.Header.Get("Location"))
	return shown, body(t, shown)
}

func TestTheCreateDialogOffersAPasswordOrAnInviteLink(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	form := createDialog(t, f.as("alice").page("/admin/users"))
	for _, want := range []string{`name="mode" value="password" checked`, `name="mode" value="invite"`} {
		if !strings.Contains(form, want) {
			t.Errorf("the create form has no %s:\n%s", want, form)
		}
	}
	box := between(t, form, `<input id="require_password_change"`, ">")
	if !strings.Contains(box, "checked") {
		t.Errorf("the password change box is not ticked by default: %s", box)
	}
	if !strings.Contains(form, `<select id="expires" name="expires"`) {
		t.Errorf("the invite mode offers no expiry:\n%s", form)
	}
	if strings.Count(form, "<details") != strings.Count(form, `<details class="field-info tip">`) {
		t.Error("the create form hides a mode in an expander")
	}
}

func TestCreatingWithAPasswordCarriesTheBox(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for email, box := range map[string]string{"ticked@example.test": "1", "clear@example.test": ""} {
		form := url.Values{"mode": {"password"}, "email": {email}, "role": {"member"}, "password": {"correct-horse-battery"}}
		if box != "" {
			form.Set("require_password_change", box)
		}
		resp := b.post("/admin/users", form)
		_ = resp.Body.Close()
		wantStatus(t, resp, http.StatusSeeOther)
		result, err := f.svc.Login(core.WithTenant(context.Background(), core.TenantScope{TenantID: f.tenantA.ID}),
			core.LoginInput{Email: email, Password: "correct-horse-battery", Surface: core.SurfaceWeb})
		if err != nil {
			t.Fatalf("Login %s: %v", email, err)
		}
		challenged := result.Challenge != nil && result.Challenge.Kind == core.ChallengePasswordChange
		if challenged != (box != "") {
			t.Errorf("%s: challenged = %v, want %v", email, challenged, box != "")
		}
	}
}

func TestAnInviteShowsItsLinkOnceInADialog(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	resp, page := issueInvite(t, b, "ada@example.test")
	if resp.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Error("the screen showing the link sends a referrer")
	}
	dialog := between(t, page, `<dialog id="invite-link"`, "</dialog>")
	if !strings.Contains(dialog, " open") || !strings.Contains(dialog, `data-copy-target="invite-url"`) {
		t.Fatalf("the link is not shown in an open dialog with a Copy button:\n%s", dialog)
	}
	link := inviteLinkOn(t, dialog)
	if !strings.HasPrefix(link, f.server.URL+"/invite/"+auth.InviteTokenPrefix) {
		t.Fatalf("link = %q, want %s/invite/%s...", link, f.server.URL, auth.InviteTokenPrefix)
	}
	if !strings.Contains(dialog, "does not send email") {
		t.Error("the dialog does not say the link has to be passed on by hand")
	}
	again := b.page("/admin/users")
	if strings.Contains(again, link) || strings.Contains(again, `<dialog id="invite-link"`) {
		t.Error("the link is shown a second time")
	}
	pending := between(t, again, `<section class="invites"`, "</section>")
	if !strings.Contains(pending, "ada@example.test") || !strings.Contains(pending, "invited by alice") {
		t.Errorf("the pending invites do not list the address and its inviter:\n%s", pending)
	}
}

func TestPendingInvitesRevokeAndReissue(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	_, page := issueInvite(t, b, "ada@example.test")
	first := inviteLinkOn(t, page)
	id := between(t, totpForm(t, b.page("/admin/users"), "/admin/users/invites/reissue"), `name="id" value="`, `"`)

	resp := b.post("/admin/users/invites/reissue", url.Values{"id": {id}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	second := inviteLinkOn(t, body(t, b.get(resp.Header.Get("Location"))))
	if second == "" || second == first {
		t.Fatalf("re-issuing showed %q, want a new link", second)
	}
	anon := f.as("")
	if got := anon.get(strings.TrimPrefix(first, f.server.URL)); got.StatusCode != http.StatusNotFound {
		t.Errorf("the replaced link answers %d, want 404", got.StatusCode)
	}

	listed := b.page("/admin/users")
	newID := between(t, totpForm(t, listed, "/admin/users/invites/revoke"), `name="id" value="`, `"`)
	resp = b.post("/admin/users/invites/revoke", url.Values{"id": {newID}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	if strings.Contains(b.page("/admin/users"), `<section class="invites"`) {
		t.Error("a revoked invite is still listed")
	}
	if got := anon.get(strings.TrimPrefix(second, f.server.URL)); got.StatusCode != http.StatusNotFound {
		t.Errorf("the revoked link answers %d, want 404", got.StatusCode)
	}
}

// csrfFree drops the per-browser CSRF value, the one part of a page that
// differs between two visitors shown the same thing.
var csrfFree = regexp.MustCompile(`name="csrf_token" value="[^"]*"`)

func TestAnInviteIsAcceptedOnceAndSignsIn(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	_, page := issueInvite(t, f.as("alice"), "ada@example.test")
	path := strings.TrimPrefix(inviteLinkOn(t, page), f.server.URL)

	visitor := f.as("")
	opened := visitor.get(path)
	form := totpForm(t, body(t, opened), path)
	if opened.StatusCode != http.StatusOK || !strings.Contains(form, `name="confirm"`) || !strings.Contains(form, `name="display_name"`) {
		t.Fatalf("the invite page answered %d without the set-a-password form:\n%s", opened.StatusCode, form)
	}
	if opened.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Error("the invite page sends a referrer")
	}

	mismatch := visitor.post(path, url.Values{"password": {newPassword}, "confirm": {"something-else-entirely"}})
	if mismatch.StatusCode != http.StatusUnprocessableEntity || strings.Contains(mismatch.Header.Get("Set-Cookie"), auth.SessionCookieName) {
		t.Fatalf("mismatched passwords answered %d", mismatch.StatusCode)
	}
	_ = mismatch.Body.Close()

	accepted := visitor.post(path, url.Values{"password": {newPassword}, "confirm": {newPassword}, "display_name": {"Ada"}})
	_ = accepted.Body.Close()
	wantStatus(t, accepted, http.StatusSeeOther)
	if !strings.Contains(accepted.Header.Get("Set-Cookie"), auth.SessionCookieName) {
		t.Fatal("accepting issued no session cookie")
	}
	if loc := accepted.Header.Get("Location"); loc != "/projects" {
		t.Errorf("accepting sent the reader to %q, want /projects", loc)
	}

	used := f.as("").get(path)
	usedPage := body(t, used)
	if used.StatusCode != http.StatusNotFound || !strings.Contains(usedPage, "This link does not work") {
		t.Fatalf("the used link answered %d:\n%s", used.StatusCode, usedPage)
	}
	unknown := f.as("").get("/invite/" + auth.InviteTokenPrefix + strings.Repeat("a", 52))
	unknownPage := body(t, unknown)
	if unknown.StatusCode != used.StatusCode || csrfFree.ReplaceAllString(unknownPage, "") != csrfFree.ReplaceAllString(usedPage, "") {
		t.Errorf("an unknown link and a used one answer differently:\nused %d\n%s\nunknown %d\n%s",
			used.StatusCode, usedPage, unknown.StatusCode, unknownPage)
	}
}

func TestAFlaggedSignInMustChooseANewPassword(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.flaggedPerson("ada@example.test", "correct-horse-battery")
	b := f.as("")

	resp := b.post("/login", url.Values{"email": {"ada@example.test"}, "password": {"correct-horse-battery"}, "next": {"/tasks"}})
	page := body(t, resp)
	if strings.Contains(resp.Header.Get("Set-Cookie"), auth.SessionCookieName) {
		t.Fatal("a flagged account was given a session before changing its password")
	}
	form := totpForm(t, page, "/login")
	if !strings.Contains(page, "Choose a new password") || !strings.Contains(form, `name="new_password"`) {
		t.Fatalf("the sign-in did not stop at the password change page:\n%s", page)
	}
	if hidden(t, form, "email") != "ada@example.test" || hidden(t, form, "next") != "/tasks" {
		t.Errorf("the page does not carry the address and destination:\n%s", form)
	}
	if !strings.Contains(page, `<a href="/login">`) {
		t.Error("the page offers no way back to sign-in")
	}
	away := b.get("/projects")
	_ = away.Body.Close()
	if away.StatusCode != http.StatusSeeOther || !strings.HasPrefix(away.Header.Get("Location"), "/login") {
		t.Errorf("another screen answered %d %q, want a redirect to sign-in", away.StatusCode, away.Header.Get("Location"))
	}

	change := func(current, next, confirm string) *http.Response {
		return b.post("/login", url.Values{"email": {"ada@example.test"}, "next": {"/tasks"},
			"password": {current}, "new_password": {next}, "confirm": {confirm}})
	}
	for name, r := range map[string]*http.Response{
		"mismatched": change("correct-horse-battery", newPassword, "not-the-same-password"),
		"the same":   change("correct-horse-battery", "correct-horse-battery", "correct-horse-battery"),
		"too short":  change("correct-horse-battery", "short", "short"),
	} {
		refused := body(t, r)
		if r.StatusCode < 400 || strings.Contains(r.Header.Get("Set-Cookie"), auth.SessionCookieName) {
			t.Errorf("%s: answered %d with a session", name, r.StatusCode)
		}
		if !strings.Contains(refused, `name="new_password"`) || !strings.Contains(refused, `class="field-error"`) {
			t.Errorf("%s: the page was not redrawn with the refusal:\n%s", name, refused)
		}
	}

	done := change("correct-horse-battery", newPassword, newPassword)
	_ = done.Body.Close()
	wantStatus(t, done, http.StatusSeeOther)
	if !strings.Contains(done.Header.Get("Set-Cookie"), auth.SessionCookieName) || done.Header.Get("Location") != "/tasks" {
		t.Errorf("the change answered cookie %q location %q", done.Header.Get("Set-Cookie"), done.Header.Get("Location"))
	}
	again := f.as("").post("/login", url.Values{"email": {"ada@example.test"}, "password": {newPassword}})
	_ = again.Body.Close()
	if again.StatusCode != http.StatusSeeOther || !strings.Contains(again.Header.Get("Set-Cookie"), auth.SessionCookieName) {
		t.Errorf("signing in with the new password answered %d", again.StatusCode)
	}
}

func TestTheManageDialogSetsThePasswordChange(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	user := f.person("ada@example.test", "correct-horse-battery")
	b := f.as("alice")
	box := func() string {
		return between(t, b.page("/admin/users"), `<input id="pwchange-`+user.ID+`"`, ">")
	}
	if strings.Contains(box(), "checked") {
		t.Fatal("the box is ticked for an account without the flag")
	}
	for _, want := range []bool{true, false} {
		form := url.Values{"id": {user.ID}, "display_name": {""}}
		if want {
			form.Set("require_password_change", "1")
		}
		resp := b.post("/admin/users/update", form)
		_ = resp.Body.Close()
		wantStatus(t, resp, http.StatusSeeOther)
		got, err := f.svc.GetUser(f.adminCtx(), user.ID)
		if err != nil {
			t.Fatalf("GetUser: %v", err)
		}
		if got.RequirePasswordChange != want || strings.Contains(box(), "checked") != want {
			t.Errorf("after saving %v the flag is %v and the box %q", want, got.RequirePasswordChange, box())
		}
	}
}

// An invite to a person of another tenant who holds a second factor asks for
// a code on the invite page, and the password alone does not sign them in.
func TestAnInviteToAnAccountWithASecondFactorAsksForItsCode(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	bob := core.WithTenant(core.WithActor(context.Background(),
		copyActor(f.actorB, []core.Scope{core.ScopeAll}, core.RoleAdmin)),
		core.TenantScope{TenantID: f.tenantB.ID})
	pat, err := f.svc.CreateUser(bob, core.CreateUserInput{Email: "pat@other.test", Password: newPassword})
	if err != nil {
		t.Fatalf("creating pat in the other tenant: %v", err)
	}
	patCtx := core.WithActor(context.Background(), &core.Actor{ID: pat.ID, TenantID: f.tenantB.ID,
		Kind: core.ActorUser, Role: core.RoleMember})
	enrolment, err := f.svc.BeginTOTPEnrolment(patCtx)
	if err != nil {
		t.Fatalf("beginning pat's enrolment: %v", err)
	}
	code := func() string {
		c, err := auth.TOTPCode(enrolment.Secret, auth.TOTPStep(f.clock.Now()))
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	if _, err := f.svc.ConfirmTOTPEnrolment(patCtx, code()); err != nil {
		t.Fatalf("confirming pat's enrolment: %v", err)
	}

	_, page := issueInvite(t, f.as("alice"), "pat@other.test")
	path := strings.TrimPrefix(inviteLinkOn(t, page), f.server.URL)
	visitor := f.as("")
	form := totpForm(t, body(t, visitor.get(path)), path)
	if !strings.Contains(form, `name="code"`) || strings.Contains(form, `name="confirm"`) {
		t.Fatalf("the invite page for pat does not ask for her password and a code:\n%s", form)
	}

	bare := visitor.post(path, url.Values{"password": {newPassword}})
	if bare.StatusCode == http.StatusSeeOther || strings.Contains(bare.Header.Get("Set-Cookie"), auth.SessionCookieName) {
		t.Fatalf("the password alone accepted the invite: %d", bare.StatusCode)
	}
	if refusal := body(t, bare); !strings.Contains(refusal, "needs a code") {
		t.Errorf("the refusal does not ask for the code:\n%s", refusal)
	}

	f.clock.Advance(30 * time.Second)
	accepted := visitor.post(path, url.Values{"password": {newPassword}, "code": {code()}})
	_ = accepted.Body.Close()
	wantStatus(t, accepted, http.StatusSeeOther)
	if !strings.Contains(accepted.Header.Get("Set-Cookie"), auth.SessionCookieName) {
		t.Fatal("accepting with the password and a code issued no session cookie")
	}
}
