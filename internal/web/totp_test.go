// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"html"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// The second factor in the browser, end to end: the code step of a sign-in,
// enrolment from settings, and what an administrator may do to somebody
// else's enrolment.
//
// Every request below is a plain form submission with a cookie jar, which is
// exactly what a browser with scripting turned off sends. Nothing in this
// file runs a script, so a flow that passes here is a flow that works
// without one.

// person creates a credentialed account through the service, so the actor
// behind the user is minted the way a sign-in expects to find it.
func (f *fixture) person(email, password string) core.User {
	f.t.Helper()
	ctx := core.WithTenant(core.WithActor(context.Background(),
		copyActor(f.actorA, []core.Scope{core.ScopeAll}, core.RoleAdmin)),
		core.TenantScope{TenantID: f.tenantA.ID})
	user, err := f.svc.CreateUser(ctx, core.CreateUserInput{
		Email: email, Password: password, Role: core.RoleMember})
	if err != nil {
		f.t.Fatalf("creating %s: %v", email, err)
	}
	return *user
}

// requireTOTPWeb turns the browser requirement on for the fixture's tenant.
func (f *fixture) requireTOTPWeb() {
	f.t.Helper()
	ctx := context.Background()
	tenant := f.tenantA
	tenant.RequireTOTPWeb = true
	if err := f.store.Unscoped(ctx, func(u store.UnscopedTx) error {
		return u.UpdateTenant(ctx, &tenant)
	}); err != nil {
		f.t.Fatalf("requiring a second factor in the browser: %v", err)
	}
}

// totpForm returns the markup of the one form posting to action, so an
// assertion about a control is an assertion about that control rather than
// about the page it sits on.
func totpForm(t *testing.T, page, action string) string {
	t.Helper()
	from := strings.Index(page, `action="`+action+`"`)
	if from < 0 {
		t.Fatalf("the page has no form posting to %s:\n%s", action, page)
	}
	// Back to the opening tag, so the method is part of what is returned: a
	// control is only a working control together with the form around it.
	if start := strings.LastIndex(page[:from], "<form"); start >= 0 {
		from = start
	}
	to := strings.Index(page[from:], "</form>")
	if to < 0 {
		t.Fatalf("the form posting to %s is not closed", action)
	}
	return page[from : from+to]
}

// hidden reads one hidden field out of a form.
func hidden(t *testing.T, form, name string) string {
	t.Helper()
	marker := `name="` + name + `" value="`
	from := strings.Index(form, marker)
	if from < 0 {
		t.Fatalf("the form carries no %q field:\n%s", name, form)
	}
	rest := form[from+len(marker):]
	to := strings.Index(rest, `"`)
	if to < 0 {
		t.Fatalf("the %q field is not closed", name)
	}
	return html.UnescapeString(rest[:to])
}

// secretIn reads the shared secret out of the otpauth URI an enrolment screen
// carries forward.
func secretIn(t *testing.T, uri string) string {
	t.Helper()
	parsed, err := url.Parse(uri)
	if err != nil {
		t.Fatalf("parsing %q: %v", uri, err)
	}
	secret := parsed.Query().Get("secret")
	if secret == "" {
		t.Fatalf("%q carries no secret", uri)
	}
	return secret
}

// codeFor is the code an authenticator holding secret shows, one time step
// on from the last one asked for. The step moves because a code is single
// use: an enrolment that accepted one at this instant refuses it again, and
// a test that reused it would be testing the replay guard by accident.
func codeFor(t *testing.T, f *fixture, secret string) string {
	t.Helper()
	f.clock.Advance(auth.TOTPPeriod)
	code, err := auth.TOTPCode(secret, auth.TOTPStep(f.clock.Now()))
	if err != nil {
		t.Fatalf("generating a code: %v", err)
	}
	return code
}

