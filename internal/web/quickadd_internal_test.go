// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/heliopsy/tix/internal/output"
)

// readerNow counts in the zone the browser chose, and in the deployment's own
// when it chose none or a zone this build does not offer.
func TestReaderNowIsInTheReadersZone(t *testing.T) {
	t.Parallel()
	style, err := output.NewTimeStyle("", "utc")
	if err != nil {
		t.Fatal(err)
	}
	h := &handler{style: style}
	for _, tc := range []struct{ cookie, want string }{
		{"", "UTC"},
		{"Asia/Tokyo", "Asia/Tokyo"},
		{"America/Los_Angeles", "America/Los_Angeles"},
		{"Mars/Olympus", "UTC"},
	} {
		r := httptest.NewRequest(http.MethodGet, RouteQuickAdd, nil)
		if tc.cookie != "" {
			r.AddCookie(&http.Cookie{Name: TimezoneCookie, Value: tc.cookie})
		}
		if got := h.readerNow(r).Location().String(); got != tc.want {
			t.Errorf("cookie %q counts in %s, want %s", tc.cookie, got, tc.want)
		}
	}
}
