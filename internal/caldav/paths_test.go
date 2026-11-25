// SPDX-License-Identifier: AGPL-3.0-or-later

package caldav

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestParsePath(t *testing.T) {
	p := &core.DAVPrincipal{TenantKey: "acme", ActorID: "01abc"}
	tests := []struct {
		path string
		want target
		ok   bool
	}{
		{"/dav", target{kind: kindRoot}, true},
		{"/dav/", target{kind: kindRoot}, true},
		{"/dav/acme/principals/01abc/", target{kind: kindPrincipal}, true},
		{"/dav/acme/principals/01abc", target{kind: kindPrincipal}, true},
		{"/dav/acme/calendars/01abc/", target{kind: kindHome}, true},
		{"/dav/acme/calendars/01abc/assigned/", target{kind: kindCollection, collection: "assigned"}, true},
		{"/dav/acme/calendars/01abc/assigned", target{kind: kindCollection, collection: "assigned"}, true},
		{"/dav/acme/calendars/01abc/p-01x/t1.ics", target{kind: kindObject, collection: "p-01x", name: "t1"}, true},
		{"/dav/globex/calendars/01abc/", target{}, false},
		{"/dav/ACME/calendars/01abc/", target{}, false},
		{"/dav/acme/calendars/01other/", target{}, false},
		{"/dav/acme/principals/01other/", target{}, false},
		{"/dav/acme/calendars/01abc/assigned/t1.ics/", target{}, false},
		{"/dav/acme/calendars/01abc/assigned/t1", target{}, false},
		{"/dav/acme/calendars/01abc/assigned/.ics", target{}, false},
		{"/dav/acme/calendars/01abc/assigned/t1.ics/more", target{}, false},
		{"/dav/acme/principals/01abc/assigned/", target{}, false},
		{"/dav/acme/addressbooks/01abc/", target{}, false},
		{"/dav/acme//calendars/01abc/", target{}, false},
		{"/dav/acme/", target{}, false},
		{"/dav/acme/calendars/", target{}, false},
		{"/davx/", target{}, false},
		{"/api/v1/tasks", target{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got, ok := parsePath(tc.path, p)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("parsePath = %+v, %v; want %+v, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestHrefsAreEscapedAbsolutePaths(t *testing.T) {
	p := &core.DAVPrincipal{TenantKey: "acme", ActorID: "01abc"}
	if got := objectHref(p, "assigned", "a b"); got != "/dav/acme/calendars/01abc/assigned/a%20b.ics" {
		t.Errorf("object href %q", got)
	}
	got, ok := parseHref("https://tix.example/dav/acme/calendars/01abc/assigned/a%20b.ics", p)
	if !ok || got.name != "a b" {
		t.Errorf("an absolute href read as %+v, %v", got, ok)
	}
}

// TestNoPathReachesAnotherTenantOrPerson uses two tenants with the same keys,
// the same project keys and handles, and one account that is a member of
// both. Every crossing path answers exactly what nonsense answers.
func TestNoPathReachesAnotherTenantOrPerson(t *testing.T) {
	f := newFixture(t)
	carol, err := f.svc.CreateUser(adminCtx(f.acme), core.CreateUserInput{Email: "carol@example.com",
		Handle: "carol", Role: core.RoleMember})
	if err != nil {
		t.Fatal(err)
	}
	mine := f.task(f.acme, "mine", f.acme.bob.ID)
	theirs := f.task(f.globex, "theirs", f.globex.bob.ID)
	acme := f.password(f.acme.bob, core.DAVCollectionsAll)
	globex := f.password(f.globex.bob, core.DAVCollectionsAll)
	ctx := davCtx(f.acme.scope.TenantID, acme)

	bob, gbob := strings.ToLower(f.acme.bob.ID), strings.ToLower(f.globex.bob.ID)
	ownHome := "/dav/acme/calendars/" + bob + "/"
	ownInfra := ownHome + "p-" + strings.ToLower(f.acme.project.ID) + "/"
	crossing := []string{
		"/dav/globex/calendars/" + bob + "/",
		"/dav/globex/calendars/" + gbob + "/",
		"/dav/globex/principals/" + gbob + "/",
		"/dav/globex/calendars/" + gbob + "/assigned/",
		"/dav/globex/calendars/" + gbob + "/assigned/" + name(theirs) + ".ics",
		"/dav/acme/calendars/" + gbob + "/",
		"/dav/acme/principals/" + gbob + "/",
		"/dav/acme/calendars/" + strings.ToLower(carol.ID) + "/",
		"/dav/acme/principals/" + strings.ToLower(carol.ID) + "/",
		"/dav/acme/calendars/" + strings.ToLower(carol.ID) + "/assigned/" + name(mine) + ".ics",
		ownHome + "p-" + strings.ToLower(f.globex.project.ID) + "/",
		ownHome + "p-" + strings.ToLower(f.globex.project.ID) + "/" + name(theirs) + ".ics",
		ownHome + "assigned/" + name(theirs) + ".ics",
		ownInfra + name(theirs) + ".ics",
	}
	nonsense := f.serve(ctx, "PROPFIND", "/dav/no/such/thing/", map[string]string{"Depth": "0"}, "")
	if nonsense.status != http.StatusNotFound {
		t.Fatalf("nonsense answered %d", nonsense.status)
	}
	for _, path := range crossing {
		for _, method := range []string{"PROPFIND", "GET", "REPORT"} {
			body := ""
			if method == "REPORT" {
				body = `<d:sync-collection xmlns:d="DAV:"><d:sync-token/><d:sync-level>1</d:sync-level>` +
					`<d:prop><d:getetag/></d:prop></d:sync-collection>`
			}
			r := f.serve(ctx, method, path, map[string]string{"Depth": "0"}, body)
			if r.status != nonsense.status || r.body != nonsense.body ||
				r.header.Get("Content-Type") != nonsense.header.Get("Content-Type") || r.header.Get("ETag") != "" {
				t.Errorf("%s %s answered %d %q, unlike nonsense", method, path, r.status, r.body)
			}
		}
	}

	for _, tc := range []struct {
		ctx  context.Context
		path string
	}{
		{ctx, ownHome},
		{ctx, ownInfra},
		{ctx, ownHome + "assigned/" + name(mine) + ".ics"},
		{davCtx(f.globex.scope.TenantID, globex), "/dav/globex/calendars/" + gbob + "/assigned/" + name(theirs) + ".ics"},
	} {
		if r := f.serve(tc.ctx, "PROPFIND", tc.path, map[string]string{"Depth": "0"}, ""); r.status != http.StatusMultiStatus {
			t.Errorf("own path %s answered %d", tc.path, r.status)
		}
	}
	if r := f.serve(davCtx(f.globex.scope.TenantID, globex), "PROPFIND", ownHome, map[string]string{"Depth": "0"}, ""); r.status != http.StatusNotFound {
		t.Errorf("globex's password reached acme's home: %d", r.status)
	}
}
