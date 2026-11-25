// SPDX-License-Identifier: AGPL-3.0-or-later

package caldav

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
)

func depth0() map[string]string { return map[string]string{"Depth": "0"} }
func depth1() map[string]string { return map[string]string{"Depth": "1"} }

func TestDiscoveryNamesThePrincipalAndTheCalendarHome(t *testing.T) {
	f := newFixture(t)
	ctx := davCtx(f.acme.scope.TenantID, f.password(f.acme.bob, ""))
	bob := strings.ToLower(f.acme.bob.ID)
	principal := "/dav/acme/principals/" + bob + "/"
	homePath := "/dav/acme/calendars/" + bob + "/"

	root := parseMS(t, f.serve(ctx, "PROPFIND", "/dav/", depth0(),
		propfindBody(`<d:current-user-principal/><d:resourcetype/>`))).response(t, "/dav/")
	if got := root.okProp(t, davName("current-user-principal")).Inner; got != "<d:href>"+principal+"</d:href>" {
		t.Errorf("current-user-principal %q", got)
	}
	if got := root.okProp(t, davName("resourcetype")).Inner; got != "<d:collection/>" {
		t.Errorf("root resourcetype %q", got)
	}

	pr := parseMS(t, f.serve(ctx, "PROPFIND", principal, depth0(), propfindBody(
		`<c:calendar-home-set/><c:calendar-user-address-set/><d:displayname/><d:resourcetype/><d:principal-URL/>`))).
		response(t, principal)
	if got := pr.okProp(t, calName("calendar-home-set")).Inner; got != "<d:href>"+homePath+"</d:href>" {
		t.Errorf("calendar-home-set %q", got)
	}
	if got := pr.okProp(t, calName("calendar-user-address-set")).Inner; got !=
		"<d:href>mailto:bob@example.com</d:href><d:href>"+principal+"</d:href>" {
		t.Errorf("calendar-user-address-set %q", got)
	}
	if got := pr.okProp(t, davName("displayname")).Text; got != "Bob" {
		t.Errorf("displayname %q", got)
	}
	if got := pr.okProp(t, davName("resourcetype")).Inner; got != "<d:principal/>" {
		t.Errorf("principal resourcetype %q", got)
	}
	if got := pr.okProp(t, davName("principal-URL")).Inner; got != "<d:href>"+principal+"</d:href>" {
		t.Errorf("principal-URL %q", got)
	}
}

func TestTheHomeListsTheCollectionsThePasswordChose(t *testing.T) {
	f := newFixture(t)
	bob := strings.ToLower(f.acme.bob.ID)
	homePath := "/dav/acme/calendars/" + bob + "/"
	infra := homePath + "p-" + strings.ToLower(f.acme.project.ID) + "/"
	web := homePath + "p-" + strings.ToLower(f.acme.web.ID) + "/"
	for _, tc := range []struct {
		choice core.DAVCollections
		want   []string
	}{
		{"", []string{homePath, homePath + "assigned/"}},
		{core.DAVCollectionsProjects, []string{homePath, infra, web}},
		{core.DAVCollectionsAll, []string{homePath, homePath + "assigned/", infra, web}},
	} {
		ctx := davCtx(f.acme.scope.TenantID, f.password(f.acme.bob, tc.choice))
		d := parseMS(t, f.serve(ctx, "PROPFIND", homePath, depth1(), propfindBody(
			`<d:resourcetype/><c:supported-calendar-component-set/><d:displayname/>`)))
		if got := d.hrefs(); !slices.Equal(got, tc.want) {
			t.Errorf("%q lists %v, want %v", tc.choice, got, tc.want)
			continue
		}
		for _, href := range tc.want[1:] {
			r := d.response(t, href)
			if got := r.okProp(t, davName("resourcetype")).Inner; got != "<d:collection/><c:calendar/>" {
				t.Errorf("%s resourcetype %q", href, got)
			}
			if got := r.okProp(t, calName("supported-calendar-component-set")).Inner; got != `<c:comp name="VTODO"/>` {
				t.Errorf("%s component set %q", href, got)
			}
		}
	}
}

