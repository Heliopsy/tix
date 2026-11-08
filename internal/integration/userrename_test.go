// SPDX-License-Identifier: AGPL-3.0-or-later

package integration

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/web"
)

// A tenant admin renames a person who also belongs to another tenant, through
// the browser behind the real server, and the new name is what the Users
// screen, GetUser and ListUsers show for them afterwards. The name used to be
// saved on the person's actor here while every user read showed the
// account's, so the screen kept the old name as if the save had been lost.
func TestARenamedTwoTenantPersonReadsBackUnderTheNewName(t *testing.T) {
	h := newHarness(t)
	const password = "correct-horse-battery-staple"
	if _, err := h.server.CreateUser(h.adminCtx, core.CreateUserInput{
		Email: "renamer@example.test", Handle: "renamer", Password: password, Role: core.RoleAdmin,
	}); err != nil {
		t.Fatalf("creating the tenant admin: %v", err)
	}
	pat, err := h.server.CreateUser(h.adminCtx, core.CreateUserInput{
		Email: "pat@example.test", Handle: "pat", DisplayName: "Pat", Role: core.RoleMember,
	})
	if err != nil {
		t.Fatalf("creating pat: %v", err)
	}
	if _, err := h.server.CreateTenant(core.WithDirectStore(h.adminCtx), core.CreateTenantInput{
		Key: "other", Name: "Other", Admins: []core.TenantAdminInput{{Email: "pat@example.test"}},
	}); err != nil {
		t.Fatalf("adding pat to a second tenant: %v", err)
	}
	client := signIn(t, h.baseURL, "renamer@example.test", password)

	page := readAll(t, getOK(t, client, h.baseURL+web.RouteUsers))
	form := url.Values{
		"id":               {pat.ID},
		"display_name":     {"Pat Renamed"},
		"role":             {string(core.RoleMember)},
		web.CSRFFieldName: {betweenQuotes(page, `name="`+web.CSRFFieldName+`" value="`)},
	}
	res, err := client.PostForm(h.baseURL+web.RouteUserUpdate, form)
	if err != nil {
		t.Fatalf("posting the rename: %v", err)
	}
	if body := readAll(t, res); res.StatusCode != http.StatusSeeOther {
		t.Fatalf("renaming pat = %d, want 303: %s", res.StatusCode, body)
	}

	got, err := h.server.GetUser(h.adminCtx, pat.ID)
	if err != nil || got.DisplayName != "Pat Renamed" {
		t.Errorf("GetUser after the rename = %+v, %v; want \"Pat Renamed\"", got, err)
	}
	listed, _, err := h.server.ListUsers(h.adminCtx, core.Page{})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	for _, u := range listed {
		if u.ID == pat.ID && u.DisplayName != "Pat Renamed" {
			t.Errorf("ListUsers after the rename shows pat as %q, want \"Pat Renamed\"", u.DisplayName)
		}
	}

	after := readAll(t, getOK(t, client, h.baseURL+web.RouteUsers))
	input := between(after, `<input id="name-`+pat.ID+`"`, ">")
	if !strings.Contains(input, `value="Pat Renamed"`) {
		t.Errorf("the Users screen's name field for pat is %q, want the new name", input)
	}
	row := between(after, `id="user-`+pat.ID+`"`, "</li>")
	if !strings.Contains(row, "Pat Renamed") {
		t.Errorf("pat's row on the Users screen does not show the new name:\n%s", row)
	}
}

func getOK(t *testing.T, client *http.Client, address string) *http.Response {
	t.Helper()
	res, err := client.Get(address)
	if err != nil {
		t.Fatalf("GET %s: %v", address, err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d: %s", address, res.StatusCode, readAll(t, res))
	}
	return res
}

// between is the text after the first start and before the next end, empty
// when start is absent.
func between(s, start, end string) string {
	_, rest, ok := strings.Cut(s, start)
	if !ok {
		return ""
	}
	out, _, _ := strings.Cut(rest, end)
	return out
}
