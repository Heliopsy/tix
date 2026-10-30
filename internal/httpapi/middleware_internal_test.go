// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The envelope replaces the mux's own plain-text not found, and nothing a
// handler rendered itself: a JSON error keeps its body, and so does the
// browser interface's HTML error screen, which used to reach the reader as
// "no such route" in JSON.
func TestMuxErrorsEnvelopeOnlyTheMuxsOwnAnswer(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, contentType, body string
		replaced                bool
	}{
		{"the mux's plain text", "text/plain; charset=utf-8", "404 page not found\n", true},
		{"no content type", "", "nothing", true},
		{"a JSON error", "application/json", `{"error":{"code":"not_found","message":"no task"}}`, false},
		{"an HTML error screen", "text/html; charset=utf-8", "<h1>That was not found</h1>", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			w := &muxErrorWriter{ResponseWriter: rec}
			if tc.contentType != "" {
				w.Header().Set("Content-Type", tc.contentType)
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(tc.body))
			replaced := strings.Contains(rec.Body.String(), "no such route")
			if replaced != tc.replaced {
				t.Errorf("replaced = %v, want %v: %q", replaced, tc.replaced, rec.Body.String())
			}
			if !tc.replaced && rec.Body.String() != tc.body {
				t.Errorf("the handler's own body was changed to %q", rec.Body.String())
			}
		})
	}
}
