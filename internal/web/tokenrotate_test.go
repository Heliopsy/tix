// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// tokenRowOf returns the table row naming one token identifier in its controls.
func tokenRowOf(t *testing.T, page, tokenID string) string {
	t.Helper()
	for _, row := range strings.Split(tokenRows(t, page), "</tr>") {
		if strings.Contains(row, "token-revoke-"+tokenID) || strings.Contains(row, "token-rotate-"+tokenID) {
			return row
		}
	}
	return ""
}

// rotateControlFor returns the rotation form the listing offers for one token,
// the row's button and the form in the dialog it opens, or "" when there is
// none.
func rotateControlFor(page, tokenID string) string {
	body, _, _ := strings.Cut(page[strings.Index(page, "<tbody>")+1:], "</tbody>")
	if !strings.Contains(body, `data-dlg-open="token-rotate-`+tokenID+`"`) {
		return ""
	}
	_, form, found := strings.Cut(dialogBlock(page, "token-rotate-"+tokenID), `action="/admin/tokens/rotate"`)
	if !found {
		return ""
	}
	form, _, _ = strings.Cut(form, "</form>")
	if !strings.Contains(form, `value="`+tokenID+`"`) {
		return ""
	}
	return form
}

func TestATokenIsRotatedFromTheScreenAndItsValueShownOnce(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := issueToken(t, b, url.Values{"name": {"rotor"}, "scopes": {"task:read"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	page := b.page("/admin/tokens")
	id := tokenID(t, page)

	form := rotateControlFor(page, id)
	if form == "" {
		t.Fatalf("a live token offers no rotate control:\n%s", tokenRows(t, page))
	}
	for _, want := range []string{"same name, scopes and lifetime", `name="grace"`, `value="0"`, `value="24h"`, "Rotate rotor"} {
		if !strings.Contains(form, want) {
			t.Errorf("the rotate dialog does not carry %q:\n%s", want, form)
		}
	}
	if strings.Contains(tokenRowOf(t, page, id), "/admin/tokens/rotate") {
		t.Error("a row rotates on one click rather than through its dialog")
	}

	// The screen states a token against the wall clock, so the window the
	// service stores has to be measured from the same instant.
	f.clock.Set(time.Now())
	rotated := b.post("/admin/tokens/rotate", url.Values{"id": {id}, "grace": {"1h"}})
	_ = rotated.Body.Close()
	wantStatus(t, rotated, http.StatusSeeOther)

	after := b.page("/admin/tokens")
	secret := secretBlock(t, after)
	if !strings.Contains(secret, "tix_") {
		t.Errorf("the rotated value is not shown:\n%s", secret)
	}
	old := tokenRowOf(t, after, id)
	if !strings.Contains(old, "tok-retiring") {
		t.Errorf("the old token inside its grace window is not marked retiring:\n%s", old)
	}
	if rotateControlFor(after, id) != "" {
		t.Error("a retiring token still offers a rotate control")
	}
	if revokeControlFor(after, id) == "" {
		t.Error("a retiring token offers no revoke control to end its window early")
	}
	if again := b.page("/admin/tokens"); strings.Contains(again, `<section class="secret"`) {
		t.Error("the rotated value is shown a second time")
	}
}

// From the listing filtered to one actor, which the /actors page links to,
// the rotation lands back on that actor's tokens.
func TestRotatingAnotherActorsTokenReturnsToTheirListing(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	carol, theirs := seedOtherActorsToken(t, f, "carol", "carols-agent")
	b := f.as("alice")

	page := b.page("/admin/tokens?actor=" + url.QueryEscape(carol.ID))
	form := rotateControlFor(page, theirs.ID)
	if form == "" {
		t.Fatalf("the actor's listing offers no rotate control:\n%s", tokenRows(t, page))
	}
	if !strings.Contains(form, `name="actor" value="`+carol.ID+`"`) {
		t.Errorf("the rotate form does not carry the listing it came from:\n%s", form)
	}
	if !strings.Contains(form, "not yours") || !strings.Contains(form, "Rotate carols-agent ("+carol.Handle+")") {
		t.Errorf("the rotate dialog does not say whose token it is:\n%s", form)
	}

	resp := b.post("/admin/tokens/rotate", url.Values{"id": {theirs.ID}, "grace": {"0"}, "actor": {carol.ID}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	if loc := resp.Header.Get("Location"); !strings.HasPrefix(loc, "/admin/tokens?actor="+url.QueryEscape(carol.ID)) {
		t.Errorf("the rotation went to %q, want the actor's listing", loc)
	}
	after := b.page("/admin/tokens?actor=" + url.QueryEscape(carol.ID))
	rows := tokenRows(t, after)
	if !strings.Contains(rows, "tok-revoked") || tokenRowOf(t, after, theirs.ID) != "" {
		t.Errorf("the old token is not revoked, with nothing left to do to it, after an immediate rotation:\n%s", rows)
	}
	if !strings.Contains(secretBlock(t, after), "tix_") {
		t.Error("the rotated value is not shown on the actor's listing")
	}
}

func TestARotationWithAnUnknownGraceIsRefused(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	resp := issueToken(t, b, url.Values{"name": {"strict"}, "scopes": {"task:read"}})
	_ = resp.Body.Close()
	id := tokenID(t, b.page("/admin/tokens"))

	refused := b.post("/admin/tokens/rotate", url.Values{"id": {id}, "grace": {"1000h"}})
	_ = refused.Body.Close()
	if refused.StatusCode != http.StatusBadRequest {
		t.Errorf("an unknown grace answered %d, want 400", refused.StatusCode)
	}
	if !strings.Contains(tokenRowOf(t, b.page("/admin/tokens"), id), "tok-active") {
		t.Error("a refused rotation changed the token")
	}
}
