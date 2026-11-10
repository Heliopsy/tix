// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/wire"
)

// TestAnArtifactUnderItsLimitIsNotHeldToTheBodyLimit puts a 512 KiB file on a
// server whose general body limit is 4 KiB, and a file past what
// artifacts.max_bytes leaves room for, which is refused naming that key.
func TestAnArtifactUnderItsLimitIsNotHeldToTheBodyLimit(t *testing.T) {
	f := newFixtureWith(t, func(_ *apiFixture, cfg *httpapi.Config) { cfg.MaxArtifactBytes = 1 << 20 })
	resp := f.call(http.MethodPost, wire.RouteTasks, core.CreateTaskInput{ProjectRef: "infra", Title: "t"})
	mustStatus(t, resp, http.StatusCreated)
	var task core.Task
	decodeBody(t, resp, &task)
	route := strings.Replace(wire.RouteTaskArtifacts, "{ref}", task.Ref, 1)

	resp = f.call(http.MethodPut, route, core.ArtifactInput{
		Kind: core.ArtifactFile, ContentType: "text/plain", Blob: make([]byte, 512<<10)})
	mustStatus(t, resp, http.StatusOK)
	_ = readBody(t, resp)

	resp = f.call(http.MethodPut, route, core.ArtifactInput{
		Kind: core.ArtifactFile, ContentType: "text/plain", Blob: make([]byte, 3<<20)})
	mustStatus(t, resp, http.StatusRequestEntityTooLarge)
	var env wire.ErrorEnvelope
	decodeBody(t, resp, &env)
	if want := config.ArtifactLimitRefusal(1 << 20); env.Error.Message != want {
		t.Errorf("refused with %q, want %q", env.Error.Message, want)
	}

	resp = f.call(http.MethodGet, wire.RouteArtifactLimits, nil)
	mustStatus(t, resp, http.StatusOK)
	var limits core.ArtifactLimits
	decodeBody(t, resp, &limits)
	if limits.MaxBytes != core.DefaultArtifactMaxBytes || len(limits.AllowedTypes) == 0 {
		t.Errorf("limits = %+v", limits)
	}
}
