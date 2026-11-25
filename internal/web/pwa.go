// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/heliopsy/tix/internal/i18n"
)

// The installable app: its manifest, the share target the manifest names and
// the service worker a browser looks for before it offers Install.
const (
	RouteManifest      = "/manifest.webmanifest"
	RouteServiceWorker = "/sw.js"
	RouteShareTarget   = "/share"

	// QuickAddParam puts a line in the task list's quick add box and focuses
	// it. The Quick add shortcut sends it empty.
	QuickAddParam = "add"
)

// Page colours the manifest and the theme-color meta name. They must stay
// equal to --bg in assets/css/010-tokens.css's light scheme and dark scheme.
const (
	pageLight = "#ffffff"
	pageDark  = "#14171b"
)

// manifestShortNameMax is the longest tenant name a launcher label shows
// whole; past it the label is tix.
const manifestShortNameMax = 12

// webManifest is the part of the Web App Manifest tix fills in.
type webManifest struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	ShortName       string             `json:"short_name"`
	Lang            string             `json:"lang"`
	StartURL        string             `json:"start_url"`
	Scope           string             `json:"scope"`
	Display         string             `json:"display"`
	ThemeColor      string             `json:"theme_color"`
	BackgroundColor string             `json:"background_color"`
	Icons           []manifestIcon     `json:"icons"`
	Shortcuts       []manifestShortcut `json:"shortcuts"`
	ShareTarget     manifestShare      `json:"share_target"`
}

// manifestIcon is one icon a launcher may draw.
type manifestIcon struct {
	Src     string `json:"src"`
	Sizes   string `json:"sizes"`
	Type    string `json:"type"`
	Purpose string `json:"purpose"`
}

// manifestShortcut is one entry of the launcher's menu for the app.
type manifestShortcut struct {
	Name      string `json:"name"`
	ShortName string `json:"short_name"`
	URL       string `json:"url"`
}

// manifestShare declares the route text shared from another app arrives at.
type manifestShare struct {
	Action  string            `json:"action"`
	Method  string            `json:"method"`
	Enctype string            `json:"enctype"`
	Params  map[string]string `json:"params"`
}

// pwaRoutes are the manifest, the share target and the service worker.
func (h *handler) pwaRoutes() []route {
	return []route{
		public(getFile(RouteManifest, h.serveManifest)),
		public(getFile(RouteServiceWorker, serveWorker)),
		getFile(RouteShareTarget, shareTarget),
	}
}

// serveManifest answers the manifest in the reader's language, named after
// their tenant when they are signed in.
func (h *handler) serveManifest(w http.ResponseWriter, r *http.Request) error {
	body, err := json.Marshal(h.manifestFor(r))
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, err = w.Write(body)
	return err
}

// manifestFor builds the manifest one request is answered with.
func (h *handler) manifestFor(r *http.Request) webManifest {
	lang := langOf(r)
	name := h.brand(r).Title
	short := name
	if utf8.RuneCountInString(short) > manifestShortNameMax {
		short = "tix"
	}
	shortcut := func(label, href string) manifestShortcut {
		text := i18n.T(lang, label)
		return manifestShortcut{Name: text, ShortName: text, URL: href}
	}
	return webManifest{
		ID: RouteRoot, Name: name, ShortName: short, Lang: lang,
		StartURL: RouteRoot, Scope: RouteRoot, Display: "standalone",
		ThemeColor: pageLight, BackgroundColor: pageLight,
		Icons: []manifestIcon{
			{Src: "/assets/tix.svg", Sizes: "any", Type: "image/svg+xml", Purpose: "any"},
			{Src: "/assets/tix-maskable.svg", Sizes: "any", Type: "image/svg+xml", Purpose: "maskable"},
		},
		Shortcuts: []manifestShortcut{
			shortcut("Quick add", RouteTasks+"?"+QuickAddParam+"="),
			shortcut("Inbox", RouteInbox),
			shortcut("My work", RouteMine),
		},
		ShareTarget: manifestShare{
			Action: RouteShareTarget, Method: http.MethodGet,
			Enctype: "application/x-www-form-urlencoded",
			Params:  map[string]string{"title": "title", "text": "text", "url": "url"},
		},
	}
}

// serveWorker answers the service worker from the root, so its scope is the
// whole interface, and uncached, so a changed worker is picked up at once.
func serveWorker(w http.ResponseWriter, _ *http.Request) error {
	body, err := fs.ReadFile(assetFS, "assets/sw.js")
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", assetContentTypes[".js"])
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, err = w.Write(body)
	return err
}

// shareTarget takes text another app shared and opens quick add with it. It
// only redirects: the task is created by the reader's own Add.
func shareTarget(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	line := sharedLine(q.Get("title"), q.Get("text"), q.Get("url"))
	http.Redirect(w, r, RouteTasks+"?"+url.Values{QuickAddParam: {line}}.Encode(), http.StatusSeeOther)
	return nil
}

// sharedLine joins a share into one quick add line the way the app does: the
// title leads unless the text already starts with it, and the address follows
// unless the text already holds it.
func sharedLine(title, text, link string) string {
	title, text, link = oneLine(title), oneLine(text), oneLine(link)
	line := text
	switch {
	case title == "" || strings.HasPrefix(text, title):
	case text == "":
		line = title
	default:
		line = title + " " + text
	}
	if link != "" && !strings.Contains(line, link) {
		line = strings.TrimSpace(line + " " + link)
	}
	return line
}

// oneLine folds every run of whitespace, newlines included, into one space.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// quickAddPrefill reads the line a share or the Quick add shortcut asked for,
// and returns the request without it, so the pager, the filter links and the
// live refresh built from that request do not carry it on.
func quickAddPrefill(r *http.Request) (*http.Request, string, bool) {
	q := r.URL.Query()
	if !q.Has(QuickAddParam) {
		return r, "", false
	}
	line := oneLine(q.Get(QuickAddParam))
	q.Del(QuickAddParam)
	out := r.Clone(r.Context())
	out.URL.RawQuery = q.Encode()
	return out, line, true
}
