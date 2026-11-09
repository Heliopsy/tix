// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
)

// endpointRow returns the one endpoint row of the webhook screen that names
// this url, so an assertion about its actions reads that row and not its
// neighbour's.
func endpointRow(t *testing.T, page, rawURL string) string {
	t.Helper()
	at := strings.Index(page, `<span class="recrow-title mono">`+rawURL+`</span>`)
	if at < 0 {
		t.Fatalf("the webhook screen lists no endpoint %s", rawURL)
	}
	start := strings.LastIndex(page[:at], "<li ")
	end := strings.Index(page[at:], "</li>")
	if start < 0 || end < 0 {
		t.Fatalf("the endpoint %s is not inside a row", rawURL)
	}
	return page[start : at+end]
}

// TestAnActiveEndpointCanBePingedFromItsRow covers the button and what it
// does: only an active endpoint offers it, and pressing it queues a delivery
// the log below shows.
func TestAnActiveEndpointCanBePingedFromItsRow(t *testing.T) {
	t.Parallel()
	f := newFixtureHooks(t, service.HookServer)
	b := f.as("alice")

	const on, off = "https://hooks.example.test/on", "https://hooks.example.test/off"
	saveEndpoint(t, b, url.Values{"url": {on}, "event_types": {"task.*"}, "active": {"1"}})
	saveEndpoint(t, b, url.Values{"url": {off}, "event_types": {"task.*"}})

	page := b.page("/admin/webhooks")
	if row := endpointRow(t, page, off); strings.Contains(row, `action="/admin/webhooks/ping"`) {
		t.Errorf("an inactive endpoint offers a ping it would refuse:\n%s", row)
	}
	row := endpointRow(t, page, on)
	form := between(t, row, `action="/admin/webhooks/ping"`, "</form>")
	id := between(t, form, `<input type="hidden" name="id" value="`, `"`)
	if id == "" || !strings.Contains(form, `<button`) {
		t.Fatalf("the active endpoint's ping form names no endpoint or has no button:\n%s", form)
	}

	resp := b.post("/admin/webhooks/ping", url.Values{"id": {id}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	if loc := resp.Header.Get("Location"); !strings.Contains(loc, "flash=ping+queued") {
		t.Errorf("ping redirected to %q, want the webhook screen saying the ping is queued", loc)
	}

	deliveries, _, err := f.svc.ListDeliveries(f.ctx(), core.DeliveryFilter{EndpointID: id})
	if err != nil {
		t.Fatalf("ListDeliveries: %v", err)
	}
	if len(deliveries) != 1 || deliveries[0].Status != core.DeliveryPending {
		t.Fatalf("pinging queued %+v, want one pending delivery to %s", deliveries, id)
	}
	log := between(t, b.page("/admin/webhooks"), "<h2>", "</ul>")
	if !strings.Contains(log, "<code>"+deliveries[0].ID+"</code>") {
		t.Errorf("the delivery log does not list the ping's delivery %s:\n%s", deliveries[0].ID, log)
	}
}