// A tenant that turns the requirement on must not lock out the people it
// applies to, so an unenrolled account is taken through enrolment in the
// browser and signed in at the end of it, with its recovery codes shown
// once on the way.
func TestSignInEnrolsWhenTheTenantRequiresAFactor(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.requireTOTPWeb()
	f.person("newcomer@example.test", "correct-horse-battery")
	b := f.as("")

	resp := b.post("/login", url.Values{
		"email": {"newcomer@example.test"}, "password": {"correct-horse-battery"},
		"next": {"/tasks"}})
	step := body(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the password step answered %d, want the enrolment step", resp.StatusCode)
	}
	if strings.Contains(resp.Header.Get("Set-Cookie"), auth.SessionCookieName) {
		t.Fatal("a session was issued before the second factor was proved")
	}
	form := totpForm(t, step, "/login")
	if !strings.Contains(step, "<svg class=\"qr\"") {
		t.Errorf("the enrolment step draws no code to scan:\n%s", step)
	}
	if hidden(t, form, "next") != "/tasks" {
		t.Errorf("next = %q, want the destination to survive the first step", hidden(t, form, "next"))
	}

	secret := secretIn(t, hidden(t, form, "uri"))
	done := b.post("/login", url.Values{
		"ticket": {hidden(t, form, "ticket")}, "next": {hidden(t, form, "next")},
		"uri": {hidden(t, form, "uri")}, "code": {codeFor(t, f, secret)}})
	codes := body(t, done)
	if done.StatusCode != http.StatusOK {
		t.Fatalf("the code step answered %d, want the recovery codes: %s", done.StatusCode, codes)
	}
	if !strings.Contains(done.Header.Get("Set-Cookie"), auth.SessionCookieName) {
		t.Error("the completed sign-in issued no session cookie")
	}
	if got := len(recoveryCodesOn(t, codes)); got != 10 {
		t.Errorf("the screen shows %d recovery codes, want ten:\n%s", got, codes)
	}
	if !strings.Contains(codes, `href="/tasks"`) {
		t.Errorf("the reader is not sent on to where they were going:\n%s", codes)
	}
}

// recoveryCodesOn reads the codes out of the one block that holds them,
// rather than out of the page, so a page that lost the block cannot pass by
// carrying the word "code" somewhere else.
func recoveryCodesOn(t *testing.T, page string) []string {
	t.Helper()
	block := between(t, page, `<pre id="recovery-codes"><code>`, "</code>")
	if block == "" {
		return nil
	}
	var out []string
	for _, line := range strings.Split(block, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// enrol takes a browser through enrolment from the settings screen and
// returns the shared secret and the recovery codes it was issued.
func enrol(t *testing.T, f *fixture, b *browser) (string, []string) {
	t.Helper()
	started := body(t, b.post("/settings/totp", url.Values{}))
	uri := hidden(t, totpForm(t, started, "/settings/totp/confirm"), "uri")
	secret := secretIn(t, uri)
	confirmed := body(t, b.post("/settings/totp/confirm", url.Values{
		"uri": {uri}, "code": {codeFor(t, f, secret)}}))
	codes := recoveryCodesOn(t, confirmed)
	if len(codes) != 10 {
		t.Fatalf("enrolment issued %d codes, want ten:\n%s", len(codes), confirmed)
	}
	return secret, codes
}

// The second step of a sign-in takes a code from the authenticator or a
// recovery code in the same field, and refuses anything else without giving
// up the ticket the reader is holding.
func TestSignInSecondStepTakesACodeOrARecoveryCode(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	user := f.person("enrolled@example.test", "correct-horse-battery")
	secret, codes := enrol(t, f, f.asActor(user.ID))

	tests := []struct {
		name     string
		code     func() string
		want     int
		location string
	}{
		{name: "the code from the app", code: func() string { return codeFor(t, f, secret) },
			want: http.StatusSeeOther, location: "/tasks"},
		{name: "a recovery code", code: func() string { return codes[0] },
			want: http.StatusSeeOther, location: "/tasks"},
		{name: "neither", code: func() string { return "000000" },
			want: http.StatusUnauthorized},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := f.as("")
			first := b.post("/login", url.Values{
				"email": {"enrolled@example.test"}, "password": {"correct-horse-battery"},
				"next": {"/tasks"}})
			step := body(t, first)
			if first.StatusCode != http.StatusOK {
				t.Fatalf("the password step answered %d, want the code step", first.StatusCode)
			}
			form := totpForm(t, step, "/login")
			if strings.Contains(step, "<svg class=\"qr\"") {
				t.Error("an enrolled account was offered an enrolment code")
			}

			second := b.post("/login", url.Values{
				"ticket": {hidden(t, form, "ticket")}, "next": {hidden(t, form, "next")},
				"code": {tc.code()}})
			page := body(t, second)
			if second.StatusCode != tc.want {
				t.Fatalf("status = %d, want %d: %s", second.StatusCode, tc.want, page)
			}
			if tc.location == "" {
				if !strings.Contains(page, "incorrect code") {
					t.Errorf("the refusal does not say the code was wrong:\n%s", page)
				}
				if hidden(t, totpForm(t, page, "/login"), "ticket") == "" {
					t.Error("the refused step dropped the ticket, so the password has to be typed again")
				}
				return
			}
			if got := second.Header.Get("Location"); got != tc.location {
				t.Errorf("location = %q, want %q", got, tc.location)
			}
			if !strings.Contains(second.Header.Get("Set-Cookie"), auth.SessionCookieName) {
				t.Error("the accepted code issued no session")
			}
		})
	}
}

