// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/output"
)

// issueToken submits the issue-a-token form the way a browser without
// scripting does, and returns the response so a caller can read its status.
func issueToken(t *testing.T, b *browser, form url.Values) *http.Response {
	t.Helper()
	form.Set("csrf_token", b.csrf())
	return b.postRaw("/admin/tokens", form)
}

// secretBlock returns the one-time secret region of the token screen, and
// nothing else, so an assertion about how the value is presented reads the
// element that presents it rather than the page it sits on.
func secretBlock(t *testing.T, page string) string {
	t.Helper()
	return between(t, page, `<section class="secret"`, "</section>")
}

// tokenRows returns the body of the token table, so an assertion about a
// column reads the table rather than the form underneath it, which also
// carries the words "name", "scopes" and "expires".
func tokenRows(t *testing.T, page string) string {
	t.Helper()
	body := between(t, page, "<tbody>", "</tbody>")
	if body == "" {
		t.Fatalf("the token screen renders no table body")
	}
	return body
}

// tokenCell returns one cell of the first token row, counting from zero.
func tokenCell(t *testing.T, page string, index int) string {
	t.Helper()
	row := tokenRows(t, page)
	cells := strings.Split(row, "<td>")
	if len(cells) <= index+1 {
		t.Fatalf("the first token row has no cell %d:\n%s", index, row)
	}
	cell, _, _ := strings.Cut(cells[index+1], "</td>")
	return strings.TrimSpace(cell)
}

// issueForm returns the issue-a-token form alone.
func issueForm(t *testing.T, page string) string {
	t.Helper()
	return formAt(t, page, "/admin/tokens")
}

// style renders an instant exactly as an unconfigured handler does.
var style output.TimeStyle

