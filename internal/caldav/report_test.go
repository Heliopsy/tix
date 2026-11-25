// SPDX-License-Identifier: AGPL-3.0-or-later

package caldav

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

const reportNS = `xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"`

func syncBody(token string, limit int) string {
	l := ""
	if limit > 0 {
		l = "<d:limit><d:nresults>" + strconv.Itoa(limit) + "</d:nresults></d:limit>"
	}
	return `<d:sync-collection ` + reportNS + `><d:sync-token>` + token + `</d:sync-token><d:sync-level>1</d:sync-level>` +
		l + `<d:prop><d:getetag/></d:prop></d:sync-collection>`
}

func multigetBody(hrefs ...string) string {
	var b strings.Builder
	b.WriteString(`<c:calendar-multiget ` + reportNS + `><d:prop><d:getetag/><c:calendar-data/></d:prop>`)
	for _, h := range hrefs {
		b.WriteString("<d:href>" + h + "</d:href>")
	}
	b.WriteString(`</c:calendar-multiget>`)
	return b.String()
}

// newReportFixture is bob's Assigned to me collection with two open tasks
// and a finished one.
func newReportFixture(t *testing.T) (*fixture, string, []*core.Task) {
	t.Helper()
	f := newFixture(t)
	open := f.task(f.acme, "Rotate keys", f.acme.bob.ID)
	also := f.task(f.acme, "Renew certificate", f.acme.bob.ID)
	done := f.task(f.acme, "Order laptops", f.acme.bob.ID)
	f.move(f.acme, done, "done")
	return f, home(f.acme, f.acme.bob) + "assigned/", []*core.Task{open, also, done}
}

func TestCalendarQueryAnswersIncompleteTodos(t *testing.T) {
	f, coll, tasks := newReportFixture(t)
	ctx := davCtx(f.acme.scope.TenantID, f.password(f.acme.bob, ""))
	body := `<c:calendar-query ` + reportNS + `><d:prop><d:getetag/><c:calendar-data/></d:prop><c:filter>` +
		todoFilter(`<c:prop-filter name="COMPLETED"><c:is-not-defined/></c:prop-filter>`) + `</c:filter></c:calendar-query>`
	d := parseMS(t, f.serve(ctx, "REPORT", coll, depth1(), body))
	want := []string{coll + name(tasks[0]) + ".ics", coll + name(tasks[1]) + ".ics"}
	got := d.hrefs()
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("matched %v, want the two open tasks %v", got, want)
	}
	for _, href := range want {
		data := d.response(t, href).okProp(t, calName("calendar-data")).Text
		if strings.Contains(data, "COMPLETED") || !strings.Contains(data, "BEGIN:VTODO") {
			t.Errorf("%s data %q", href, data)
		}
	}
	one := parseMS(t, f.serve(ctx, "REPORT", want[0], depth0(), body))
	if got := one.hrefs(); !slices.Equal(got, want[:1]) {
		t.Errorf("a query on one object answered %v", got)
	}
	bad := strings.Replace(body, `<c:is-not-defined/>`, `<c:text-match collation="i;unicode-casemap">x</c:text-match>`, 1)
	if r := f.serve(ctx, "REPORT", coll, depth1(), bad); r.status != http.StatusForbidden ||
		!strings.Contains(r.body, "<c:supported-collation/>") {
		t.Errorf("an unsupported collation answered %d %s", r.status, r.body)
	}
}

func TestMultigetAnswersEachNameAndBoundsTheHrefs(t *testing.T) {
	f, coll, tasks := newReportFixture(t)
	ctx := davCtx(f.acme.scope.TenantID, f.password(f.acme.bob, ""))
	f.move(f.acme, tasks[1], "cancelled")
	held := coll + name(tasks[0]) + ".ics"
	departed := coll + name(tasks[1]) + ".ics"
	elsewhere := home(f.acme, f.acme.bob) + "p-" + strings.ToLower(f.acme.project.ID) + "/" + name(tasks[0]) + ".ics"
	d := parseMS(t, f.serve(ctx, "REPORT", coll, depth1(), multigetBody(held, departed, elsewhere, "/nonsense")))
	if got := d.hrefs(); !slices.Equal(got, []string{held, departed, elsewhere, "/nonsense"}) {
		t.Fatalf("answered %v", got)
	}
	r := d.response(t, held)
	if data := r.okProp(t, calName("calendar-data")).Text; !strings.Contains(data, "SUMMARY:Rotate keys") {
		t.Errorf("held object data %q", data)
	}
	r.okProp(t, davName("getetag"))
	for _, href := range []string{departed, elsewhere, "/nonsense"} {
		if got := d.response(t, href); got.Status != "HTTP/1.1 404 Not Found" || len(got.Propstats) != 0 {
			t.Errorf("%s answered %+v, want its own 404", href, got)
		}
	}

	hrefs := make([]string, maxMultigetHrefs)
	for i := range hrefs {
		hrefs[i] = "/n"
	}
	hrefs[0] = held
	if r := f.serve(ctx, "REPORT", coll, depth1(), multigetBody(hrefs...)); r.status != http.StatusMultiStatus {
		t.Errorf("%d hrefs answered %d", len(hrefs), r.status)
	}
	if r := f.serve(ctx, "REPORT", coll, depth1(), multigetBody(append(hrefs, "/n")...)); r.status != http.StatusForbidden ||
		!strings.Contains(r.body, "<d:number-of-matches-within-limits/>") {
		t.Errorf("%d hrefs answered %d %s", len(hrefs)+1, r.status, r.body)
	}
	if r := f.serve(ctx, "REPORT", home(f.acme, f.acme.bob)+"nope/", depth1(), multigetBody("/nonsense")); r.status != http.StatusNotFound {
		t.Errorf("a multiget on a missing collection answered %d", r.status)
	}
}

