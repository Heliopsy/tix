// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestFingerprintCoversMethodPathQueryAndBody(t *testing.T) {
	base := fingerprint(httptest.NewRequest("POST", "/api/v1/tasks?a=1", nil), []byte(`{"t":1}`))
	for name, got := range map[string]string{
		"method": fingerprint(httptest.NewRequest("PUT", "/api/v1/tasks?a=1", nil), []byte(`{"t":1}`)),
		"path":   fingerprint(httptest.NewRequest("POST", "/api/v1/task?a=1", nil), []byte(`{"t":1}`)),
		"query":  fingerprint(httptest.NewRequest("POST", "/api/v1/tasks?a=2", nil), []byte(`{"t":1}`)),
		"body":   fingerprint(httptest.NewRequest("POST", "/api/v1/tasks?a=1", nil), []byte(`{"t":2}`)),
	} {
		if got == base {
			t.Errorf("a different %s gives the same fingerprint", name)
		}
	}
	same := fingerprint(httptest.NewRequest("POST", "/api/v1/tasks?a=1", nil), []byte(`{"t":1}`))
	if same != base {
		t.Error("the same request gives a different fingerprint")
	}
}

func TestRetrySecondsRoundsUpToAtLeastOne(t *testing.T) {
	for d, want := range map[time.Duration]int{0: 1, time.Millisecond: 1, 1500 * time.Millisecond: 2, time.Minute: 60} {
		if got := retrySeconds(d); got != want {
			t.Errorf("retrySeconds(%s) = %d, want %d", d, got, want)
		}
	}
}