func TestIssuedTokenIsPresentedAsASecretNotAFlash(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := issueToken(t, b, url.Values{"name": {"agent"}, "scopes": {"task:read"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)

	page := b.page("/admin/tokens")
	secret := secretBlock(t, page)
	if secret == "" {
		t.Fatalf("the issued token is not in a secret region:\n%s", page)
	}
	value := between(t, secret, `<code id="issued-token">`, "</code>")
	if !strings.HasPrefix(value, "tix_") {
		t.Fatalf("the secret region does not carry the token value, it carries %q", value)
	}
	if !strings.Contains(secret, `data-copy-target="issued-token"`) {
		t.Errorf("the secret has no copy button pointing at it:\n%s", secret)
	}
	if !strings.Contains(secret, `class="copyline"`) {
		t.Errorf("the value is not in a copyline, so nothing wraps it:\n%s", secret)
	}
	// The complaint this fixes: the one value nobody can fetch again was
	// styled as the transient confirmation that a task saved.
	if strings.Contains(secret, "flash") {
		t.Errorf("the secret is still presented as a flash:\n%s", secret)
	}
	if strings.Contains(page, "<pre>"+value) {
		t.Errorf("the secret is still in an unwrapped pre block")
	}
}

func TestEmptyScopeSelectionRedrawsTheFormAndKeepsTheName(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := issueToken(t, b, url.Values{"name": {"half-typed"}})
	page := body(t, resp)
	if resp.StatusCode == http.StatusSeeOther {
		t.Fatalf("a submission with no scope was accepted")
	}
	// Not the error screen: that is what used to happen, and it threw away
	// everything the reader had entered.
	if strings.Contains(page, "Back to where you were") {
		t.Fatalf("an empty scope selection still answers with the error screen:\n%s", page)
	}
	form := issueForm(t, page)
	if got := inputValue(t, form+">", "name"); got != "half-typed" {
		t.Errorf("the name the reader entered was not preserved, the field holds %q", got)
	}
	if !strings.Contains(form, "at least one scope") {
		t.Errorf("the form does not say why it was refused:\n%s", form)
	}
	// On the line under the control it is about, not loose at the top of the
	// page: the form carries three controls and a toast cannot say which.
	_, tail, ok := strings.Cut(form, `id="scopes"`)
	if !ok {
		t.Fatalf("the re-rendered form has no scopes control:\n%s", form)
	}
	_, tail, _ = strings.Cut(tail, "</select>")
	if !strings.HasPrefix(strings.TrimSpace(tail), `<p class="field-error" id="scopes-error"`) {
		t.Errorf("the refusal does not sit under the scopes control, it is followed by %.120q", tail)
	}
	if !strings.Contains(form, `aria-describedby="scopes-error"`) {
		t.Errorf("the scopes control does not point at its own message:\n%s", form)
	}
	if !strings.Contains(form, `id="scopes" name="scopes" multiple size="8" required`) {
		t.Errorf("the scopes control is not marked required:\n%s", form)
	}
}

// TestARefusedFormKeepsTheExpiryChoice covers the third value on this form,
// and the one whose loss a reader cannot see: a select with nothing selected
// shows its first option, so a form that came back without the choice marked
// said "7 days" while the handler meant ninety. The proposed default itself is
// pinned by TestTheFormProposesAnExpiry.
func TestARefusedFormKeepsTheExpiryChoice(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	for _, tc := range []struct{ sent, want string }{
		{"365d", "365d"},
		{"never", "never"},
		{"", "90d"},
	} {
		t.Run("sent "+tc.sent, func(t *testing.T) {
			form := url.Values{"name": {"half-typed"}}
			if tc.sent != "" {
				form.Set("expires", tc.sent)
			}
			resp := issueToken(t, b, form)
			page := body(t, resp)
			if resp.StatusCode == http.StatusSeeOther {
				t.Fatalf("a submission with no scope was accepted")
			}
			control := between(t, issueForm(t, page), `<select id="expires"`, "</select>")
			if !strings.Contains(control, `<option value="`+tc.want+`" selected>`) {
				t.Errorf("the expiry control came back without %q selected:\n%s", tc.want, control)
			}
		})
	}
}

func TestTwoTokensCannotShareAName(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	first := issueToken(t, b, url.Values{"name": {"ci"}, "scopes": {"task:read"}})
	_ = first.Body.Close()
	wantStatus(t, first, http.StatusSeeOther)

	second := issueToken(t, b, url.Values{"name": {"ci"}, "scopes": {"task:write"}})
	page := body(t, second)
	if second.StatusCode == http.StatusSeeOther {
		t.Fatalf("a second token named ci was accepted")
	}
	form := issueForm(t, page)
	if got := inputValue(t, form+">", "name"); got != "ci" {
		t.Errorf("the refused name was not preserved, the field holds %q", got)
	}
	if !strings.Contains(form, "already exists") {
		t.Errorf("the form does not say the name is taken:\n%s", form)
	}
	if n := strings.Count(tokenRows(t, b.page("/admin/tokens")), ">ci<"); n != 1 {
		t.Errorf("the tenant holds %d tokens named ci, want 1", n)
	}
}

// TestARevokedNameIsFreeAgain is the other half of the rule: rotation is
// revoke-then-reissue, so holding a revoked token's name forever would make
// the common case impossible.
func TestARevokedNameIsFreeAgain(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := issueToken(t, b, url.Values{"name": {"rotating"}, "scopes": {"task:read"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)

	page := b.page("/admin/tokens")
	revoked := b.post("/admin/tokens/revoke", url.Values{"id": {tokenID(t, page)}})
	_ = revoked.Body.Close()
	wantStatus(t, revoked, http.StatusSeeOther)

	again := issueToken(t, b, url.Values{"name": {"rotating"}, "scopes": {"task:read"}})
	defer func() { _ = again.Body.Close() }()
	wantStatus(t, again, http.StatusSeeOther)
}

func TestTokenListingShowsWhenATokenWasCreated(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := issueToken(t, b, url.Values{"name": {"agent"}, "scopes": {"task:read"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)

	page := b.page("/admin/tokens")
	if !strings.Contains(between(t, page, "<thead>", "</thead>"), "<th>Created</th>") {
		t.Fatalf("the token table has no Created column:\n%s", page)
	}
	// The second cell: Name, Scopes, Created, with every column shown.
	want := style.Format(f.clock.Now())
	if got := tokenCell(t, page, 2); got != want {
		t.Errorf("the created cell reads %q, want the token's own creation time %q", got, want)
	}
}

func TestExpiryChosenOnTheFormReachesTheToken(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := issueToken(t, b, url.Values{
		"name": {"short-lived"}, "scopes": {"task:read"}, "expires": {"7d"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)

	page := b.page("/admin/tokens")
	cell := tokenCell(t, page, 3)
	if cell == "" || strings.Contains(cell, "Never") {
		t.Fatalf("the expires cell reads %q, want a stored expiry", cell)
	}
	// The expiry is counted from the wall clock, since it is the reader's
	// "seven days from now" rather than anything the service stamps, so the
	// day is what this pins.
	wantDay := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	if !strings.Contains(cell, wantDay) {
		t.Errorf("the expires cell reads %q, want an expiry on %s", cell, wantDay)
	}
}

func TestATokenWithNoExpirySaysSo(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := issueToken(t, b, url.Values{
		"name": {"forever"}, "scopes": {"task:read"}, "expires": {"never"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)

	if got := tokenCell(t, b.page("/admin/tokens"), 3); !strings.Contains(got, "Never") {
		t.Errorf("a token with no expiry renders its expires cell as %q, want it to say so", got)
	}
}

// TestTheFormProposesAnExpiry pins the default, because "no expiry unless you
// say otherwise" is the behaviour this change deliberately reversed.
func TestTheFormProposesAnExpiry(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	form := issueForm(t, b.page("/admin/tokens"))
	selected := between(t, form, `<select id="expires"`, "</select>")
	if !strings.Contains(selected, `<option value="90d" selected>`) {
		t.Errorf("the expiry control proposes no expiry by default:\n%s", selected)
	}
	if !strings.Contains(selected, `<option value="never" `) {
		t.Errorf("the expiry control offers no way to opt out:\n%s", selected)
	}
}

func TestAnExpiryTheFormDoesNotOfferIsRefused(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := issueToken(t, b, url.Values{
		"name": {"forged"}, "scopes": {"task:read"}, "expires": {"9999d"}})
	page := body(t, resp)
	if resp.StatusCode == http.StatusSeeOther {
		t.Fatalf("an expiry outside the offered choices was accepted")
	}
	if !strings.Contains(issueForm(t, page), "expiry choices") {
		t.Errorf("the form does not say the expiry was not one of its choices:\n%s", page)
	}
}

func TestRevokedTokenSaysSoAndOffersNoRevocation(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := issueToken(t, b, url.Values{"name": {"doomed"}, "scopes": {"task:read"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)

	live := b.page("/admin/tokens")
	if !strings.Contains(tokenRows(t, live), "tok-active") {
		t.Fatalf("a live token is not marked active:\n%s", tokenRows(t, live))
	}
	revoked := b.post("/admin/tokens/revoke", url.Values{"id": {tokenID(t, live)}})
	_ = revoked.Body.Close()
	wantStatus(t, revoked, http.StatusSeeOther)

	rows := tokenRows(t, b.page("/admin/tokens"))
	if !strings.Contains(rows, "tok-revoked") {
		t.Errorf("the revoked token is not marked revoked:\n%s", rows)
	}
	if strings.Contains(rows, "/admin/tokens/revoke") {
		t.Errorf("a revoked token still offers a revoke control:\n%s", rows)
	}
}

// TestRevocationIsBehindAConfirmation covers the destructive control itself:
// ending a credential should not be one stray click away, and the words
// saying what it ends have to be somewhere.
func TestRevocationIsBehindAConfirmation(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := issueToken(t, b, url.Values{"name": {"guarded"}, "scopes": {"task:read"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)

	rows := tokenRows(t, b.page("/admin/tokens"))
	if !strings.Contains(rows, `<details class="panel danger">`) {
		t.Errorf("revocation is not behind a disclosure:\n%s", rows)
	}
	confirm := formAt(t, rows, "/admin/tokens/revoke")
	if !strings.Contains(confirm, "stops authenticating") {
		t.Errorf("the revoke control does not say what it does:\n%s", confirm)
	}
	if !strings.Contains(confirm, "Revoke guarded") {
		t.Errorf("the revoke button does not name the token it ends:\n%s", confirm)
	}
}
