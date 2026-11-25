// SPDX-License-Identifier: AGPL-3.0-or-later

package caldav

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// todoFor is an object for a task tix made, with the given lines.
func todoFor(task *core.Task, lines ...string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VTODO\r\nUID:task-" + name(task) + "@tix\r\n" +
		strings.Join(lines, "\r\n") + "\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"
}

// etagOf is an object's current entity tag.
func (f *fixture) etagOf(ctx context.Context, path string) string {
	f.t.Helper()
	r := f.serve(ctx, http.MethodGet, path, nil, "")
	if r.status != http.StatusOK {
		f.t.Fatalf("GET %s answered %d", path, r.status)
	}
	return r.header.Get("ETag")
}

func TestEveryWriteStatusCodePath(t *testing.T) {
	f := newFixture(t)
	ctx := davCtx(f.acme.scope.TenantID, f.password(f.acme.bob, core.DAVCollectionsAll))
	task := f.task(f.acme, "Rotate keys", f.acme.bob.ID)
	coll := home(f.acme, f.acme.bob) + "assigned/"
	infra := home(f.acme, f.acme.bob) + "p-" + strings.ToLower(f.acme.project.ID) + "/"
	obj := coll + name(task) + ".ics"
	ical := map[string]string{"Content-Type": "text/calendar; charset=utf-8"}
	with := func(k, v string) map[string]string {
		return map[string]string{"Content-Type": "text/calendar; charset=utf-8", k: v}
	}
	claimed := f.task(f.acme, "Claimed by an agent", f.acme.bob.ID)
	if _, err := f.svc.ClaimTask(adminCtx(f.acme), core.TaskRef{ID: claimed.ID}, core.ClaimInput{}); err != nil {
		t.Fatal(err)
	}
	claimedObj := coll + name(claimed) + ".ics"
	tests := []struct {
		name   string
		method string
		path   string
		header func() map[string]string
		body   string
		status int
		has    string
	}{
		{"an update without If-Match", http.MethodPut, obj, func() map[string]string { return ical },
			todoFor(task, "SUMMARY:x"), http.StatusPreconditionRequired, ""},
		{"a stale If-Match", http.MethodPut, obj, func() map[string]string { return with("If-Match", `"stale"`) },
			todoFor(task, "SUMMARY:x"), http.StatusPreconditionFailed, ""},
		{"If-None-Match on an object that exists", http.MethodPut, obj,
			func() map[string]string { return with("If-None-Match", "*") }, todoFor(task, "SUMMARY:x"),
			http.StatusPreconditionFailed, ""},
		{"another media type", http.MethodPut, obj,
			func() map[string]string { return map[string]string{"Content-Type": "application/json"} }, `{}`,
			http.StatusUnsupportedMediaType, ""},
		{"an event", http.MethodPut, coll + "event.ics", func() map[string]string { return with("If-None-Match", "*") },
			"BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:e\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n", http.StatusForbidden,
			"<c:valid-calendar-object-resource/>"},
		{"a body over the bound", http.MethodPut, coll + "big.ics", func() map[string]string { return ical },
			strings.Repeat("x", 256<<10+1), http.StatusForbidden, "<c:valid-calendar-object-resource/>"},
		{"a UID in use", http.MethodPut, coll + "copy.ics", func() map[string]string { return with("If-None-Match", "*") },
			todoFor(task, "SUMMARY:copy"), http.StatusForbidden, "<c:no-uid-conflict/>"},
		{"a move the task's claim refuses", http.MethodPut, claimedObj,
			func() map[string]string { return with("If-Match", f.etagOf(ctx, claimedObj)) },
			todoFor(claimed, "SUMMARY:Claimed by an agent", "STATUS:COMPLETED"), http.StatusForbidden, "claim"},
		{"an update", http.MethodPut, obj, func() map[string]string { return with("If-Match", f.etagOf(ctx, obj)) },
			todoFor(task, "SUMMARY:Rotate keys today"), http.StatusNoContent, ""},
		{"a create in a tenant with no default project", http.MethodPut, coll + "NEW-0.ics",
			func() map[string]string { return with("If-None-Match", "*") },
			"BEGIN:VCALENDAR\r\nBEGIN:VTODO\r\nUID:NEW-0\r\nSUMMARY:Call the bank\r\nEND:VTODO\r\nEND:VCALENDAR\r\n",
			http.StatusForbidden, "project's list"},
		{"a create", http.MethodPut, infra + "NEW-1.ics", func() map[string]string { return with("If-None-Match", "*") },
			"BEGIN:VCALENDAR\r\nBEGIN:VTODO\r\nUID:NEW-1\r\nSUMMARY:Call the bank\r\nEND:VTODO\r\nEND:VCALENDAR\r\n",
			http.StatusCreated, ""},
		{"a write into another tenant", http.MethodPut, "/dav/globex/calendars/" + strings.ToLower(f.acme.bob.ID) +
			"/assigned/" + name(task) + ".ics", func() map[string]string { return with("If-Match", "*") },
			todoFor(task, "SUMMARY:x"), http.StatusNotFound, ""},
		{"a write into a missing collection", http.MethodPut, home(f.acme, f.acme.bob) + "nope/x.ics",
			func() map[string]string { return ical }, todoFor(task, "SUMMARY:x"), http.StatusNotFound, ""},
		{"a PUT on a collection", http.MethodPut, coll, func() map[string]string { return ical },
			todoFor(task, "SUMMARY:x"), http.StatusForbidden, "<d:write-content/>"},
		{"a delete of a missing object", http.MethodDelete, coll + "nope.ics", func() map[string]string { return nil },
			"", http.StatusNotFound, ""},
		{"a delete", http.MethodDelete, obj, func() map[string]string { return nil }, "", http.StatusNoContent, ""},
	}
	for _, tc := range tests {
		r := f.serve(ctx, tc.method, tc.path, tc.header(), tc.body)
		if r.status != tc.status || !strings.Contains(r.body, tc.has) {
			t.Errorf("%s: answered %d %q, want %d holding %q", tc.name, r.status, r.body, tc.status, tc.has)
		}
		if r.header.Get("ETag") != "" {
			t.Errorf("%s: answered an ETag", tc.name)
		}
	}
	created := f.serve(ctx, http.MethodGet, infra+"NEW-1.ics", nil, "")
	if created.status != http.StatusOK || !strings.Contains(created.body, "UID:NEW-1\r\n") {
		t.Errorf("the created object answered %d:\n%s", created.status, created.body)
	}
	after, err := f.svc.GetTask(adminCtx(f.acme), core.TaskRef{ID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	if after.Title != "Rotate keys today" || after.Status != "cancelled" || after.DeletedAt != nil {
		t.Errorf("the task after an update and a delete: %q %q %v", after.Title, after.Status, after.DeletedAt)
	}
	if got, _ := f.svc.GetTask(adminCtx(f.acme), core.TaskRef{ID: claimed.ID}); got.Status != claimed.Status {
		t.Errorf("the claimed task moved to %q", got.Status)
	}
}

// A person's password reaches only their own tenant: the same path in a
// second tenant they belong to, with the first tenant's password, is not
// found, and nothing there changes.
func TestAWriteNeverReachesAnotherTenant(t *testing.T) {
	f := newFixture(t)
	acme := davCtx(f.acme.scope.TenantID, f.password(f.acme.bob, core.DAVCollectionsAll))
	theirs := f.task(f.globex, "Globex's task", f.globex.bob.ID)
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		header := map[string]string{"If-Match": "*", "Content-Type": "text/calendar"}
		body := todoFor(theirs, "SUMMARY:changed from acme", "STATUS:COMPLETED")
		for _, path := range []string{
			home(f.globex, f.globex.bob) + "assigned/" + name(theirs) + ".ics",
			"/dav/globex/calendars/" + strings.ToLower(f.acme.bob.ID) + "/assigned/" + name(theirs) + ".ics",
		} {
			if r := f.serve(acme, method, path, header, body); r.status != http.StatusNotFound {
				t.Errorf("%s %s answered %d", method, path, r.status)
			}
		}
		nonsense := f.serve(acme, method, home(f.acme, f.acme.bob)+"assigned/nonsense.ics", header, body)
		r := f.serve(acme, method, home(f.acme, f.acme.bob)+"assigned/"+name(theirs)+".ics", header, body)
		if r.status != nonsense.status || r.body != nonsense.body {
			t.Errorf("%s of globex's task name in acme answered %d, a nonsense name %d", method, r.status, nonsense.status)
		}
	}
	got, err := f.svc.GetTask(adminCtx(f.globex), core.TaskRef{ID: theirs.ID})
	if err != nil || got.Title != theirs.Title || got.Version != theirs.Version {
		t.Errorf("globex's task changed: %+v %v", got, err)
	}
	r := f.serve(acme, http.MethodPut, home(f.acme, f.acme.bob)+"p-"+strings.ToLower(f.acme.project.ID)+"/x.ics",
		map[string]string{"If-None-Match": "*"}, todoFor(theirs, "SUMMARY:same UID as globex"))
	if r.status != http.StatusCreated {
		t.Errorf("a UID another tenant's task holds answered %d %s", r.status, r.body)
	}
}

func TestADemotionTurnsWritesOffOnTheNextRequest(t *testing.T) {
	f := newFixture(t)
	ctx := davCtx(f.acme.scope.TenantID, f.password(f.acme.bob, core.DAVCollectionsAll))
	task := f.task(f.acme, "Rotate keys", f.acme.bob.ID)
	obj := home(f.acme, f.acme.bob) + "assigned/" + name(task) + ".ics"
	etag := f.etagOf(ctx, obj)
	if _, err := f.svc.UpdateUser(adminCtx(f.acme), f.acme.bob.ID, core.UpdateUserInput{Role: new(core.RoleViewer)}); err != nil {
		t.Fatal(err)
	}
	r := f.serve(ctx, http.MethodPut, obj, map[string]string{"If-Match": etag}, todoFor(task, "SUMMARY:x"))
	if r.status != http.StatusForbidden || !strings.Contains(r.body, "<d:need-privileges>") {
		t.Errorf("a viewer's PUT answered %d %s", r.status, r.body)
	}
	privs := parseMS(t, f.serve(ctx, "PROPFIND", home(f.acme, f.acme.bob)+"assigned/", depth0(),
		propfindBody(`<d:current-user-privilege-set/>`))).response(t, home(f.acme, f.acme.bob)+"assigned/")
	if got := privs.okProp(t, davName("current-user-privilege-set")).Inner; got != "<d:privilege><d:read/></d:privilege>" {
		t.Errorf("a viewer's privileges %q", got)
	}
	if got, _ := f.svc.GetTask(adminCtx(f.acme), core.TaskRef{ID: task.ID}); got.Title != task.Title {
		t.Errorf("the title changed to %q", got.Title)
	}
}