// Rate limiting and a wrong code are different answers. A reader told their
// code is wrong tries again at once, which is the thing the lock exists to
// stop, so the locked step has to say to wait instead.
func TestALockedVerifierSaysToWait(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	user := f.person("locked@example.test", "correct-horse-battery")
	enrol(t, f, f.asActor(user.ID))

	var page string
	for range auth.TOTPMaxFailures + 1 {
		b := f.as("")
		step := body(t, b.post("/login", url.Values{
			"email": {"locked@example.test"}, "password": {"correct-horse-battery"}}))
		form := totpForm(t, step, "/login")
		page = body(t, b.post("/login", url.Values{
			"ticket": {hidden(t, form, "ticket")}, "code": {"000000"}}))
	}
	if !strings.Contains(page, "verification is locked until") {
		t.Errorf("the locked step does not say it is locked:\n%s", page)
	}
	if !strings.Contains(page, "Wait, then try again") {
		t.Errorf("the locked step does not tell the reader to wait:\n%s", page)
	}
}

// A first step that is never finished is a first step: no session, and no
// tenant record on the screen that asks for the code.
func TestAnAbandonedSecondStepGrantsNothing(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	user := f.person("halfway@example.test", "correct-horse-battery")
	enrol(t, f, f.asActor(user.ID))

	b := f.as("")
	resp := b.post("/login", url.Values{
		"email": {"halfway@example.test"}, "password": {"correct-horse-battery"},
		"next": {"/tasks"}})
	step := body(t, resp)
	if strings.Contains(resp.Header.Get("Set-Cookie"), auth.SessionCookieName) {
		t.Fatal("the password step issued a session on its own")
	}
	if b.cookie(auth.SessionCookieName) != "" {
		t.Fatal("the browser is holding a session after one step")
	}
	if strings.Contains(step, f.project.Key) || strings.Contains(step, "<nav") {
		t.Errorf("the code step drew tenant data:\n%s", step)
	}
}

// The deep link a reader was bounced from survives the sign-in screen itself,
// not only the two steps after it.
func TestTheSignInScreenCarriesTheDestination(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("")

	form := totpForm(t, b.page("/login?next=%2Ftasks%3Ffilter%3Dmine"), "/login")
	if got := hidden(t, form, "next"); got != "/tasks?filter=mine" {
		t.Errorf("next = %q, want the page the visitor was bounced from", got)
	}

	off := totpForm(t, b.page("/login?next=https%3A%2F%2Fevil.example%2Fsteal"), "/login")
	if got := hidden(t, off, "next"); got != "/projects" {
		t.Errorf("next = %q, want the off-origin destination discarded", got)
	}
}

