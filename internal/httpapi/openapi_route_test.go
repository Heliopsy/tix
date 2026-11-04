// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/wire"
)

// TestTheOpenAPIDocumentIsServedWithoutACredential reads the document from a
// host no tenant owns and with no credential, which is how a client generator
// or a browser tool meets it first.
func TestTheOpenAPIDocumentIsServedWithoutACredential(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	resp := f.do(http.MethodGet, wire.RouteOpenAPI, "nobody.test", "", nil)
	mustStatus(t, resp, http.StatusOK)
	if ct := resp.Header.Get(wire.HeaderContentType); ct != wire.ContentJSON {
		t.Errorf("content type = %q, want %q", ct, wire.ContentJSON)
	}
	var doc struct {
		OpenAPI string         `json:"openapi"`
		Paths   map[string]any `json:"paths"`
	}
	decodeBody(t, resp, &doc)
	if !strings.HasPrefix(doc.OpenAPI, "3.1") {
		t.Errorf("openapi = %q, want 3.1", doc.OpenAPI)
	}
	if _, ok := doc.Paths[wire.RouteOpenAPI]; !ok {
		t.Errorf("the document does not describe %s itself", wire.RouteOpenAPI)
	}
}