func TestSyncCollection(t *testing.T) {
	f, coll, tasks := newReportFixture(t)
	ctx := davCtx(f.acme.scope.TenantID, f.password(f.acme.bob, ""))

	full := parseMS(t, f.serve(ctx, "REPORT", coll, depth0(), syncBody("", 0)))
	if len(full.Responses) != 3 || full.SyncToken == "" {
		t.Fatalf("a full sync answered %v and token %q", full.hrefs(), full.SyncToken)
	}
	for _, r := range full.Responses {
		r.okProp(t, davName("getetag"))
	}

	again := parseMS(t, f.serve(ctx, "REPORT", coll, depth0(), syncBody(full.SyncToken, 0)))
	if len(again.Responses) != 0 || again.SyncToken != full.SyncToken {
		t.Fatalf("nothing changed, yet answered %v and token %q", again.hrefs(), again.SyncToken)
	}

	f.move(f.acme, tasks[0], "cancelled")
	title := "Renew the certificate"
	if _, err := f.svc.UpdateTask(adminCtx(f.acme), core.TaskRef{ID: tasks[1].ID}, core.UpdateTaskInput{Title: &title}); err != nil {
		t.Fatal(err)
	}
	inc := parseMS(t, f.serve(ctx, "REPORT", coll, depth0(), syncBody(full.SyncToken, 0)))
	if inc.SyncToken == full.SyncToken || len(inc.Responses) != 2 {
		t.Fatalf("after two changes answered %v and token %q", inc.hrefs(), inc.SyncToken)
	}
	if got := inc.response(t, coll+name(tasks[0])+".ics"); got.Status != "HTTP/1.1 404 Not Found" {
		t.Errorf("the cancelled task answered %+v, want 404", got)
	}
	inc.response(t, coll+name(tasks[1])+".ics").okProp(t, davName("getetag"))

	if r := f.serve(ctx, "REPORT", coll, depth0(), syncBody("https://tix.red/ns/sync/00", 0)); r.status != http.StatusForbidden ||
		!strings.Contains(r.body, "<d:valid-sync-token/>") {
		t.Errorf("a forgotten token answered %d %s", r.status, r.body)
	}
	if r := f.serve(ctx, "REPORT", home(f.acme, f.acme.bob), depth0(), syncBody("", 0)); r.status != http.StatusForbidden ||
		!strings.Contains(r.body, "<d:supported-report/>") {
		t.Errorf("a sync of the home answered %d %s", r.status, r.body)
	}
}

func TestSyncCollectionHonoursALimit(t *testing.T) {
	f, coll, tasks := newReportFixture(t)
	ctx := davCtx(f.acme.scope.TenantID, f.password(f.acme.bob, ""))
	start := parseMS(t, f.serve(ctx, "REPORT", coll, depth0(), syncBody("", 0)))
	for i := range 3 {
		f.task(f.acme, "more "+strconv.Itoa(i), f.acme.bob.ID)
	}
	f.move(f.acme, tasks[0], "cancelled")
	f.move(f.acme, tasks[2], "cancelled")

	first := parseMS(t, f.serve(ctx, "REPORT", coll, depth0(), syncBody(start.SyncToken, 2)))
	if len(first.Responses) != 3 {
		t.Fatalf("a limit of 2 answered %v", first.hrefs())
	}
	if got := first.response(t, coll); got.Status != "HTTP/1.1 507 Insufficient Storage" {
		t.Errorf("the request URI answered %+v, want 507", got)
	}
	rest := parseMS(t, f.serve(ctx, "REPORT", coll, depth0(), syncBody(first.SyncToken, 0)))
	if len(rest.Responses) != 3 {
		t.Fatalf("the rest answered %v, want the other 3", rest.hrefs())
	}
	seen := map[string]bool{}
	for _, h := range append(first.hrefs(), rest.hrefs()...) {
		if seen[h] {
			t.Errorf("%s answered twice", h)
		}
		seen[h] = true
	}
	for _, body := range []string{strings.Replace(syncBody("", 0), "<d:prop>", "<d:limit><d:nresults>-1</d:nresults></d:limit><d:prop>", 1),
		strings.Replace(syncBody("", 0), "<d:prop>", "<d:limit><d:nresults>many</d:nresults></d:limit><d:prop>", 1),
		strings.Replace(syncBody("", 0), "<d:sync-level>1<", "<d:sync-level>2<", 1)} {
		if r := f.serve(ctx, "REPORT", coll, depth0(), body); r.status != http.StatusBadRequest {
			t.Errorf("a bad sync body answered %d", r.status)
		}
	}
}

func TestAnUnknownReportIsRefused(t *testing.T) {
	f, coll, _ := newReportFixture(t)
	ctx := davCtx(f.acme.scope.TenantID, f.password(f.acme.bob, ""))
	for _, body := range []string{`<c:free-busy-query ` + reportNS + `/>`, `<d:expand-property xmlns:d="DAV:"/>`} {
		if r := f.serve(ctx, "REPORT", coll, depth0(), body); r.status != http.StatusForbidden ||
			!strings.Contains(r.body, "<d:supported-report/>") {
			t.Errorf("%s answered %d %s", body, r.status, r.body)
		}
	}
	if r := f.serve(ctx, "REPORT", coll, depth0(), ""); r.status != http.StatusBadRequest {
		t.Errorf("an empty report answered %d", r.status)
	}
}
