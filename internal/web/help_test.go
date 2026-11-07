// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"encoding/json"
	"html"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/web"
)

// helpAnchor is a screen's help button.
var helpAnchor = regexp.MustCompile(`<a class="helplink" href="([^"]*)" target="_blank" rel="noopener" aria-label="([^"]*)"`)

// pageTitle is the screen's name in the document title, before the tenant's.
var pageTitle = regexp.MustCompile(`<title>(.*?) &middot; `)

// headerOpeners are the elements a screen's header row is drawn as, outermost
// first: a heading inside a .pagehead belongs to the .pagehead, not to the
// .headtip around the heading alone.
var headerOpeners = []string{`<div class="pagehead">`, `<header class="titlebar taskhead"`,
	`<div class="cform-head">`, `<div class="headtip">`}

// headerOf is the header element holding the page's h1: the element itself,
// balanced on its own tag, so a link anywhere else on the page is not in it.
func headerOf(t *testing.T, page string) string {
	t.Helper()
	h1 := strings.Index(page, "<h1")
	if h1 < 0 {
		t.Fatal("the page has no h1")
	}
	for _, open := range headerOpeners {
		start := strings.LastIndex(page[:h1], open)
		if start < 0 {
			continue
		}
		tag := open[1:strings.IndexAny(open, " >")]
		depth, i := 0, start
		for {
			o := strings.Index(page[i:], "<"+tag)
			c := strings.Index(page[i:], "</"+tag+">")
			if c < 0 {
				t.Fatalf("%s is never closed", open)
			}
			if o >= 0 && o < c {
				depth++
				i += o + 1
				continue
			}
			depth--
			i += c + len("</"+tag+">")
			if depth == 0 {
				break
			}
		}
		if i > h1 {
			return page[start:i]
		}
	}
	t.Fatal("the h1 is in no header row")
	return ""
}

// helpRoutes is the screenshot manifest's list of screens.
type helpRoutes struct {
	Routes []struct {
		Pattern string            `json:"pattern"`
		Skip    string            `json:"skip"`
		Public  bool              `json:"public"`
		Params  map[string]string `json:"params"`
	} `json:"routes"`
}

// TestEveryScreenHasAHelpButtonInItsHeader renders every screen the
// screenshot manifest captures and finds the (?) in its header row, named for
// the screen and opening the documentation for the running release in a new
// tab, landing on a section.
func TestEveryScreenHasAHelpButtonInItsHeader(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "scripts", "screenshots", "routes.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m helpRoutes
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	f := newFixture(t)
	f.makeGlobalAdmin()
	b := f.as("alice")
	ref := b.createTask("infra", "first task")
	if _, err := f.svc.SaveTemplate(f.ctx(), core.SaveTemplateInput{Name: "daily", TaskRef: ref}); err != nil {
		t.Fatal(err)
	}
	channel, _ := addChannel(t, f, b, url.Values{"name": {"phone"}, "url": {"https://ntfy.invalid/alice-tix"},
		"format": {"ntfy"}, "kinds": {"mention"}, "active": {"on"}})
	values := map[string]string{
		"project": "infra", "workflow": "default", "task": ref, "tenant": "acme",
		"template": templateID(t, f, "daily"),
		"channel":  strings.TrimPrefix(channel, "/settings/notifications/"),
		"filter":   saveFilter(t, b, url.Values{"name": {"Mine"}, "q": {"tag:ops"}}),
	}
	screens := 0
	for _, r := range m.Routes {
		if r.Skip != "" {
			continue
		}
		path := r.Pattern
		for placeholder, resolver := range r.Params {
			path = strings.ReplaceAll(path, "{"+placeholder+"}", values[resolver])
		}
		t.Run(r.Pattern, func(t *testing.T) {
			as := b
			if r.Public {
				as = f.as("")
			}
			resp := as.get(path)
			for hops := 0; resp.StatusCode == http.StatusSeeOther && hops < 3; hops++ {
				_ = resp.Body.Close()
				resp = as.get(resp.Header.Get("Location"))
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET %s = %d", path, resp.StatusCode)
			}
			page := body(t, resp)
			head := headerOf(t, page)
			found := helpAnchor.FindAllStringSubmatch(head, -1)
			if len(found) != 1 {
				t.Fatalf("GET %s: the header row holds %d help buttons, want 1:\n%s", path, len(found), head)
			}
			title := pageTitle.FindStringSubmatch(page)
			if title == nil || found[0][2] != "Help: "+title[1] {
				t.Errorf("GET %s: the help button is named %q, want Help: and the page title %v", path, found[0][2], title)
			}
			href := html.UnescapeString(found[0][1])
			doc, err := url.Parse(href)
			if err != nil || !strings.HasPrefix(doc.Path, web.RouteDocs+"/") || doc.Query().Get("section") == "" {
				t.Fatalf("GET %s: the help button opens %q, not a section through /docs", path, href)
			}
			resp = as.get(href)
			_ = resp.Body.Close()
			want := "/docs/" + strings.TrimPrefix(doc.Path, web.RouteDocs+"/") + "#" + doc.Query().Get("section")
			if resp.StatusCode != http.StatusFound || !strings.HasSuffix(resp.Header.Get("Location"), want) {
				t.Errorf("GET %s = %d to %q, want a redirect ending %s", href, resp.StatusCode, resp.Header.Get("Location"), want)
			}
		})
		screens++
	}
	if screens < 30 {
		t.Fatalf("checked %d screens; the manifest has moved or lost its routes", screens)
	}
}

// TestTheTaskDialogDrawsNoHelpButton keeps the (?) on the screen under the
// dialog: the dialog is a view of a task, not a screen of its own.
func TestTheTaskDialogDrawsNoHelpButton(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "first task")
	if page := b.page("/tasks/" + ref); !helpAnchor.MatchString(page) {
		t.Fatal("the task screen has no help button")
	}
	req, err := http.NewRequest(http.MethodGet, f.server.URL+"/tasks/"+ref, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(web.DialogHeader, "1")
	b.stamp(req)
	resp, err := b.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if page := body(t, resp); helpAnchor.MatchString(page) || !strings.Contains(page, `id="dlg-`) {
		t.Errorf("the task drawn for the dialog carries a help button, or is not the dialog's drawing")
	}
}

// TestTheDocsRedirectLandsOnACheckedSection holds the section to the shape of
// a heading anchor: anything else is dropped rather than put in the Location.
func TestTheDocsRedirectLandsOnACheckedSection(t *testing.T) {
	t.Parallel()
	b := newFixture(t).as("")
	cases := map[string]string{
		"/docs/web-ui.md?section=the-task-list":     "/docs/web-ui.md#the-task-list",
		"/docs/web-ui.md":                           "/docs/web-ui.md",
		"/docs/web-ui.md?section=Bad%20Section":     "/docs/web-ui.md",
		"/docs/web-ui.md?section=x%0d%0aSet-Cookie": "/docs/web-ui.md",
		"/docs/web-ui.md?section=%23top":            "/docs/web-ui.md",
	}
	for path, want := range cases {
		resp := b.get(path)
		_ = resp.Body.Close()
		if loc := resp.Header.Get("Location"); resp.StatusCode != http.StatusFound || !strings.HasSuffix(loc, want) {
			t.Errorf("GET %s = %d to %q, want a redirect ending %s", path, resp.StatusCode, loc, want)
		}
	}
}
