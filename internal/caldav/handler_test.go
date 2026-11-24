// SPDX-License-Identifier: AGPL-3.0-or-later

package caldav

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
)

func TestARequestWithoutAPasswordIsChallenged(t *testing.T) {
	f := newFixture(t)
	f.task(f.acme, "secret plans", f.acme.bob.ID)
	p := f.password(f.acme.bob, core.DAVCollectionsAll)
	gone := *p
	gone.ID = "01kdvdna00nosuchpassword00"
	for _, tc := range []struct {
		name string
		ctx  context.Context
	}{
		{"no actor", context.Background()},
		{"a password that signs nobody in", davCtx(f.acme.scope.TenantID, &gone)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := f.serve(tc.ctx, "PROPFIND", "/dav/acme/calendars/"+strings.ToLower(f.acme.bob.ID)+"/", depth1(), "")
			if r.status != http.StatusUnauthorized || r.header.Get("WWW-Authenticate") != auth.DAVChallenge {
				t.Fatalf("answered %d with challenge %q", r.status, r.header.Get("WWW-Authenticate"))
			}
			for _, leak := range []string{"acme", "bob", "Infra", "secret", strings.ToLower(f.acme.bob.ID)} {
				if strings.Contains(strings.ToLower(r.body), strings.ToLower(leak)) {
					t.Errorf("the 401 body names %q: %s", leak, r.body)
				}
			}
		})
	}
}

func TestDisabledCalDAVIsNotThere(t *testing.T) {
	f := newFixture(t)
	p := f.password(f.acme.bob, "")
	off := New(service.New(f.st, service.WithClock(f.clk), service.WithCalDAV(false, -1, 0)), nil)
	r := httptest.NewRequestWithContext(davCtx(f.acme.scope.TenantID, p), "PROPFIND", "/dav/", nil)
	r.Header.Set("Depth", "0")
	w := httptest.NewRecorder()
	off.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("disabled CalDAV answered %d", w.Code)
	}
}

func TestOptionsAnnouncesCalendarAccess(t *testing.T) {
	f := newFixture(t)
	writer := f.serve(davCtx(f.acme.scope.TenantID, f.password(f.acme.bob, "")), http.MethodOptions,
		home(f.acme, f.acme.bob), nil, "")
	for _, m := range []string{"PUT", "DELETE"} {
		if !strings.Contains(writer.header.Get("Allow"), m) {
			t.Errorf("a principal that writes is not allowed %s: %q", m, writer.header.Get("Allow"))
		}
	}
	ctx := davCtx(f.acme.scope.TenantID, f.readOnly(f.acme.bob, ""))
	r := f.serve(ctx, http.MethodOptions, home(f.acme, f.acme.bob), nil, "")
	if r.status != http.StatusOK || r.header.Get("DAV") != "1, 3, access-control, calendar-access" {
		t.Fatalf("answered %d with DAV %q", r.status, r.header.Get("DAV"))
	}
	allow := r.header.Get("Allow")
	for _, m := range []string{"PROPFIND", "REPORT", "GET"} {
		if !strings.Contains(allow, m) {
			t.Errorf("Allow %q lacks %s", allow, m)
		}
	}
	for _, m := range []string{"PUT", "DELETE"} {
		if strings.Contains(allow, m) {
			t.Errorf("a read-only principal is allowed %s: %q", m, allow)
		}
	}
}

