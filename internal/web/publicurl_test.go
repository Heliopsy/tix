// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
)

const publicURL = "https://tix.example.com/tix"

// servePublic puts the configured public address on every request, as the
// router's middleware does, and the host the request came by when served is
// set, as tenant resolution does.
func servePublic(f *fixture, served *core.ServedHost) {
	next := f.server.Config.Handler
	f.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := httpapi.WithPublicURL(r.Context(), publicURL)
		if served != nil {
			ctx = core.WithServedHost(ctx, *served)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// rootTag is the page's <html> start tag and nothing else of it.
func rootTag(t *testing.T, page string) string {
	t.Helper()
	start := strings.Index(page, "<html")
	if start < 0 {
		t.Fatal("the page has no <html> tag")
	}
	end := strings.Index(page[start:], ">")
	return page[start : start+end+1]
}

// On a host serving several tenants the links the interface draws are under
// the public address, and the page tells share.js to copy under it too.
func TestLinksPreferThePublicAddressOnASharedHost(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	servePublic(f, &core.ServedHost{Host: "tix.lan", Shared: true})
	b := f.as("alice")
	ref := b.createTask("infra", "renew the certificate")
	page := b.page("/tasks/" + ref)

	if tag := rootTag(t, page); !strings.Contains(tag, `data-share-origin="`+publicURL+`"`) {
		t.Errorf("the root carries no data-share-origin: %s", tag)
	}
	menu := shareMenu(t, page, "share-"+ref)
	if want := `<a href="` + publicURL + `/tasks/` + ref + `" class="mono" data-share-url>`; !strings.Contains(menu, want) {
		t.Errorf("the Share menu's address is not under the public address:\n%s", menu)
	}
	if dlg := dialogBlock(page, "handoff"); !strings.Contains(dlg, publicURL) || strings.Contains(dlg, f.server.URL) {
		t.Errorf("the hand-off dialog does not name the public address:\n%s", dlg)
	}
	cli := connectStep(t, f.as("alice").page("/connect"), "cli")
	if !strings.Contains(cli, "--server "+publicURL+" ") {
		t.Errorf("the Connect screen does not name the public address:\n%s", cli)
	}
}

// On a tenant's own domain, and with no public address, the links stay on
// the host the reader is on and the page draws no origin of its own.
func TestLinksStayOnTheRequestHostOtherwise(t *testing.T) {
	t.Parallel()
	for name, served := range map[string]*core.ServedHost{
		"own domain": {Host: "127.0.0.1", Owner: "acme"},
		"unset":      nil,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			if served != nil {
				servePublic(f, served)
			}
			b := f.as("alice")
			ref := b.createTask("infra", "renew the certificate")
			page := b.page("/tasks/" + ref)
			if tag := rootTag(t, page); strings.Contains(tag, "data-share-origin") {
				t.Errorf("the root carries data-share-origin: %s", tag)
			}
			menu := shareMenu(t, page, "share-"+ref)
			if want := `<a href="` + f.server.URL + `/tasks/` + ref + `"`; !strings.Contains(menu, want) {
				t.Errorf("the Share menu's address left the request host:\n%s", menu)
			}
		})
	}
}