// Nothing on either step depends on a script, and the code to scan is drawn
// on the server rather than by a client-side library that would need the
// secret handed to it.
func TestTheSecondStepNeedsNoScripting(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.requireTOTPWeb()
	f.person("noscript@example.test", "correct-horse-battery")

	b := f.as("")
	step := body(t, b.post("/login", url.Values{
		"email": {"noscript@example.test"}, "password": {"correct-horse-battery"}}))
	form := totpForm(t, step, "/login")

	if strings.Contains(form, "hx-post") || strings.Contains(form, "hx-get") ||
		strings.Contains(form, "onsubmit=") || strings.Contains(form, "oninput=") {
		t.Errorf("the code form depends on scripting:\n%s", form)
	}
	if !strings.Contains(form, `method="post"`) || !strings.Contains(form, `name="code"`) {
		t.Errorf("the code form is not a plain form:\n%s", form)
	}
	qr := between(t, step, `<svg class="qr"`, "</svg>")
	if qr == "" {
		t.Fatalf("the enrolment step draws no code to scan:\n%s", step)
	}
	if strings.Contains(qr, "<script") || strings.Contains(qr, "data-") {
		t.Errorf("the code to scan is assembled in the browser:\n%s", qr)
	}
}

// Enrolment from the settings screen: the URI, something to scan, the codes
// once, and the state afterwards.
func TestSettingsEnrolsAndShowsTheCodesOnce(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	user := f.person("settings@example.test", "correct-horse-battery")
	b := f.asActor(user.ID)

	before := b.page("/settings")
	if !strings.Contains(before, "This account has <strong>no</strong> second factor") {
		t.Errorf("the settings screen does not report an unenrolled account:\n%s", before)
	}

	started := body(t, b.post("/settings/totp", url.Values{}))
	form := totpForm(t, started, "/settings/totp/confirm")
	uri := hidden(t, form, "uri")
	if !strings.HasPrefix(uri, "otpauth://totp/") {
		t.Errorf("the enrolment screen carries no otpauth uri, got %q", uri)
	}
	if !strings.Contains(started, `<svg class="qr"`) {
		t.Errorf("the enrolment screen draws no code to scan:\n%s", started)
	}

	secret := secretIn(t, uri)
	confirmed := body(t, b.post("/settings/totp/confirm", url.Values{
		"uri": {uri}, "code": {codeFor(t, f, secret)}}))
	codes := recoveryCodesOn(t, confirmed)
	if len(codes) != 10 {
		t.Fatalf("confirmation showed %d codes, want ten:\n%s", len(codes), confirmed)
	}
	if !strings.Contains(confirmed, `data-copy-target="recovery-codes"`) {
		t.Errorf("the codes come with no way to copy them:\n%s", confirmed)
	}

	after := b.page("/settings")
	if !strings.Contains(after, "A second factor is <strong>on</strong>") {
		t.Errorf("the settings screen does not report the enrolment:\n%s", after)
	}
	if !strings.Contains(after, "<strong>10</strong> of ten recovery codes") {
		t.Errorf("the settings screen does not count the recovery codes:\n%s", after)
	}
	for _, code := range codes {
		if strings.Contains(after, code) {
			t.Fatalf("a recovery code is shown again on a later visit: %s", code)
		}
	}
	if strings.Contains(after, secret) {
		t.Fatal("the settings screen renders the shared secret")
	}
}