func TestGetAnObjectHonoursIfNoneMatch(t *testing.T) {
	f := newFixture(t)
	task := f.task(f.acme, "Rotate keys", f.acme.bob.ID)
	ctx := davCtx(f.acme.scope.TenantID, f.password(f.acme.bob, ""))
	obj := home(f.acme, f.acme.bob) + "assigned/" + name(task) + ".ics"

	first := f.serve(ctx, http.MethodGet, obj, nil, "")
	etag := first.header.Get("ETag")
	if first.status != http.StatusOK || !strings.HasPrefix(etag, `"`) ||
		first.header.Get("Content-Type") != "text/calendar; charset=utf-8" ||
		!strings.Contains(first.body, "UID:task-"+name(task)+"@tix") {
		t.Fatalf("GET answered %d etag %q type %q: %s", first.status, etag, first.header.Get("Content-Type"), first.body)
	}
	for _, inm := range []string{etag, `"other", ` + etag, "W/" + etag, "*"} {
		r := f.serve(ctx, http.MethodGet, obj, map[string]string{"If-None-Match": inm}, "")
		if r.status != http.StatusNotModified || r.body != "" || r.header.Get("ETag") != etag {
			t.Errorf("If-None-Match %s answered %d %q", inm, r.status, r.body)
		}
	}
	if r := f.serve(ctx, http.MethodGet, obj, map[string]string{"If-None-Match": `"stale"`}, ""); r.status != http.StatusOK {
		t.Errorf("a stale If-None-Match answered %d", r.status)
	}
	head := f.serve(ctx, http.MethodHead, obj, nil, "")
	if head.status != http.StatusOK || head.body != "" || head.header.Get("ETag") != etag {
		t.Errorf("HEAD answered %d %q", head.status, head.body)
	}
	if r := f.serve(ctx, http.MethodGet, home(f.acme, f.acme.bob)+"assigned/nope.ics", nil, ""); r.status != http.StatusNotFound {
		t.Errorf("GET of a name the collection does not hold answered %d", r.status)
	}
	if r := f.serve(ctx, http.MethodGet, home(f.acme, f.acme.bob)+"assigned/", nil, ""); r.status != http.StatusMethodNotAllowed {
		t.Errorf("GET of a collection answered %d", r.status)
	}
}

