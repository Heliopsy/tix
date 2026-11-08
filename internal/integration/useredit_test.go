// SPDX-License-Identifier: AGPL-3.0-or-later

package integration

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/web"
)

// twoTenantMember sets up, behind the real server, a tenant admin who is not
// a global admin and a person, pat, who belongs to the admin's tenant and to
// a second one. It returns the admin's signed-in browser and pat's actor in
// the admin's tenant.
func twoTenantMember(t *testing.T, h *harness) (*http.Client, string) {
	t.Helper()
	const password = "correct-horse-battery-staple"
	if _, err := h.server.CreateUser(h.adminCtx, core.CreateUserInput{
		Email: "keeper@example.test", Handle: "keeper", Password: password, Role: core.RoleAdmin,
	}); err != nil {
		t.Fatalf("creating the tenant admin: %v", err)
	}
	pat, err := h.server.CreateUser(h.adminCtx, core.CreateUserInput{
		Email: "pat@example.test", Handle: "pat", DisplayName: "Pat", Password: password, Role: core.RoleMember,
	})
	if err != nil {
		t.Fatalf("creating pat: %v", err)
	}
	if _, err := h.server.CreateTenant(core.WithDirectStore(h.adminCtx), core.CreateTenantInput{
		Key: "other", Name: "Other", Admins: []core.TenantAdminInput{{Email: "pat@example.test"}},
	}); err != nil {
		t.Fatalf("adding pat to a second tenant: %v", err)
	}
	return signIn(t, h.baseURL, "keeper@example.test", password), pat.ID
}

var (
	formInput   = regexp.MustCompile(`<input\b[^>]*>`)
	formSelect  = regexp.MustCompile(`(?s)<select\b[^>]*\bname="([^"]+)"[^>]*>(.*?)</select>`)
	optionValue = regexp.MustCompile(`<option value="([^"]*)"\s+selected>`)
	attrPattern = regexp.MustCompile(`\b(name|value|type)="([^"]*)"`)
)

// manageForm is the update form of one user's Manage dialog, as drawn.
func manageForm(t *testing.T, page, id string) string {
	t.Helper()
	return userForm(t, page, web.RouteUserUpdate, id)
}

// userForm is the form posting to action for one user.
func userForm(t *testing.T, page, action, id string) string {
	t.Helper()
	for _, block := range strings.Split(page, `action="`+action+`"`)[1:] {
		form, _, _ := strings.Cut(block, "</form>")
		if strings.Contains(form, `name="id" value="`+id+`"`) {
			return form
		}
	}
	t.Fatalf("no form posting to %s for user %q:\n%s", action, id, page)
	return ""
}

// submitted is what a browser sends for a form left as drawn: every text and
// hidden input, the ticked boxes, and each select's selected option.
func submitted(form string) url.Values {
	out := url.Values{}
	for _, tag := range formInput.FindAllString(form, -1) {
		attrs := map[string]string{}
		for _, m := range attrPattern.FindAllStringSubmatch(tag, -1) {
			attrs[m[1]] = m[2]
		}
		if attrs["name"] == "" || attrs["type"] == "checkbox" && !strings.Contains(tag, " checked") {
			continue
		}
		out.Set(attrs["name"], attrs["value"])
	}
	for _, m := range formSelect.FindAllStringSubmatch(form, -1) {
		if chosen := optionValue.FindStringSubmatch(m[2]); chosen != nil {
			out.Set(m[1], chosen[1])
		}
	}
	return out
}

