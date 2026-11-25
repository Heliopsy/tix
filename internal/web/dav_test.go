// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/web"
)

// davSectionStart opens the Tasks in other apps section.
const davSectionStart = `<section class="setgroup" id="dav">`

// davSection returns the Tasks in other apps section of the settings screen,
// so an assertion about app passwords reads that section and not the page.
func davSection(t *testing.T, page string) string {
	t.Helper()
	section := between(t, page, davSectionStart, "</section>")
	if section == "" {
		t.Fatalf("the settings screen has no Tasks in other apps section:\n%s", page)
	}
	return section
}

// davHead is the section's heading row, where its create button sits.
func davHead(t *testing.T, section string) string {
	t.Helper()
	return between(t, section, `<div class="sechead">`, "</div>")
}

// davRow is the one row of the section listing an app password.
func davRow(t *testing.T, section, name string) string {
	t.Helper()
	at := strings.Index(section, `<span class="setname">`+name+`</span>`)
	if at < 0 {
		t.Fatalf("the section lists no app password named %q:\n%s", name, section)
	}
	start := strings.LastIndex(section[:at], `<div class="setrow dav-row"`)
	if start < 0 {
		t.Fatalf("the name %q sits in no password row:\n%s", name, section)
	}
	rest := section[start:]
	if end := strings.Index(rest[1:], `<div class="setrow dav-row"`); end >= 0 {
		rest = rest[:end+1]
	}
	return rest
}

// issuedValue reads one value the issued screen shows in its copy line.
func issuedValue(t *testing.T, page, id string) string {
	t.Helper()
	v := between(t, page, `<pre id="`+id+`" class="wraps"><code>`, "</code>")
	if v == "" {
		t.Fatalf("the issued screen shows no %s:\n%s", id, page)
	}
	return v
}

// davReader is a member with an email, the person an app password is for.
func davReader(t *testing.T, f *fixture) (*browser, core.User) {
	t.Helper()
	user := f.person("dav@example.com", "correct horse battery")
	return f.asActor(user.ID), user
}

func TestDAVSectionShowsTheAddressAndUsername(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b, user := davReader(t, f)
	page := b.page(web.RouteSettings)
	section := davSection(t, page)

	state := between(t, section, `<div class="setrow stacked" id="dav-state">`, `<div class="setrow" id="dav-empty">`)
	if got := between(t, state, `<pre id="dav-url" class="wraps"><code>`, "</code>"); got != f.server.URL+"/dav/" {
		t.Errorf("the section gives the address %q, want %q", got, f.server.URL+"/dav/")
	}
	if got := between(t, state, `<pre id="dav-username" class="wraps"><code>`, "</code>"); got != user.Email {
		t.Errorf("the section gives the username %q, want %q", got, user.Email)
	}
	if !strings.Contains(state, `data-copy-target="dav-url"`) || !strings.Contains(state, `data-copy-target="dav-username"`) {
		t.Errorf("the address and username carry no Copy button:\n%s", state)
	}
	if tip := between(t, state, `<details class="field-info tip">`, "</details>"); !strings.Contains(tip, "Apple Reminders") ||
		!strings.Contains(tip, "Thunderbird") || !strings.Contains(tip, "DAVx5") || !strings.Contains(tip, "Evolution") ||
		!strings.Contains(tip, "tick off, edit, add and cancel") || !strings.Contains(tip, "read-only") {
		t.Errorf("the (i) tip does not explain CalDAV, its apps, that they write back and the read-only choice:\n%s", state)
	}
	if !strings.Contains(davHead(t, section), `data-dlg-open="dav-new"`) {
		t.Errorf("the heading carries no button opening the create dialog:\n%s", section)
	}
	empty := between(t, section, `<div class="setrow" id="dav-empty">`, "</div>\n    </div>")
	if !strings.Contains(empty, `data-dlg-open="dav-new"`) {
		t.Errorf("the empty list does not carry the create button:\n%s", section)
	}
	if !strings.Contains(between(t, page, `<nav class="setnav"`, "</nav>"), `href="#dav"`) {
		t.Error("the settings navigation does not lead to the section")
	}
	before, _, _ := strings.Cut(page, davSectionStart)
	if strings.Count(before, "<details") != strings.Count(before, "</details>") {
		t.Error("the section sits inside a <details> fold")
	}
	if strings.Count(section, "<details") != strings.Count(section, `<details class="field-info`) {
		t.Errorf("the section folds something that is not an (i):\n%s", section)
	}
}