// A refused confirmation keeps the enrolment on screen, because the service
// hands the secret over once and a reader who mistyped would otherwise have
// to scan a new one.
func TestARefusedConfirmationKeepsTheEnrolmentOnScreen(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	user := f.person("mistyped@example.test", "correct-horse-battery")
	b := f.asActor(user.ID)

	started := body(t, b.post("/settings/totp", url.Values{}))
	uri := hidden(t, totpForm(t, started, "/settings/totp/confirm"), "uri")

	resp := b.post("/settings/totp/confirm", url.Values{"uri": {uri}, "code": {"000000"}})
	page := body(t, resp)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want the refusal", resp.StatusCode)
	}
	if hidden(t, totpForm(t, page, "/settings/totp/confirm"), "uri") != uri {
		t.Error("the refused screen dropped the enrolment it was confirming")
	}
	if !strings.Contains(page, `<svg class="qr"`) {
		t.Errorf("the refused screen dropped the code to scan:\n%s", page)
	}
}

// Replacing the codes issues a new set and voids the old one, and removal
// returns the account to a password alone.
func TestRecoveryCodesAreReplacedAndTheFactorRemoved(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	user := f.person("rotating@example.test", "correct-horse-battery")
	b := f.asActor(user.ID)
	_, first := enrol(t, f, b)

	replaced := recoveryCodesOn(t, body(t, b.post("/settings/totp/recovery-codes", url.Values{})))
	if len(replaced) != 10 {
		t.Fatalf("got %d replacement codes, want ten", len(replaced))
	}
	for _, code := range replaced {
		if strings.Contains(strings.Join(first, "\n"), code) {
			t.Fatalf("a replacement code repeats one of the first set: %s", code)
		}
	}

	resp := b.post("/settings/totp/remove", url.Values{})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	page := b.page("/settings")
	if !strings.Contains(page, "This account has <strong>no</strong> second factor") {
		t.Errorf("the factor was not removed:\n%s", page)
	}
}

// A tenant that requires a factor in the browser is not offered the control
// that takes it away, and is told where it can be cleared instead.
func TestRemovalIsNotOfferedWhereAFactorIsRequired(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	user := f.person("required@example.test", "correct-horse-battery")
	b := f.asActor(user.ID)
	enrol(t, f, b)
	f.requireTOTPWeb()

	page := b.page("/settings")
	if hasPlainForm(page, "/settings/totp/remove") {
		t.Errorf("the screen offers removal although the tenant requires a factor:\n%s", page)
	}
	if !strings.Contains(page, "This tenant requires a second factor in the browser, so it cannot be turned off here") {
		t.Errorf("the screen does not explain why removal is absent:\n%s", page)
	}
}

// An administrator sees the control that clears somebody else's factor, a
// reader without the scope does not, and neither sees a secret.
func TestTheUserListOffersTheClearToAnAdministratorOnly(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	user := f.person("cleared@example.test", "correct-horse-battery")
	secret, codes := enrol(t, f, f.asActor(user.ID))

	page := f.as("alice").page("/admin/users")
	form := totpForm(t, page, "/admin/users/totp/clear")
	if hidden(t, form, "id") == "" {
		t.Errorf("the clear control names no account:\n%s", form)
	}
	if strings.Contains(page, secret) {
		t.Fatal("the user list renders a shared secret")
	}
	for _, code := range codes {
		if strings.Contains(page, code) {
			t.Fatalf("the user list renders a recovery code: %s", code)
		}
	}

	member := f.as("useradmin").page("/admin/users")
	if hasPlainForm(member, "/admin/users/totp/clear") {
		t.Errorf("a reader without totp:admin is offered the clear:\n%s", member)
	}
}

// Clearing returns the account to an unenrolled state, which is the way back
// in for somebody who lost both the device and the codes.
func TestClearingAUsersFactorUnenrolsThem(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	user := f.person("lostphone@example.test", "correct-horse-battery")
	owner := f.asActor(user.ID)
	enrol(t, f, owner)

	resp := f.as("alice").post("/admin/users/totp/clear", url.Values{"id": {user.ID}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)

	page := owner.page("/settings")
	if !strings.Contains(page, "This account has <strong>no</strong> second factor") {
		t.Errorf("the account still holds an enrolment:\n%s", page)
	}
}
