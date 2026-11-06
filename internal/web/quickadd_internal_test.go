// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/heliopsy/tix/internal/output"
)

// readerNow counts in the zone the reader resolved to, and in the
// deployment's own for a request that resolved none.
func TestReaderNowIsInTheReadersZone(t *testing.T) {
	t.Parallel()
	style, err := output.NewTimeStyle("", "utc")
	if err != nil {
		t.Fatal(err)
	}
	h := &handler{style: style}
	for _, zone := range []string{"", "Asia/Tokyo", "America/Los_Angeles"} {
		r := httptest.NewRequest(http.MethodGet, RouteQuickAdd, nil)
		want := "UTC"
		if zone != "" {
			reader, err := output.NewTimeStyle("", zone)
			if err != nil {
				t.Fatal(err)
			}
			r = r.WithContext(context.WithValue(r.Context(), readerTimeKey{}, readerTime{style: reader}))
			want = zone
		}
		if got := h.readerNow(r).Location().String(); got != want {
			t.Errorf("a reader in %q counts in %s, want %s", zone, got, want)
		}
	}
}