func TestCreatingAnAppPasswordShowsItOnce(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b, user := davReader(t, f)
	dlg := dialogBlock(b.page(web.RouteSettings), "dav-new")
	form := totpForm(t, dlg, web.RouteDAVPasswordCreate)
	for _, want := range []string{`name="name"`, `type="radio" name="collections" value="assigned" checked`,
		`value="projects"`, `value="all"`, `type="checkbox" name="read_only"`, `name="csrf_token"`} {
		if !strings.Contains(form, want) {
			t.Errorf("the create dialog's form lacks %s:\n%s", want, form)
		}
	}

	resp := b.post(web.RouteDAVPasswordCreate, url.Values{"name": {"iPhone"}, "collections": {"assigned"}})
	issued := body(t, resp)
	wantStatus(t, resp, http.StatusOK)
	password := issuedValue(t, issued, "dav-issued-password")
	if got := issuedValue(t, issued, "dav-issued-url"); got != f.server.URL+"/dav/" {
		t.Errorf("the issued screen gives the address %q", got)
	}
	if got := issuedValue(t, issued, "dav-issued-username"); got != user.Email {
		t.Errorf("the issued screen gives the username %q", got)
	}
	if !strings.Contains(issued, `data-copy-target="dav-issued-password"`) {
		t.Errorf("the password has no Copy button:\n%s", issued)
	}
	if !strings.Contains(between(t, issued, `<p class="backlink">`, "</p>"), `href="/settings#dav" data-way-back`) {
		t.Errorf("the issued screen has no way back to the section:\n%s", issued)
	}
	if resp.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("the issued screen may be cached: %q", resp.Header.Get("Cache-Control"))
	}
	if loc := resp.Header.Get("Location"); loc != "" {
		t.Errorf("creating redirected to %q, which puts nothing on screen", loc)
	}

	settings := b.page(web.RouteSettings)
	if strings.Contains(settings, password) {
		t.Fatal("settings shows the app password after it was issued")
	}
	row := davRow(t, davSection(t, settings), "iPhone")
	for _, want := range []string{"Assigned to me", "Created ", "Never used.", `data-dlg-open="dav-revoke-`} {
		if !strings.Contains(row, want) {
			t.Errorf("the password's row lacks %q:\n%s", want, row)
		}
	}
	if strings.Contains(row, "Revoked") {
		t.Errorf("a live password is marked revoked:\n%s", row)
	}
}

func TestARefusedAppPasswordComesBackWithItsDialogOpen(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b, _ := davReader(t, f)
	first := b.post(web.RouteDAVPasswordCreate, url.Values{"name": {"iPhone"}})
	_ = body(t, first)
	wantStatus(t, first, http.StatusOK)

	for _, tc := range []struct {
		name   string
		form   url.Values
		status int
		typed  string
	}{
		{"duplicate", url.Values{"name": {"iphone"}, "collections": {"projects"}, "read_only": {"on"}}, http.StatusConflict, `value="iphone"`},
		{"blank", url.Values{"name": {"  "}}, http.StatusBadRequest, ""},
		{"too long", url.Values{"name": {strings.Repeat("x", 65)}}, http.StatusBadRequest, ""},
	} {
		resp := b.post(web.RouteDAVPasswordCreate, tc.form)
		page := body(t, resp)
		wantStatus(t, resp, tc.status)
		if tag := dialogTag(t, page, "dav-new"); !strings.HasSuffix(tag, " open") {
			t.Errorf("%s: the refusal does not come back with the dialog open: %s", tc.name, tag)
		}
		dlg := dialogBlock(page, "dav-new")
		field := between(t, dlg, `<input id="dav-name"`, ">")
		if !strings.Contains(field, `aria-invalid="true"`) || !strings.Contains(between(t, dlg, field, "</fieldset>"), `class="field-error"`) {
			t.Errorf("%s: the message is not beside the name field:\n%s", tc.name, dlg)
		}
		if tc.typed != "" && !strings.Contains(field, tc.typed) {
			t.Errorf("%s: the dialog lost the name typed:\n%s", tc.name, dlg)
		}
		if tc.name == "duplicate" && (!strings.Contains(dlg, `value="projects" checked`) ||
			!strings.Contains(dlg, `name="read_only" value="on" checked`)) {
			t.Errorf("%s: the dialog lost the choices made:\n%s", tc.name, dlg)
		}
		if got := resp.Header.Get("HX-Replace-Url"); got != "/settings?dialog=dav-new" {
			t.Errorf("%s: the refusal asks htmx to show %q", tc.name, got)
		}
	}
}