func TestACollectionAnswersItsPropertiesAndMembers(t *testing.T) {
	f := newFixture(t, service.WithCalDAV(true, 14*24*time.Hour, 2))
	open := f.task(f.acme, "open", f.acme.bob.ID)
	second := f.task(f.acme, "second", f.acme.bob.ID)
	f.task(f.acme, "third", f.acme.bob.ID)
	ctx := davCtx(f.acme.scope.TenantID, f.password(f.acme.bob, core.DAVCollectionsAll))
	coll := home(f.acme, f.acme.bob) + "p-" + strings.ToLower(f.acme.project.ID) + "/"
	d := parseMS(t, f.serve(ctx, "PROPFIND", strings.TrimSuffix(coll, "/"), depth1(), propfindBody(
		`<d:resourcetype/><d:displayname/><c:calendar-description/><ical:calendar-color/>`+
			`<d:sync-token/><cs:getctag/><d:current-user-privilege-set/><d:supported-report-set/>`+
			`<d:owner/><d:getetag/><d:getcontenttype/>`)))
	c := d.response(t, coll)
	token := c.okProp(t, davName("sync-token")).Text
	if !strings.HasPrefix(token, "https://tix.red/ns/sync/") || c.okProp(t, csName("getctag")).Text != token {
		t.Errorf("sync-token %q and getctag %q", token, c.okProp(t, csName("getctag")).Text)
	}
	if got := c.okProp(t, davName("current-user-privilege-set")).Inner; got != "<d:privilege><d:read/></d:privilege>" {
		t.Errorf("privileges %q, want DAV:read alone", got)
	}
	if got := c.okProp(t, appleName("calendar-color")).Text; got != "#336699ff" {
		t.Errorf("colour %q", got)
	}
	if got := c.okProp(t, davName("displayname")).Text; got != "Infra" {
		t.Errorf("displayname %q", got)
	}
	if got := c.okProp(t, calName("calendar-description")).Text; !strings.Contains(got, "1 more left out") {
		t.Errorf("description %q does not say a task was left out", got)
	}
	reports := c.okProp(t, davName("supported-report-set")).Inner
	for _, r := range []string{"<c:calendar-query/>", "<c:calendar-multiget/>", "<d:sync-collection/>"} {
		if !strings.Contains(reports, r) {
			t.Errorf("supported-report-set %q lacks %s", reports, r)
		}
	}
	if _, status, _ := c.prop(davName("getetag")); status != "HTTP/1.1 404 Not Found" {
		t.Errorf("a collection's getetag is in %q", status)
	}
	want := []string{coll, coll + name(open) + ".ics", coll + name(second) + ".ics"}
	slices.Sort(want[1:])
	if got := d.hrefs(); !slices.Equal(got, want) {
		t.Fatalf("hrefs %v, want %v", got, want)
	}
	m := d.response(t, want[1])
	if !strings.HasPrefix(m.okProp(t, davName("getetag")).Text, `"`) {
		t.Errorf("member etag %q is not a strong tag", m.okProp(t, davName("getetag")).Text)
	}
	if got := m.okProp(t, davName("getcontenttype")).Text; got != "text/calendar; charset=utf-8; component=VTODO" {
		t.Errorf("member content type %q", got)
	}
	if got := m.okProp(t, davName("resourcetype")).Inner; got != "" {
		t.Errorf("member resourcetype %q", got)
	}
}