// A tenant admin who is not a global admin may change a two-tenant person's
// name and role in this tenant. The browser form used to send the forced
// password change on every save, which the service refuses for such a person,
// so renaming pat or making her an admin failed on the error page.
func TestATenantAdminEditsATwoTenantMemberInTheBrowser(t *testing.T) {
	cases := []struct {
		name   string
		change func(url.Values)
		check  func(*testing.T, *core.Actor, core.Role)
	}{
		// The name a tenant admin gives a two-tenant person is theirs in this
		// tenant alone, so it lands on the person's actor here.
		{"display name", func(v url.Values) { v.Set("display_name", "Pat Renamed") },
			func(t *testing.T, a *core.Actor, _ core.Role) {
				if a.DisplayName != "Pat Renamed" {
					t.Errorf("pat is called %q here after the save, want \"Pat Renamed\"", a.DisplayName)
				}
			}},
		{"role", func(v url.Values) { v.Set("role", string(core.RoleAdmin)) },
			func(t *testing.T, _ *core.Actor, role core.Role) {
				if role != core.RoleAdmin {
					t.Errorf("pat's role after the save is %q, want admin", role)
				}
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			client, patID := twoTenantMember(t, h)

			page := readAll(t, mustGet(t, client, h.baseURL+web.RouteUsers))
			form := submitted(manageForm(t, page, patID))
			tc.change(form)
			res, err := client.PostForm(h.baseURL+web.RouteUserUpdate, form)
			if err != nil {
				t.Fatalf("posting the edit: %v", err)
			}
			body := readAll(t, res)
			if res.StatusCode != http.StatusSeeOther {
				t.Fatalf("saving pat's %s = %d, want 303: %s", tc.name, res.StatusCode, body)
			}

			ctx := h.adminCtx
			u, err := h.server.GetUser(ctx, patID)
			if err != nil {
				t.Fatalf("reading pat: %v", err)
			}
			a, err := h.server.GetActor(ctx, patID)
			if err != nil {
				t.Fatalf("reading pat's actor: %v", err)
			}
			tc.check(t, a, roleOf(t, ctx, h, patID))
			if u.RequirePasswordChange {
				t.Error("an edit that left the password box alone set the forced password change")
			}
		})
	}
}

// When a tenant admin does ask for a two-tenant person's password to be
// changed, the refusal comes back on the form that asked, in words, rather
// than as a bare error page.
func TestARefusedPasswordChangeForATwoTenantMemberIsShownOnTheForm(t *testing.T) {
	h := newHarness(t)
	client, patID := twoTenantMember(t, h)

	page := readAll(t, mustGet(t, client, h.baseURL+web.RouteUsers))
	form := submitted(manageForm(t, page, patID))
	form.Set("display_name", "Pat Renamed")
	form.Set("require_password_change", "1")
	res, err := client.PostForm(h.baseURL+web.RouteUserUpdate, form)
	if err != nil {
		t.Fatalf("posting the edit: %v", err)
	}
	body := readAll(t, res)
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("the refused save answered %d, want 403", res.StatusCode)
	}
	drawn := manageForm(t, body, patID)
	if !strings.Contains(drawn, "Pat also belongs to another tenant, so only they or a global admin can ask them to change their password") {
		t.Errorf("the manage form does not say why the save was refused:\n%s", drawn)
	}
	if !strings.Contains(drawn, `value="Pat Renamed"`) {
		t.Errorf("the refused form lost what was typed:\n%s", drawn)
	}
	u, err := h.server.GetUser(h.adminCtx, patID)
	if err != nil {
		t.Fatalf("reading pat: %v", err)
	}
	if u.RequirePasswordChange || u.DisplayName != "Pat" {
		t.Errorf("a refused save changed pat: %+v", u)
	}
}

func mustGet(t *testing.T, client *http.Client, address string) *http.Response {
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

func roleOf(t *testing.T, ctx context.Context, h *harness, actorID string) core.Role {
	t.Helper()
	members, err := h.server.ListMembers(ctx)
	if err != nil {
		t.Fatalf("listing members: %v", err)
	}
	for _, m := range members {
		if m.ActorID == actorID {
			return m.Role
		}
	}
	return ""
}

// Clearing a two-tenant person's second factor is refused too, and says so on
// the Manage dialog rather than on the error page.
func TestARefusedSecondFactorClearForATwoTenantMemberIsShownOnTheDialog(t *testing.T) {
	h := newHarness(t)
	client, patID := twoTenantMember(t, h)

	page := readAll(t, mustGet(t, client, h.baseURL+web.RouteUsers))
	form := url.Values{"id": {patID}, web.CSRFFieldName: {betweenQuotes(page, `name="`+web.CSRFFieldName+`" value="`)}}
	res, err := client.PostForm(h.baseURL+web.RouteUserTOTPClear, form)
	if err != nil {
		t.Fatalf("posting the clear: %v", err)
	}
	body := readAll(t, res)
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("the refused clear answered %d, want 403", res.StatusCode)
	}
	part := userForm(t, body, web.RouteUserTOTPClear, patID)
	if !strings.Contains(part, "Pat also belongs to another tenant, so only they or a global admin can clear their second factor") {
		t.Errorf("the second factor part of the dialog does not say why the clear was refused:\n%s", part)
	}
}
