// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// The service generated a secret and the handler wrote the endpoint, whose
// secret field never serialises, so an HTTP caller registered an endpoint it
// could not verify a single delivery from. The registration answer carries
// it under "secret"; the listing never does.
func TestRegisteringAWebhookOverHTTPReturnsTheGeneratedSecretOnce(t *testing.T) {
	f := newFixture(t)

	created := f.call(http.MethodPut, wire.RouteWebhooks,
		core.WebhookInput{URL: "https://example.com/hook", Active: true})
	mustStatus(t, created, http.StatusOK)
	var issued map[string]any
	decodeBody(t, created, &issued)
	if issued["secret"] != fixtureWebhookSecret {
		t.Fatalf("PUT %s answered %v, want the generated secret under \"secret\"", wire.RouteWebhooks, issued)
	}

	supplied := f.call(http.MethodPut, wire.RouteWebhooks,
		core.WebhookInput{ID: "hook-own", URL: "https://example.com/own", Secret: "mine", Active: true})
	mustStatus(t, supplied, http.StatusOK)
	if body := readBody(t, supplied); strings.Contains(body, `"secret"`) {
		t.Errorf("a supplied secret was answered back: %s", body)
	}

	listed := f.call(http.MethodGet, wire.RouteWebhooks, nil)
	mustStatus(t, listed, http.StatusOK)
	if body := readBody(t, listed); strings.Contains(body, `"secret"`) || strings.Contains(body, fixtureWebhookSecret) {
		t.Errorf("the endpoint listing carries a secret: %s", body)
	}
}
