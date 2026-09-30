// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// seedDeliveries writes n delivery rows straight to the store, which is the
// only way to get a delivery log without a worker and a listening receiver.
// Each one carries its index in its last error, so a test can say which of
// them a screen is showing.
func seedDeliveries(t *testing.T, f *fixture, endpointID string, n int) {
	t.Helper()
	ctx := context.Background()
	if err := f.store.Update(ctx, core.TenantScope{TenantID: f.tenantA.ID}, func(tx store.Tx) error {
		for i := range n {
			d := core.WebhookDelivery{
				EndpointID: endpointID,
				EventSeq:   int64(i + 1),
				Status:     core.DeliveryFailed,
				LastError:  fmt.Sprintf("attempt-%02d", i),
			}
			if err := tx.EnqueueDelivery(ctx, &d); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seeding deliveries: %v", err)
	}
}

// The delivery log is keyset-paginated and the handler already computes the
// cursor that resumes it. Without a control on the screen that cursor reaches
// nobody: page two exists, is addressable, and is offered by nothing, so an
// older delivery can be found only by somebody who knows to type ?cursor=
// into the address bar.
//
// The assertion reads the pager's own Next link rather than the page, because
// a webhook screen is full of identifiers and form targets and "the document
// mentions a cursor" would be true of a screen carrying no control at all.
func TestTheDeliveryLogOffersItsNextPage(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	created := b.post("/admin/webhooks", url.Values{
		"url": {"https://receiver.example.test/hook"}, "active": {"1"}})
	_ = created.Body.Close()

	endpoints, err := f.svc.ListWebhooks(f.ctx())
	if err != nil || len(endpoints) != 1 {
		t.Fatalf("listing endpoints: %v (%d endpoints)", err, len(endpoints))
	}
	const rows = 3
	seedDeliveries(t, f, endpoints[0].ID, rows*2)

	first := b.page("/admin/webhooks?limit=" + fmt.Sprint(rows))
	next := hrefWithClass(t, first, "pager-next")
	if next == "" {
		t.Fatalf("the delivery log offers no way to the next page:\n%s", tail(first))
	}

	second := b.page(next)
	onFirst := deliveryMarkers(first)
	onSecond := deliveryMarkers(second)
	if len(onFirst) != rows || len(onSecond) != rows {
		t.Fatalf("page one showed %d deliveries and page two %d, want %d each (%v, %v)",
			len(onFirst), len(onSecond), rows, onFirst, onSecond)
	}
	for _, marker := range onSecond {
		if slices.Contains(onFirst, marker) {
			t.Errorf("delivery %q appears on both pages", marker)
		}
	}
}

// deliveryMarkers names the deliveries a rendered log is showing, read from
// the marker seedDeliveries put in each row's last error.
func deliveryMarkers(page string) []string {
	const width = len("attempt-00")
	var out []string
	for rest := page; ; {
		i := strings.Index(rest, "attempt-")
		if i < 0 || len(rest[i:]) < width {
			return out
		}
		rest = rest[i:]
		out = append(out, rest[:width])
		rest = rest[width:]
	}
}
