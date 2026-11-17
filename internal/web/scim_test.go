// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// scimSection is one headed section of the SCIM screen and nothing else.
func scimSection(t *testing.T, page, id string) string {
	t.Helper()
	return between(t, page, `<section id="`+id+`"`, "</section>")
}

func TestTheSCIMScreenTurnsProvisioningOnShowsTheTokenOnceAndMapsGroups(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	page := b.page("/admin/scim")
	status := scimSection(t, page, "scim-status")
	if !strings.Contains(status, `action="/admin/scim/enable"`) || !strings.Contains(status, "/scim/v2") {
		t.Fatalf("an off tenant's status offers no way to turn it on or no endpoint:\n%s", status)
	}

	resp := b.postFrom("/admin/scim", "/admin/scim/enable", url.Values{})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	page = b.page("/admin/scim")
	if !strings.Contains(page, `class="secret"`) || !strings.Contains(page, "scim:provision") {
		t.Errorf("the minted token is not shown after enabling")
	}
	if !strings.Contains(scimSection(t, page, "scim-status"), `action="/admin/scim/disable"`) {
		t.Error("an enabled tenant is not offered a way to turn it off")
	}
	if rows := scimSection(t, page, "scim-token"); !strings.Contains(rows, "<td>scim</td>") {
		t.Errorf("the token section does not list the scim token:\n%s", rows)
	}
	if again := b.page("/admin/scim"); strings.Contains(again, `class="secret"`) {
		t.Error("the token is shown a second time")
	}

	resp = b.postFrom("/admin/scim", "/admin/scim/groups", url.Values{"group": {"Engineering"}, "role": {"admin"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	groups := scimSection(t, b.page("/admin/scim"), "scim-groups")
	if !strings.Contains(groups, "<td>Engineering</td><td>admin</td>") {
		t.Errorf("the mapping is not listed:\n%s", groups)
	}

	resp = b.postFrom("/admin/scim", "/admin/scim/groups", url.Values{"group": {""}, "role": {"admin"}})
	refused := body(t, resp)
	if resp.StatusCode == http.StatusSeeOther || !strings.Contains(refused, `id="scim-map"`) {
		t.Errorf("an empty group name was not refused into the dialog (status %d)", resp.StatusCode)
	}

	resp = b.postFrom("/admin/scim", "/admin/scim/groups/remove", url.Values{"group": {"Engineering"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	if groups := scimSection(t, b.page("/admin/scim"), "scim-groups"); strings.Contains(groups, "<td>Engineering</td>") {
		t.Error("the removed mapping is still listed")
	}

	resp = b.postFrom("/admin/scim", "/admin/scim/token", url.Values{})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	if !strings.Contains(b.page("/admin/scim"), `class="secret"`) {
		t.Error("the rotated token is not shown")
	}
	resp = b.postFrom("/admin/scim", "/admin/scim/disable", url.Values{})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	if status := scimSection(t, b.page("/admin/scim"), "scim-status"); !strings.Contains(status, `action="/admin/scim/enable"`) {
		t.Error("turning it off did not offer turning it on again")
	}
}
