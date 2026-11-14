// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// claimBox is the "require a claim" checkbox of the issue form, and nothing
// else, so an assertion about it cannot pass on another control's markup.
func claimBox(t *testing.T, form string) string {
	t.Helper()
	return between(t, form, `<input id="require_claim"`, ">")
}

func TestTheIssueFormOffersTheClaimRequirementUnticked(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	box := claimBox(t, issueForm(t, b.page("/admin/tokens")))
	if !strings.Contains(box, `name="require_claim"`) || strings.Contains(box, "checked") {
		t.Errorf("the claim checkbox reads %q, want it offered and unticked", box)
	}
}

func TestATickedClaimRequirementReachesTheTokenAndTheListing(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := issueToken(t, b, url.Values{"name": {"bot"}, "scopes": {"task:read"}, "require_claim": {"1"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	resp = issueToken(t, b, url.Values{"name": {"script"}, "scopes": {"task:read"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)

	rows := tokenRows(t, b.page("/admin/tokens"))
	for name, want := range map[string]bool{"bot": true, "script": false} {
		cell := between(t, rows, `<td class="token">`+name, "</td>")
		if strings.Contains(cell, "claim required") != want {
			t.Errorf("the %s row's name cell reads %q, want the claim mark %v", name, cell, want)
		}
	}
}

func TestARefusedFormKeepsTheClaimRequirementTicked(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := issueToken(t, b, url.Values{"name": {"half-typed"}, "require_claim": {"1"}})
	page := body(t, resp)
	if resp.StatusCode == http.StatusSeeOther {
		t.Fatalf("a submission with no scope was accepted")
	}
	if box := claimBox(t, issueForm(t, page)); !strings.Contains(box, "checked") {
		t.Errorf("the claim checkbox came back unticked: %q", box)
	}
}
