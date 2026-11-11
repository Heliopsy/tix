// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/client"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/wire"
)

func movePath(ref string) string { return strings.Replace(wire.RouteTaskMove, "{ref}", ref, 1) }

func TestMoveEndpoint(t *testing.T) {
	f := newFixture(t)
	seedProject(t, f.store, f.tenantA.ID, "web")
	cases := []struct {
		name  string
		host  string
		token string
		body  func(src *core.Task) any
		want  int
	}{
		{"another project", f.hostA, f.tokenA, func(src *core.Task) any {
			return map[string]any{"project_ref": "web", "version": src.Version}
		}, http.StatusOK},
		{"stale version", f.hostA, f.tokenA, func(src *core.Task) any {
			return map[string]any{"project_ref": "web", "version": src.Version + 1}
		}, http.StatusConflict},
		{"its own project", f.hostA, f.tokenA, func(*core.Task) any {
			return map[string]any{"project_ref": "infra"}
		}, http.StatusBadRequest},
		{"no body", f.hostA, f.tokenA, func(*core.Task) any { return nil }, http.StatusBadRequest},
		{"unknown project", f.hostA, f.tokenA, func(*core.Task) any {
			return map[string]any{"project_ref": "nope"}
		}, http.StatusNotFound},
		{"another tenant", f.hostB, f.tokenB, func(*core.Task) any {
			return map[string]any{"project_ref": "web"}
		}, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := f.createTask("rotate keys")
			resp := f.do(http.MethodPost, movePath(src.Ref), tc.host, tc.token, tc.body(&src))
			mustStatus(t, resp, tc.want)
			if tc.want != http.StatusOK {
				_ = resp.Body.Close()
				return
			}
			var got core.Task
			decodeBody(t, resp, &got)
			if !strings.HasPrefix(got.Ref, "web-") || got.ID != src.ID {
				t.Fatalf("moved = %+v, want %s under a web reference", got, src.ID)
			}
			gone := f.do(http.MethodGet, strings.Replace(wire.RouteTask, "{ref}", src.Ref, 1), tc.host, tc.token, nil)
			mustStatus(t, gone, http.StatusNotFound)
			_ = gone.Body.Close()
		})
	}
}

func TestMoveSurvivesTheClientRoundTrip(t *testing.T) {
	f := newFixtureWith(t, func(fx *apiFixture, cfg *httpapi.Config) {
		cfg.DefaultTenantID = fx.tenantA.ID
	})
	seedProject(t, f.store, f.tenantA.ID, "web")
	src := f.createTask("rotate keys")

	remote, err := client.New(f.server.URL, f.tokenA)
	if err != nil {
		t.Fatalf("building client: %v", err)
	}
	t.Cleanup(func() { _ = remote.Close() })
	got, err := remote.MoveTask(context.Background(), core.TaskRef{ID: src.ID}, core.MoveTaskInput{ProjectRef: "web"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if !strings.HasPrefix(got.Ref, "web-") || got.ID != src.ID {
		t.Fatalf("moved = %+v", got)
	}
	_, err = remote.MoveTask(context.Background(), core.TaskRef{ID: src.ID}, core.MoveTaskInput{ProjectRef: "web"})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("moving into its own project over the client = %v, want invalid", err)
	}
}
