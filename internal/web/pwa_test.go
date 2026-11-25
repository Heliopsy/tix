// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// manifestDoc is the manifest as a browser reads it.
type manifestDoc struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	ShortName       string `json:"short_name"`
	Lang            string `json:"lang"`
	StartURL        string `json:"start_url"`
	Scope           string `json:"scope"`
	Display         string `json:"display"`
	ThemeColor      string `json:"theme_color"`
	BackgroundColor string `json:"background_color"`
	Icons           []struct {
		Src, Sizes, Type, Purpose string
	} `json:"icons"`
	Shortcuts []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"shortcuts"`
	ShareTarget struct {
		Action  string            `json:"action"`
		Method  string            `json:"method"`
		Enctype string            `json:"enctype"`
		Params  map[string]string `json:"params"`
	} `json:"share_target"`
}

// manifestOf fetches and decodes the manifest the browser is served.
func manifestOf(t *testing.T, b *browser) manifestDoc {
	t.Helper()
	resp := b.get("/manifest.webmanifest")
	wantStatus(t, resp, http.StatusOK)
	if ct := resp.Header.Get("Content-Type"); ct != "application/manifest+json" {
		t.Errorf("the manifest is served as %q, want application/manifest+json", ct)
	}
	var m manifestDoc
	if err := json.Unmarshal([]byte(body(t, resp)), &m); err != nil {
		t.Fatalf("decoding the manifest: %v", err)
	}
	return m
}

func TestTheManifestDescribesTheApp(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	for _, tc := range []struct {
		actor, name, short string
	}{
		{"alice", "Acme Works", "Acme Works"},
		{"", "tix", "tix"},
	} {
		m := manifestOf(t, f.as(tc.actor))
		if m.Name != tc.name || m.ShortName != tc.short {
			t.Errorf("as %q the app is named %q (%q), want %q (%q)", tc.actor, m.Name, m.ShortName, tc.name, tc.short)
		}
		if m.ID != "/" || m.StartURL != "/" || m.Scope != "/" || m.Display != "standalone" {
			t.Errorf("as %q the app opens %q in scope %q as %q, id %q", tc.actor, m.StartURL, m.Scope, m.Display, m.ID)
		}
		if m.ThemeColor != "#ffffff" || m.BackgroundColor != "#ffffff" {
			t.Errorf("as %q the colours are %q and %q, not the light page", tc.actor, m.ThemeColor, m.BackgroundColor)
		}
	}
}

func TestTheManifestIconsAreServed(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	anon := f.as("")
	m := manifestOf(t, anon)
	purposes := map[string]bool{}
	for _, icon := range m.Icons {
		purposes[icon.Purpose] = true
		if icon.Sizes != "any" || icon.Type != "image/svg+xml" {
			t.Errorf("icon %s is %q %q, want an SVG of any size", icon.Src, icon.Sizes, icon.Type)
		}
		resp := anon.get(icon.Src)
		wantStatus(t, resp, http.StatusOK)
		if ct := resp.Header.Get("Content-Type"); ct != "image/svg+xml" {
			t.Errorf("icon %s is served as %q", icon.Src, ct)
		}
		if !strings.Contains(body(t, resp), "<svg") {
			t.Errorf("icon %s is not an SVG", icon.Src)
		}
	}
	if !purposes["any"] || !purposes["maskable"] {
		t.Errorf("the icons cover purposes %v, want any and maskable", purposes)
	}
}

func TestTheManifestOffersTheLauncherShortcutsInTheReadersLanguage(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	for _, tc := range []struct {
		lang  string
		names []string
	}{
		{"en", []string{"Quick add", "Inbox", "My work"}},
		{"bg", []string{"Бързо добавяне", "Входящи", "Моята работа"}},
	} {
		b := f.as("alice")
		b.lang = tc.lang
		m := manifestOf(t, b)
		if m.Lang != tc.lang {
			t.Errorf("the %s manifest says lang %q", tc.lang, m.Lang)
		}
		urls := []string{"/tasks?add=", "/inbox", "/mine"}
		if len(m.Shortcuts) != len(urls) {
			t.Fatalf("the %s manifest lists %d shortcuts, want %d", tc.lang, len(m.Shortcuts), len(urls))
		}
		for i, s := range m.Shortcuts {
			if s.Name != tc.names[i] || s.URL != urls[i] {
				t.Errorf("%s shortcut %d is %q at %q, want %q at %q", tc.lang, i, s.Name, s.URL, tc.names[i], urls[i])
			}
		}
	}
}

func TestTheManifestDeclaresTheShareTarget(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	st := manifestOf(t, f.as("alice")).ShareTarget
	if st.Action != "/share" || st.Method != http.MethodGet || st.Enctype != "application/x-www-form-urlencoded" {
		t.Errorf("the share target is %s %s (%s), want GET /share", st.Method, st.Action, st.Enctype)
	}
	for _, p := range []string{"title", "text", "url"} {
		if st.Params[p] != p {
			t.Errorf("the share target maps %s to %q", p, st.Params[p])
		}
	}
}

func TestEveryPageLinksTheManifest(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	for _, tc := range []struct{ actor, path string }{{"alice", "/tasks"}, {"", "/login"}} {
		page := f.as(tc.actor).page(tc.path)
		head := between(t, page, "<head>", "</head>")
		for _, want := range []string{
			`<link rel="manifest" href="/manifest.webmanifest" crossorigin="use-credentials">`,
			`<meta name="theme-color" content="#ffffff" media="(prefers-color-scheme: light)">`,
			`<meta name="theme-color" content="#14171b" media="(prefers-color-scheme: dark)">`,
			`<script src="/assets/pwa.js" defer></script>`,
		} {
			if !strings.Contains(head, want) {
				t.Errorf("%s: the head lacks %s", tc.path, want)
			}
		}
	}
}

// The theme colours are the page's own, so they must follow the stylesheet.
func TestTheThemeColoursAreThePageBackgrounds(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("assets/css/010-tokens.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(raw)
	light := between(t, css, ":root {", "}")
	dark := between(t, css, `:root[data-theme="dark"] {`, "}")
	if !strings.Contains(light, "--bg: #ffffff;") || !strings.Contains(dark, "--bg: #14171b;") {
		t.Errorf("--bg is no longer #ffffff light and #14171b dark; update pwa.go and layout.html")
	}
}

func TestSharingOpensQuickAddWithTheLine(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, query, line string
	}{
		{"title and url", "title=Release+notes&url=https%3A%2F%2Fexample.test%2Fnotes",
			"Release notes https://example.test/notes"},
		{"text holding the url", "title=Notes&text=Notes+at+https%3A%2F%2Fexample.test&url=https%3A%2F%2Fexample.test",
			"Notes at https://example.test"},
		{"text over lines", "text=first%0Asecond", "first second"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			b := f.as("alice")
			resp := b.get("/share?" + tc.query)
			_ = resp.Body.Close()
			wantStatus(t, resp, http.StatusSeeOther)
			loc, err := url.Parse(resp.Header.Get("Location"))
			if err != nil {
				t.Fatal(err)
			}
			if loc.Path != "/tasks" || loc.Query().Get("add") != tc.line {
				t.Fatalf("the share redirects to %q, want /tasks with add=%q", loc, tc.line)
			}
			box := pwaQuickBox(t, b.page(loc.String()))
			if !strings.Contains(box, `value="`+tc.line+`"`) || !strings.Contains(box, " autofocus") {
				t.Errorf("quick add does not hold %q focused:\n%s", tc.line, box)
			}
			if n := taskCount(t, f); n != 0 {
				t.Errorf("sharing created %d tasks before anyone pressed Add", n)
			}
		})
	}
}

func TestTheQuickAddShortcutFocusesAnEmptyBox(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	if box := pwaQuickBox(t, b.page("/tasks?add=")); !strings.Contains(box, " autofocus") {
		t.Errorf("the Quick add shortcut does not focus the box:\n%s", box)
	}
	if box := pwaQuickBox(t, b.page("/tasks")); strings.Contains(box, "autofocus") {
		t.Errorf("the plain task list focuses the box:\n%s", box)
	}
}

func TestASharedLineIsPreviewedAndNotCarriedOn(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "already here")
	page := b.page("/tasks?add=" + url.QueryEscape("zebra crossing #road"))
	preview := between(t, page, `<div id="quickadd-preview"`, "</div>")
	if !strings.Contains(preview, "road") {
		t.Errorf("the shared line's tag is not previewed:\n%s", preview)
	}
	if strings.Contains(page, "add=zebra") {
		t.Errorf("a link on the page carries the shared line on")
	}

	resp := b.postFrom("/tasks?add=zebra", "/tasks", url.Values{"project_ref": {"infra"}, "quick": {"1"}, "title": {"zebra"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if loc.Path != "/tasks" || loc.Query().Has("add") {
		t.Errorf("staying after the add returns to %q, which puts the line back in the box", loc)
	}
}

func TestASignedOutShareSignsInAndComesBack(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	resp := f.as("").get("/share?text=hello")
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if loc.Path != "/login" || loc.Query().Get("next") != "/share?text=hello" {
		t.Errorf("a signed-out share goes to %q, want sign-in returning to the share", loc)
	}
	if n := taskCount(t, f); n != 0 {
		t.Errorf("a signed-out share created %d tasks", n)
	}
}

func TestTheServiceWorkerIsServedFromTheRootUncached(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	resp := f.as("").get("/sw.js")
	wantStatus(t, resp, http.StatusOK)
	if ct := resp.Header.Get("Content-Type"); ct != "text/javascript; charset=utf-8" {
		t.Errorf("the worker is served as %q", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("the worker carries Cache-Control %q, want no-cache", cc)
	}
	want, err := os.ReadFile("assets/sw.js")
	if err != nil {
		t.Fatal(err)
	}
	if body(t, resp) != string(want) {
		t.Errorf("/sw.js is not assets/sw.js")
	}
	reg, err := os.ReadFile("assets/pwa.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(reg), `register("/sw.js", { scope: "/" })`) {
		t.Errorf("pwa.js does not register /sw.js at scope /")
	}
}

// jsComment matches a line comment, so the guard reads code and not the
// comment explaining what the code does not do.
var jsComment = regexp.MustCompile(`(?m)//.*$`)

// TestTheServiceWorkerCachesNothing holds the worker to storing no response
// and answering no request: a cached screen would outlive a sign-out.
func TestTheServiceWorkerCachesNothing(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("assets/sw.js")
	if err != nil {
		t.Fatal(err)
	}
	code := jsComment.ReplaceAllString(string(raw), "")
	if !strings.Contains(code, `addEventListener("fetch"`) {
		t.Fatalf("the worker has no fetch handler, so this guard reads the wrong file")
	}
	for _, banned := range []string{"caches", "Cache", "respondWith", "indexedDB", "localStorage", "importScripts", "fetch("} {
		if strings.Contains(code, banned) {
			t.Errorf("the service worker uses %s", banned)
		}
	}
}

// pwaQuickBox returns the quick add box's input tag.
func pwaQuickBox(t *testing.T, page string) string {
	t.Helper()
	form := between(t, page, `class="quickadd">`, "</form>")
	box := between(t, form, `<input name="title"`, ">")
	if box == "" {
		t.Fatalf("the page has no quick add box")
	}
	return box
}

// taskCount is how many tasks tenant acme holds.
func taskCount(t *testing.T, f *fixture) int {
	t.Helper()
	page, err := f.svc.ListTasks(f.ctx(), core.TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}
	return len(page.Tasks)
}
