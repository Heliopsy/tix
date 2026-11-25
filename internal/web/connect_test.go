// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/web"
)

// issueConnectToken posts the connect screen's token form.
func issueConnectToken(t *testing.T, b *browser, form url.Values) *http.Response {
	t.Helper()
	form.Set("csrf_token", b.csrf())
	return b.postRaw("/connect/token", form)
}

// connectPane returns one main tab's panel of the connect screen, from its
// opening tag, so the hidden attribute is part of what it returns.
func connectPane(t *testing.T, page, id string) string {
	t.Helper()
	start := `<section class="connect-step connect-pane" role="tabpanel" id="` + id + `"`
	body := between(t, page, start, "</section>")
	if body == "" {
		return ""
	}
	return start + body
}

// connectCredential returns the shared first step, up to the tab strip.
func connectCredential(t *testing.T, page string) string {
	t.Helper()
	return between(t, page, `<section class="connect-step" id="token">`, `<section class="connect-step" id="connect">`)
}

// connectStrip returns the main tab strip.
func connectStrip(t *testing.T, page string) string {
	t.Helper()
	return between(t, page, `<div class="tabset-list connect-tablist" role="tablist"`, "</div>")
}

// tabLink returns the opening tag of the tab link called id in a strip.
func tabLink(t *testing.T, strip, id string) string {
	t.Helper()
	return between(t, strip, `<a id="`+id+`"`, ">")
}

// paneShown reports whether a panel's opening tag lacks the hidden attribute.
func paneShown(panel string) bool {
	open, _, _ := strings.Cut(panel, ">")
	return panel != "" && !strings.Contains(open, " hidden")
}

func TestConnectScreenNamesThisServerAndTenant(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := f.as("alice").page("/connect")

	cli := connectPane(t, page, "cli")
	if !strings.Contains(cli, "tix ctx add acme --server "+f.server.URL+" --token ") ||
		!strings.Contains(cli, "--tenant acme --use") {
		t.Errorf("the CLI step does not name this server and tenant:\n%s", cli)
	}
	mcp := connectPane(t, page, "mcp")
	if !strings.Contains(mcp, "--env TIX_SERVER_URL="+f.server.URL+" --env TIX_TOKEN=") {
		t.Errorf("the MCP step does not name this server:\n%s", mcp)
	}
	if strings.Contains(mcp, "TIX_SERVER_URL=URL") || strings.Contains(mcp, `&#34;URL&#34;`) {
		t.Errorf("the MCP step still carries the docs' URL placeholder:\n%s", mcp)
	}
	for _, client := range []string{"Claude Code", "Codex", "OpenCode", "Gemini CLI", "Claude Desktop", "Cursor", "VS Code", "Zed"} {
		if !strings.Contains(mcp, `role="tab"`) || !strings.Contains(mcp, ">"+client+"</a>") {
			t.Errorf("the MCP step has no tab for %s", client)
		}
	}
	if n := strings.Count(page, "data-token-slot>$TIX_TOKEN</span>"); n < 15 {
		t.Errorf("the screen marks %d token positions, want one in every command that needs it", n)
	}
	ssh := connectPane(t, page, "ssh")
	if strings.Contains(ssh, "ssh -p") || strings.Contains(ssh, "@127.0.0.1") || !strings.Contains(ssh, "runs no SSH listener") {
		t.Errorf("a server with no SSH listener shows SSH commands, or does not say it runs none:\n%s", ssh)
	}
}

