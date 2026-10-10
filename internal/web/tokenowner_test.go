// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// The browser used to list the signed-in actor's own tokens and nothing else,
// which made it useless in the one case that matters: somebody else's token has
// leaked and has to stop working now. The command line has taken another
// actor's identifier all along.

// adminCtx is a context speaking for the tenant's administrator, for seeding a
// token that belongs to somebody other than the reader.
func adminCtx(f *fixture) context.Context {
	ctx := core.WithTenant(context.Background(), core.TenantScope{TenantID: f.tenantA.ID})
	return core.WithActor(ctx, copyActor(f.actorA, []core.Scope{core.ScopeAll}, core.RoleAdmin))
}

// seedOtherActorsToken mints a token held by a second actor of this tenant,
// and returns that actor with the token.
func seedOtherActorsToken(t *testing.T, f *fixture, handle, name string) (core.Actor, core.APIToken) {
	t.Helper()
	other := seedActor(t, f.store, f.tenantA.ID, handle, core.RoleMember)
	issued, err := f.svc.CreateToken(adminCtx(f), core.CreateTokenInput{
		Name: name, ActorID: other.ID, Scopes: []core.Scope{core.ScopeTaskRead}})
	if err != nil {
		t.Fatalf("minting %s's token: %v", handle, err)
	}
	return other, issued.APIToken
}

// revokeControlFor returns the revocation form the listing offers for one
// token identifier, and an empty string when it offers none. The form, not the
// page: a page that merely mentions an identifier somewhere is not a page
// offering to end it.
func revokeControlFor(rows, tokenID string) string {
	for _, form := range strings.Split(rows, `action="/admin/tokens/revoke"`)[1:] {
		body, _, _ := strings.Cut(form, "</form>")
		if strings.Contains(body, `value="`+tokenID+`"`) {
			return body
		}
	}
	return ""
}

// TestAReaderWithoutTenantAdminSeesOnlyTheirOwnTokens is the unchanged half.
//
// Both halves are asserted, not only the first: the row is absent *and* no
// revoke control carries that token's identifier. A screen that lists a
// credential under a revoke button the reader may not use is the defect even
// when the service then refuses the submission, because the reader has already
// been told the row is theirs to end.
func TestAReaderWithoutTenantAdminSeesOnlyTheirOwnTokens(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	_, theirs := seedOtherActorsToken(t, f, "carol", "carols-agent")

	b := f.as("tokenkeeper")
	mine := issueToken(t, b, url.Values{"name": {"mine"}, "scopes": {"task:read"}})
	_ = mine.Body.Close()
	wantStatus(t, mine, http.StatusSeeOther)

	page := b.page("/admin/tokens")
	rows := tokenRows(t, page)
	if strings.Contains(rows, "carols-agent") {
		t.Errorf("a reader without tenant:admin is shown another actor's token:\n%s", rows)
	}
	if strings.Contains(rows, theirs.ID) {
		t.Errorf("another actor's token identifier reaches the listing:\n%s", rows)
	}
	if offered := revokeControlFor(rows, theirs.ID); offered != "" {
		t.Errorf("a revoke control is offered for a token this reader may not see:\n%s", offered)
	}
	if !strings.Contains(rows, "mine") {
		t.Errorf("the reader's own token is missing:\n%s", rows)
	}
	// No owner column either: for a reader who can only ever see their own, a
	// column reading "you" on every row is noise.
	if headers := tokenHeaders(t, page); slices.Contains(headers, "Owner") {
		t.Errorf("the table carries an owner column for a single-owner listing: %v", headers)
	}
}

// TestATenantAdminSeesOtherActorsTokensWithTheOwnerNamed is the new half.
func TestATenantAdminSeesOtherActorsTokensWithTheOwnerNamed(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	carol, theirs := seedOtherActorsToken(t, f, "carol", "carols-agent")

	b := f.as("alice")
	page := b.page("/admin/tokens")
	rows := tokenRows(t, page)
	if !strings.Contains(rows, "carols-agent") {
		t.Fatalf("a tenant administrator cannot see another actor's token:\n%s", rows)
	}
	// Whose, by name. A list of other people's credentials with no owner
	// column cannot be acted on: every row looks like your own.
	headers := tokenHeaders(t, page)
	if !slices.Contains(headers, "Owner") {
		t.Fatalf("the table has no owner column: %v", headers)
	}
	if got := tokenCellUnder(t, page, "Owner"); !strings.Contains(got, carol.Handle) {
		t.Errorf("the owner cell reads %q, want it to name %q", got, carol.Handle)
	}
	offered := revokeControlFor(rows, theirs.ID)
	if offered == "" {
		t.Fatalf("no revoke control is offered for another actor's token:\n%s", rows)
	}
	if !strings.Contains(offered, "not yours") {
		t.Errorf("the confirmation does not say the token is somebody else's:\n%s", offered)
	}
	if !strings.Contains(offered, "Revoke carols-agent ("+carol.Handle+")") {
		t.Errorf("the revoke button does not name whose token it ends:\n%s", offered)
	}
}

// TestATenantAdminCanRevokeAnotherActorsTokenInTheBrowser drives the control
// the screen offered and reads the row back.
func TestATenantAdminCanRevokeAnotherActorsTokenInTheBrowser(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	_, theirs := seedOtherActorsToken(t, f, "carol", "carols-agent")

	b := f.as("alice")
	resp := b.post("/admin/tokens/revoke", url.Values{"id": {theirs.ID}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)

	rows := tokenRows(t, b.page("/admin/tokens"))
	row, _, _ := strings.Cut(rows, "</tr>")
	if !strings.Contains(row, "carols-agent") {
		t.Fatalf("the first row is not the token just revoked:\n%s", row)
	}
	if !strings.Contains(row, "tok-revoked") {
		t.Errorf("another actor's token was not revoked:\n%s", row)
	}
	if revokeControlFor(rows, theirs.ID) != "" {
		t.Errorf("the revoked token still offers a revoke control:\n%s", rows)
	}
}
