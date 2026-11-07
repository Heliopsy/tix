// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

func TestTimeSettingsOverHTTP(t *testing.T) {
	f := newFixture(t)
	resp := f.call(http.MethodGet, wire.RouteTimeSettings, nil)
	mustStatus(t, resp, http.StatusOK)
	var got core.TimeSettings
	decodeBody(t, resp, &got)
	if got.FormatSource == "" || got.TimezoneSource == "" {
		t.Errorf("settings name no source: %+v", got)
	}

	zone := "Mars/Olympus"
	resp = f.call(http.MethodPatch, wire.RouteTimeSettings, core.TimePrefsInput{Timezone: &zone})
	mustStatus(t, resp, http.StatusBadRequest)
	if body := readBody(t, resp); !strings.Contains(body, "Mars/Olympus") {
		t.Errorf("an unknown zone answered %s", body)
	}

	resp = f.call(http.MethodPatch, wire.RouteTimeSettings, "not an object")
	mustStatus(t, resp, http.StatusBadRequest)
	_ = resp.Body.Close()
}