func TestEveryWriteIsRefusedWhileReadOnly(t *testing.T) {
	f := newFixture(t)
	task := f.task(f.acme, "Rotate keys", f.acme.bob.ID)
	ctx := davCtx(f.acme.scope.TenantID, f.readOnly(f.acme.bob, ""))
	coll := home(f.acme, f.acme.bob) + "assigned/"
	obj := coll + name(task) + ".ics"
	put := vtodo("UID:task-"+name(task)+"@tix", "SUMMARY:Changed on the phone", "STATUS:COMPLETED")
	tests := []struct {
		method, path, body string
		privilege          string
	}{
		{http.MethodPut, obj, put, "<d:write-content/>"},
		{http.MethodPut, coll + "new.ics", put, "<d:write-content/>"},
		{http.MethodDelete, obj, "", "<d:unbind/>"},
		{"MKCOL", home(f.acme, f.acme.bob) + "x/", "", "<d:bind/>"},
		{"MKCALENDAR", home(f.acme, f.acme.bob) + "new/", "", "<d:bind/>"},
		{"MOVE", obj, "", "<d:unbind/>"},
		{"COPY", obj, "", "<d:bind/>"},
		{"ACL", obj, "", "<d:write-acl/>"},
		{"LOCK", obj, "", ""},
		{"UNLOCK", obj, "", ""},
		{http.MethodPost, coll, "", ""},
	}
	for _, tc := range tests {
		r := f.serve(ctx, tc.method, tc.path, map[string]string{"Content-Type": "text/calendar"}, tc.body)
		if r.status != http.StatusForbidden || !strings.Contains(r.body, tc.privilege) {
			t.Errorf("%s %s answered %d %s", tc.method, tc.path, r.status, r.body)
		}
		if tc.privilege != "" && !strings.Contains(r.body, "<d:need-privileges>") {
			t.Errorf("%s names no need-privileges: %s", tc.method, r.body)
		}
	}
	after, err := f.svc.GetTask(adminCtx(f.acme), core.TaskRef{ID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	if after.Title != task.Title || after.Version != task.Version || after.Status != task.Status {
		t.Errorf("the task changed: %+v", after)
	}
	if r := f.serve(ctx, http.MethodPut, home(f.acme, f.acme.bob)+"nope/x.ics", nil, put); r.status != http.StatusNotFound {
		t.Errorf("PUT into a missing collection answered %d", r.status)
	}
	if r := f.serve(ctx, "BREW", obj, nil, ""); r.status != http.StatusMethodNotAllowed || r.header.Get("Allow") == "" {
		t.Errorf("an unknown method answered %d", r.status)
	}
}

func TestProppatchAnswersForbiddenPerProperty(t *testing.T) {
	f := newFixture(t)
	ctx := davCtx(f.acme.scope.TenantID, f.password(f.acme.bob, ""))
	coll := home(f.acme, f.acme.bob) + "assigned/"
	body := `<d:propertyupdate xmlns:d="DAV:" xmlns:ical="http://apple.com/ns/ical/"><d:set><d:prop>` +
		`<ical:calendar-color>#FF0000FF</ical:calendar-color><ical:calendar-order>3</ical:calendar-order>` +
		`</d:prop></d:set><d:remove><d:prop><d:displayname/></d:prop></d:remove></d:propertyupdate>`
	r := parseMS(t, f.serve(ctx, "PROPPATCH", coll, nil, body)).response(t, coll)
	for _, n := range []string{"calendar-color", "calendar-order"} {
		if _, status, ok := r.prop(appleName(n)); !ok || status != "HTTP/1.1 403 Forbidden" {
			t.Errorf("%s answered %v %q", n, ok, status)
		}
	}
	if _, status, _ := r.prop(davName("displayname")); status != "HTTP/1.1 403 Forbidden" {
		t.Errorf("displayname removal answered %q", status)
	}
	d := parseMS(t, f.serve(ctx, "PROPFIND", coll, depth0(), propfindBody(`<d:displayname/>`)))
	if got := d.response(t, coll).okProp(t, davName("displayname")).Text; got != "Assigned to me" {
		t.Errorf("the displayname changed to %q", got)
	}
	if r := f.serve(ctx, "PROPPATCH", home(f.acme, f.acme.bob)+"nope/", nil, body); r.status != http.StatusNotFound {
		t.Errorf("PROPPATCH of a missing collection answered %d", r.status)
	}
}

func TestTheWellKnownAddressRedirects(t *testing.T) {
	for _, tc := range []struct {
		origin func(*http.Request) string
		want   string
	}{
		{nil, "/dav/"},
		{func(*http.Request) string { return "https://tix.example/" }, "https://tix.example/dav/"},
	} {
		h := New(nil, tc.origin).WellKnown()
		for _, method := range []string{"PROPFIND", http.MethodGet, http.MethodOptions} {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(method, core.DAVWellKnownPath, nil))
			if w.Code != http.StatusMovedPermanently || w.Header().Get("Location") != tc.want {
				t.Errorf("%s answered %d to %q", method, w.Code, w.Header().Get("Location"))
			}
		}
	}
}

func TestNoAnswerCarriesCORSHeaders(t *testing.T) {
	f := newFixture(t)
	task := f.task(f.acme, "Rotate keys", f.acme.bob.ID)
	ctx := davCtx(f.acme.scope.TenantID, f.password(f.acme.bob, ""))
	obj := home(f.acme, f.acme.bob) + "assigned/" + name(task) + ".ics"
	for _, m := range []string{http.MethodOptions, http.MethodGet, "PROPFIND", http.MethodPut} {
		r := f.serve(ctx, m, obj, map[string]string{"Origin": "https://evil.example", "Depth": "0",
			"Access-Control-Request-Method": "PUT"}, "")
		for k := range r.header {
			if strings.HasPrefix(strings.ToLower(k), "access-control-") {
				t.Errorf("%s answered %s", m, k)
			}
		}
	}
}

func TestAnExpiredRequestContextIsNotAnswered(t *testing.T) {
	f := newFixture(t)
	f.task(f.acme, "Rotate keys", f.acme.bob.ID)
	ctx, cancel := context.WithDeadline(davCtx(f.acme.scope.TenantID, f.password(f.acme.bob, "")), time.Unix(0, 0))
	defer cancel()
	r := f.serve(ctx, "PROPFIND", home(f.acme, f.acme.bob)+"assigned/", depth1(), "")
	if r.status == http.StatusMultiStatus || r.status == http.StatusOK {
		t.Fatalf("a request past its deadline answered %d", r.status)
	}
}