// TestConnectTokenIsShownOnceWithItsScopes issues a token on the screen and
// reads it back: once, in the secret block the script fills the commands
// from, with the scopes it was granted, and never again.
func TestConnectTokenIsShownOnceWithItsScopes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := issueConnectToken(t, b, url.Values{"name": {"mcp-agent"}, "expires": {"30d"},
		"scopes": {"task:read", "task:claim"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	if loc := resp.Header.Get("Location"); !strings.HasPrefix(loc, "/connect") {
		t.Fatalf("issuing went to %q, want back to /connect", loc)
	}

	page := b.page("/connect")
	secret := between(t, page, `<section class="secret"`, "</section>")
	value := between(t, secret, `<code id="connect-token" data-token-value>`, "</code>")
	if !strings.HasPrefix(value, "tix_") {
		t.Fatalf("the secret does not carry the token for the script, it carries %q:\n%s", value, secret)
	}
	if n := strings.Count(page, value); n != 1 {
		t.Errorf("the token appears %d times on the page, want once: the commands are filled by the script", n)
	}
	scopes := between(t, secret, `<p class="secret-scopes">`, "</p>")
	if !strings.Contains(scopes, ">task:read<") || !strings.Contains(scopes, ">task:claim<") || strings.Contains(scopes, "task:write") {
		t.Errorf("the secret does not list exactly the scopes granted:\n%s", scopes)
	}
	if again := b.page("/connect"); strings.Contains(again, value) {
		t.Errorf("the token is shown a second time")
	}
}

func TestConnectWithoutTokenAdminDrawsNoForm(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := f.as("viewer").page("/connect")
	if strings.Contains(page, `action="/connect/token"`) {
		t.Errorf("a reader without token:admin is offered the token form")
	}
	if !strings.Contains(connectCredential(t, page), "needs the token:admin scope") {
		t.Errorf("the token step does not say why there is no form")
	}
}

// scopeBox is the checkbox for one scope in a picker.
func scopeBox(t *testing.T, form, scope string) string {
	t.Helper()
	return between(t, form, `<input type="checkbox" name="scopes" value="`+scope+`"`, ">")
}

// TestScopePickerShowsUnheldScopesDisabled checks every scope is drawn, the
// ones the reader cannot grant disabled and saying why, and * only for a
// reader who holds it.
func TestScopePickerShowsUnheldScopesDisabled(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	for _, path := range []string{"/admin/tokens?dialog=token-new", "/connect"} {
		limited := f.as("tokenkeeper").page(path)
		picker := between(t, limited, "<fieldset class=\"scopepick\" data-scope-picker>", `data-scope-summary`)
		if box := scopeBox(t, picker, "task:read"); strings.Contains(box, "disabled") {
			t.Errorf("%s: a held scope is disabled: %s", path, box)
		}
		if box := scopeBox(t, picker, "task:write"); !strings.Contains(box, "disabled") {
			t.Errorf("%s: a scope the reader does not hold can be ticked: %s", path, box)
		}
		if !strings.Contains(picker, "You do not hold it, so you cannot grant it.") {
			t.Errorf("%s: a disabled scope does not say why", path)
		}
		if strings.Contains(picker, `value="*"`) {
			t.Errorf("%s: * is offered to a reader who does not hold it", path)
		}
		admin := f.as("alice").page(path)
		if !strings.Contains(admin, `name="scopes" value="*" data-scope-star`) {
			t.Errorf("%s: * is not offered as its own choice to an administrator", path)
		}
	}
}

// TestAnUnheldScopeIsRefusedWhenPosted is the server half of the disabled
// checkbox: a forged submission naming a scope the reader lacks is refused.
func TestAnUnheldScopeIsRefusedWhenPosted(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("tokenkeeper")
	for _, path := range []string{"/admin/tokens", "/connect/token"} {
		form := url.Values{"name": {"forged-" + strings.Trim(path, "/")[:5]}, "scopes": {"task:read", "task:write"},
			"csrf_token": {b.csrf()}}
		resp := b.postRaw(path, form)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("POST %s with an unheld scope = %d, want 403", path, resp.StatusCode)
		}
	}
}

func TestConnectScreenShowsTheSSHAddress(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	cases := []struct {
		listen, public, want string
	}{
		{"0.0.0.0:2222", "tix.example.com:2020", "ssh -p 2020 acme@tix.example.com"},
		{"127.0.0.1:2222", "", "ssh -p 2222 acme@127.0.0.1"},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(f.authenticate(web.Handler(f.svc, web.WithSSH(tc.listen, tc.public))))
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/connect", nil)
		req.Header.Set(actorHeader, "alice")
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		page := readAll(t, resp)
		_ = resp.Body.Close()
		srv.Close()
		ssh := connectPane(t, page, "ssh")
		for _, want := range []string{tc.want + "</code>", tc.want + " task ls -o json", tc.want + " export &gt; snapshot.ndjson"} {
			if !strings.Contains(ssh, want) {
				t.Errorf("listen %s, public %q: the SSH tab does not read %q:\n%s", tc.listen, tc.public, want, ssh)
			}
		}
	}
}

// docsHref matches a link into the documentation.
var docsHref = regexp.MustCompile(`href="(/docs(?:/[^"#]*)?)(?:#[^"]*)?"`)

// TestEveryDocsLinkRedirectsToADocument follows every documentation link the
// menu and the connect screen draw and checks each one lands on a page.
func TestEveryDocsLinkRedirectsToADocument(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	page := b.page("/connect")
	menu := nav(t, page)
	if !strings.Contains(menu, `<a href="/docs" data-rail-name="Docs" target="_blank" rel="noopener"`) {
		t.Errorf("the menu has no Docs entry opening in a new tab")
	}
	if !strings.Contains(menu, `<a href="/connect" data-rail-name="Connect an agent"`) {
		t.Errorf("the menu has no Connect an agent entry")
	}
	links := docsHref.FindAllStringSubmatch(page, -1)
	if len(links) < 8 {
		t.Fatalf("found %d docs links on the connect screen, want the menu's and one per step", len(links))
	}
	for _, m := range links {
		resp := b.get(m[1])
		_ = resp.Body.Close()
		loc := resp.Header.Get("Location")
		if resp.StatusCode != http.StatusFound || !strings.HasPrefix(loc, "https://github.com/heliopsy/tix/blob/") {
			t.Errorf("GET %s = %d to %q, want a redirect into the documentation", m[1], resp.StatusCode, loc)
		}
	}
	for _, bad := range []string{"/docs/evil.md", "/docs/..%2fREADME.md", "/docs/mcp"} {
		resp := b.get(bad)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound || resp.Header.Get("Location") != "" {
			t.Errorf("GET %s = %d to %q, want 404 and no redirect", bad, resp.StatusCode, resp.Header.Get("Location"))
		}
	}
}

// externalRef is an attribute that would make the browser reach off this server.
var externalRef = regexp.MustCompile(`(?:href|src|action)="(?:https?:)?//`)

// TestConnectScreenLinksNowhereOffTheServer narrows the site-wide external
// origin guard for the one screen whose commands are full of URLs: the
// commands are text, and no attribute points off the server.
func TestConnectScreenLinksNowhereOffTheServer(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	if m := externalRef.FindString(f.as("alice").page("/connect")); m != "" {
		t.Errorf("the connect screen references an external origin: %s", m)
	}
}

// TestConnectTabsAreLinksTheServerDraws walks every main tab as a browser
// without scripting would: each is a link to ?tab=NAME, and following it draws
// that tab selected, its panel shown and every other panel hidden.
func TestConnectTabsAreLinksTheServerDraws(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	panes := []string{"cli", "mcp", "ssh", "api", "skill"}
	for _, path := range []string{"/connect", "/connect?tab=nope"} {
		page := b.page(path)
		if tab := tabLink(t, connectStrip(t, page), "tab-cli"); !strings.Contains(tab, `aria-selected="true"`) {
			t.Errorf("%s: the CLI tab is not the one selected: %s", path, tab)
		}
		if !paneShown(connectPane(t, page, "cli")) || paneShown(connectPane(t, page, "mcp")) {
			t.Errorf("%s: the CLI panel is not the one shown", path)
		}
	}
	first := b.page("/connect")
	for _, id := range panes {
		tab := tabLink(t, connectStrip(t, first), "tab-"+id)
		if !strings.Contains(tab, `role="tab" href="/connect?tab=`+id+`"`) {
			t.Errorf("the %s tab is not a link to ?tab=%s: %s", id, id, tab)
		}
		page := b.page("/connect?tab=" + id)
		strip := connectStrip(t, page)
		for _, other := range panes {
			selected := strings.Contains(tabLink(t, strip, "tab-"+other), `aria-selected="true"`)
			shown := paneShown(connectPane(t, page, other))
			if selected != (other == id) || shown != (other == id) {
				t.Errorf("?tab=%s: the %s tab is selected %v and shown %v", id, other, selected, shown)
			}
		}
	}
}

// TestConnectNestedTabsKeepTheMainTab follows an MCP client link: it keeps
// tab=mcp, and the client it names is the one shown.
func TestConnectNestedTabsKeepTheMainTab(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	mcp := connectPane(t, b.page("/connect?tab=mcp"), "mcp")
	codex := between(t, mcp, `<a id="tab-client-codex"`, ">")
	if !strings.Contains(codex, `href="/connect?client=codex&amp;tab=mcp"`) {
		t.Fatalf("the Codex tab does not link to its client with the MCP tab kept: %s", codex)
	}
	page := b.page("/connect?client=codex&tab=mcp")
	mcp = connectPane(t, page, "mcp")
	if !paneShown(mcp) {
		t.Fatalf("?tab=mcp&client=codex does not show the MCP tab")
	}
	shown := func(client string) bool {
		return paneShown(`<div` + between(t, mcp, `<div class="tabpanel" role="tabpanel" id="panel-client-`+client+`"`, "</div>"))
	}
	if !shown("codex") || shown("claude-code") || shown("zed") {
		t.Errorf("?client=codex does not show Codex alone:\n%s", mcp)
	}
	if !strings.Contains(between(t, mcp, `id="panel-client-codex"`, `id="panel-client-opencode"`), "codex mcp add tix") {
		t.Errorf("the Codex panel does not carry its command")
	}
	if tab := between(t, mcp, `<a id="tab-client-codex"`, ">"); !strings.Contains(tab, `aria-selected="true"`) {
		t.Errorf("the Codex tab is not selected: %s", tab)
	}
}

// TestEachConnectTabCarriesItsCommandsAndHelp holds every tab to the commands
// it exists for and to a help link into the documentation section about it.
func TestEachConnectTabCarriesItsCommandsAndHelp(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	srv := httptest.NewServer(f.authenticate(web.Handler(f.svc, web.WithSSH("127.0.0.1:2222", "tix.example.com:2020"))))
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/connect", nil)
	req.Header.Set(actorHeader, "alice")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	page := readAll(t, resp)
	_ = resp.Body.Close()
	cases := map[string]struct {
		help string
		want []string
	}{
		"cli": {"configuration.md?section=contexts", []string{"curl -fsSL https://tix.red/install.sh",
			"tix ctx add acme --server " + srv.URL, "tix task ls --limit 5",
			"tix --server " + srv.URL + " login --tenant acme --email alice@acme.test --password -"}},
		"mcp": {"mcp.md?section=wiring-it-into-a-client", []string{"claude mcp add tix --env TIX_SERVER_URL=" + srv.URL,
			"codex mcp add tix", "[mcp_servers.tix]", "tix mcp --list"}},
		"ssh": {"deployment.md?section=commands-over-ssh", []string{"tix user key add --file ~/.ssh/id_ed25519.pub",
			"ssh -p 2020 acme@tix.example.com task ls -o json", "ssh -p 2020 acme@tix.example.com watch -o ndjson"}},
		"api": {"api.md?section=authentication", []string{"Authorization: Bearer", srv.URL + "/api/v1/whoami"}},
		"skill": {"agents.md?section=the-shape-of-a-worker", []string{"https://tix.red/skill.md -o .claude/skills/tix/SKILL.md",
			"&gt;&gt; AGENTS.md"}},
	}
	for id, tc := range cases {
		pane := connectPane(t, page, id)
		if pane == "" {
			t.Errorf("there is no %s tab panel", id)
			continue
		}
		for _, want := range tc.want {
			if !strings.Contains(pane, want) {
				t.Errorf("the %s tab does not carry %q", id, want)
			}
		}
		if !strings.Contains(between(t, pane, `<div class="connect-head">`, "</div>"), `class="helplink" href="/docs/`+tc.help+`"`) {
			t.Errorf("the %s tab has no help link to %s", id, tc.help)
		}
	}
}

// TestTheTokenStepIsDrawnOnceAboveTheTabs checks the credential is a shared
// first step: one form, before the strip, whichever tab is open.
func TestTheTokenStepIsDrawnOnceAboveTheTabs(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := f.as("alice").page("/connect?tab=api")
	if n := strings.Count(page, `action="/connect/token`); n != 1 {
		t.Fatalf("the token form is drawn %d times, want once", n)
	}
	cred := connectCredential(t, page)
	if !strings.Contains(cred, `action="/connect/token?tab=api"`) {
		t.Errorf("the token form is not in the first step, or does not come back to the open tab:\n%s", cred)
	}
	for _, want := range []string{"tix token create mcp-agent", "tix token rotate mcp-agent --grace 10m", "your enrolled SSH key is the credential"} {
		if !strings.Contains(cred, want) {
			t.Errorf("the credential step does not carry %q", want)
		}
	}
	if strings.Contains(connectPane(t, page, "api"), "<form") {
		t.Errorf("the API tab draws a form of its own")
	}
}

// TestIssuingATokenComesBackToTheOpenTab posts the form from a tab and
// checks the redirect names it, so the reader is not dropped on the CLI tab.
func TestIssuingATokenComesBackToTheOpenTab(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	form := url.Values{"name": {"mcp-agent"}, "expires": {"30d"}, "scopes": {"task:read"}, "csrf_token": {b.csrf()}}
	resp := b.postRaw("/connect/token?tab=mcp&client=zed&evil=1", form)
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	if loc := resp.Header.Get("Location"); !strings.HasPrefix(loc, "/connect?client=zed&tab=mcp&flash=") {
		t.Errorf("issuing from the MCP tab went to %q, want back to it with nothing else kept", loc)
	}
}
