// SPDX-License-Identifier: AGPL-3.0-or-later

package sso

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestLoopbackRefusesAForeignState(t *testing.T) {
	answers := make(chan core.CompleteSSOInput, 1)
	h := loopbackHandler(&core.SSOBegin{Flow: "f", State: "right"}, answers)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://127.0.0.1/callback?code=c&state=wrong", nil))
	if rec.Code != http.StatusBadRequest || len(answers) != 0 {
		t.Errorf("foreign state = %d with %d answers, want 400 and none", rec.Code, len(answers))
	}
}