func TestAnObjectAnswersCalendarDataOnlyWhenAsked(t *testing.T) {
	f := newFixture(t)
	task := f.task(f.acme, "Rotate keys", f.acme.bob.ID)
	ctx := davCtx(f.acme.scope.TenantID, f.password(f.acme.bob, ""))
	obj := home(f.acme, f.acme.bob) + "assigned/" + name(task) + ".ics"

	asked := parseMS(t, f.serve(ctx, "PROPFIND", obj, depth0(), propfindBody(`<d:getetag/><c:calendar-data/>`))).response(t, obj)
	data := asked.okProp(t, calName("calendar-data")).Text
	if !strings.Contains(data, "SUMMARY:Rotate keys\r\n") || !strings.Contains(data, "BEGIN:VTODO") {
		t.Errorf("calendar-data %q", data)
	}
	all := parseMS(t, f.serve(ctx, "PROPFIND", obj, depth0(), "")).response(t, obj)
	if _, _, ok := all.prop(calName("calendar-data")); ok {
		t.Error("allprop answered calendar-data")
	}
	all.okProp(t, davName("getetag"))
	names := parseMS(t, f.serve(ctx, "PROPFIND", obj, depth0(),
		`<d:propfind xmlns:d="DAV:"><d:propname/></d:propfind>`)).response(t, obj)
	if p := names.okProp(t, calName("calendar-data")); p.Inner != "" {
		t.Errorf("propname answered a value %q", p.Inner)
	}
}

func TestAnUnknownPropertyIsInItsOwnNotFoundPropstat(t *testing.T) {
	f := newFixture(t)
	ctx := davCtx(f.acme.scope.TenantID, f.password(f.acme.bob, ""))
	coll := home(f.acme, f.acme.bob) + "assigned/"
	r := parseMS(t, f.serve(ctx, "PROPFIND", coll, depth0(),
		propfindBody(`<d:displayname/><cs:push-transports/><x:odd xmlns:x="urn:x"/>`))).response(t, coll)
	if len(r.Propstats) != 2 {
		t.Fatalf("%d propstats, want a 200 and a 404: %+v", len(r.Propstats), r.Propstats)
	}
	if _, status, _ := r.prop(davName("displayname")); status != "HTTP/1.1 200 OK" {
		t.Errorf("displayname in %q", status)
	}
	for _, n := range []string{"push-transports", "odd"} {
		found := false
		for _, p := range r.Propstats[1].Prop.Props {
			found = found || p.XMLName.Local == n
		}
		if !found || r.Propstats[1].Status != "HTTP/1.1 404 Not Found" {
			t.Errorf("%s is not in the 404 propstat: %+v", n, r.Propstats[1])
		}
	}
}

func TestPropfindRefusesAnInfiniteDepthAndABadBody(t *testing.T) {
	f := newFixture(t)
	ctx := davCtx(f.acme.scope.TenantID, f.password(f.acme.bob, ""))
	h := home(f.acme, f.acme.bob)
	tests := []struct {
		name   string
		header map[string]string
		body   string
		status int
		want   string
	}{
		{"depth infinity", map[string]string{"Depth": "infinity"}, "", http.StatusForbidden, "<d:propfind-finite-depth/>"},
		{"no depth", nil, "", http.StatusForbidden, "<d:propfind-finite-depth/>"},
		{"depth two", map[string]string{"Depth": "2"}, "", http.StatusBadRequest, ""},
		{"an entity declaration", depth0(), `<?xml version="1.0"?><!DOCTYPE d [<!ENTITY x "y">]>` +
			`<d:propfind xmlns:d="DAV:"><d:prop><d:displayname>&x;</d:displayname></d:prop></d:propfind>`,
			http.StatusBadRequest, ""},
		{"another root", depth0(), `<d:propertyupdate xmlns:d="DAV:"/>`, http.StatusBadRequest, ""},
		{"too large", depth0(), `<d:propfind xmlns:d="DAV:">` + strings.Repeat(" ", maxBodyBytes) + `</d:propfind>`,
			http.StatusBadRequest, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := f.serve(ctx, "PROPFIND", h, tc.header, tc.body)
			if r.status != tc.status || !strings.Contains(r.body, tc.want) {
				t.Fatalf("answered %d %s", r.status, r.body)
			}
			if strings.Contains(r.body, "multistatus") {
				t.Errorf("a refused request was evaluated: %s", r.body)
			}
		})
	}
}