func TestRevokingAnAppPasswordAsksFirst(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b, _ := davReader(t, f)
	made := b.post(web.RouteDAVPasswordCreate, url.Values{"name": {"Thunderbird"}})
	_ = body(t, made)
	wantStatus(t, made, http.StatusOK)

	page := b.page(web.RouteSettings)
	row := davRow(t, davSection(t, page), "Thunderbird")
	dialogID := "dav-revoke-" + between(t, row, `data-dlg-open="dav-revoke-`, `"`)
	action := "/settings/dav/passwords/" + strings.TrimPrefix(dialogID, "dav-revoke-") + "/revoke"
	dlg := dialogBlock(page, dialogID)
	outside := strings.Replace(page, dlg, "", 1)
	if dlg == "" || strings.Contains(outside, `action="`+action+`"`) {
		t.Fatalf("the revoke form is not only in its dialog:\n%s", row)
	}
	if !strings.Contains(row, `class="btn danger small" href="/settings?dialog=`+dialogID+`#`+dialogID+`" data-dlg-open="`+dialogID+`"`) {
		t.Errorf("the row has no red button opening its confirm dialog:\n%s", row)
	}
	confirm := totpForm(t, dlg, action)
	if !strings.Contains(confirm, `<button class="danger" type="submit">`) || !strings.Contains(confirm, `name="csrf_token"`) {
		t.Errorf("the dialog's own button is not the red one:\n%s", confirm)
	}
	if !strings.Contains(dlg, `data-dlg-close>Cancel</a>`) {
		t.Errorf("the confirm dialog has no Cancel:\n%s", dlg)
	}

	resp := b.post(action, url.Values{})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	wantSettingsReturn(t, resp, "dav")

	after := b.page(web.RouteSettings)
	row = davRow(t, davSection(t, after), "Thunderbird")
	if !strings.Contains(row, `<span class="chip">Revoked</span>`) || !strings.Contains(row, "Revoked ") {
		t.Errorf("the revoked password is not marked revoked:\n%s", row)
	}
	if strings.Contains(row, "data-dlg-open") || dialogBlock(after, dialogID) != "" {
		t.Errorf("a revoked password still offers Revoke:\n%s", row)
	}
}

func TestDisabledCalDAVShowsTheReason(t *testing.T) {
	t.Parallel()
	f := newFixtureService(t, []service.Option{service.WithCalDAV(false, -1, 0)})
	b, _ := davReader(t, f)
	ctx := core.WithTenant(core.WithActor(context.Background(), copyActor(f.actorA, core.RoleMember.Scopes(), core.RoleMember)),
		core.TenantScope{TenantID: f.tenantA.ID})
	info, err := f.svc.GetDAVInfo(ctx)
	if err != nil {
		t.Fatalf("GetDAVInfo: %v", err)
	}
	if info.Enabled || info.Reason == "" {
		t.Fatalf("the fixture's CalDAV is not off: %+v", info)
	}

	page := b.page(web.RouteSettings)
	section := davSection(t, page)
	if got := between(t, section, `<span class="sethelp" id="dav-reason">`, "</span>"); got != info.Reason {
		t.Errorf("the section says %q, want the reason %q", got, info.Reason)
	}
	if strings.Contains(page, `data-dlg-open="dav-new"`) || dialogBlock(page, "dav-new") != "" ||
		strings.Contains(page, `action="`+web.RouteDAVPasswordCreate+`"`) {
		t.Errorf("CalDAV is off and the screen still offers a new password:\n%s", section)
	}
	if strings.Contains(section, `id="dav-url"`) {
		t.Errorf("CalDAV is off and the section still gives an address:\n%s", section)
	}
}

// An agent holds no app passwords: the section says so instead of failing
// the screen.
func TestTheDAVSectionSaysWhenItDoesNotApply(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	agent := core.Actor{Kind: core.ActorAgent, Handle: "robot", Role: core.RoleMember, Scopes: []core.Scope{core.ScopeAll}}
	if err := f.store.Update(context.Background(), core.TenantScope{TenantID: f.tenantA.ID}, func(tx store.Tx) error {
		return tx.CreateActor(context.Background(), &agent)
	}); err != nil {
		t.Fatalf("creating the agent: %v", err)
	}
	resp := f.asActor(agent.ID).get(web.RouteSettings)
	page := body(t, resp)
	wantStatus(t, resp, http.StatusOK)
	section := davSection(t, page)
	if !strings.Contains(section, "Not applicable") || strings.Contains(page, `data-dlg-open="dav-new"`) {
		t.Errorf("an agent's section does not say it does not apply:\n%s", section)
	}
}
