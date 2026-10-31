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

func copyPath(ref string) string { return strings.Replace(wire.RouteTaskCopy, "{ref}", ref, 1) }

func TestCopyEndpoint(t *testing.T) {
	f := newFixture(t)
	seedProject(t, f.store, f.tenantA.ID, "web")
	src := f.createTask("rotate keys")

	cases := []struct {
		name   string
		host   string
		token  string
		body   any
		want   int
		prefix string
	}{
		{"another project, linked", f.hostA, f.tokenA, map[string]any{"project_ref": "web", "link": true}, http.StatusCreated, "web-"},
		{"no body copies in place", f.hostA, f.tokenA, nil, http.StatusCreated, "infra-"},
		{"unknown project", f.hostA, f.tokenA, map[string]any{"project_ref": "nope"}, http.StatusNotFound, ""},
		{"another tenant", f.hostB, f.tokenB, nil, http.StatusNotFound, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := f.do(http.MethodPost, copyPath(src.Ref), tc.host, tc.token, tc.body)
			if tc.want != http.StatusCreated {
				mustStatus(t, resp, tc.want)
				_ = resp.Body.Close()
				return
			}
			mustStatus(t, resp, http.StatusCreated)
			var got core.Task
			decodeBody(t, resp, &got)
			if !strings.HasPrefix(got.Ref, tc.prefix) || got.Title != "rotate keys" || got.ID == src.ID {
				t.Fatalf("copy = %+v, want a fresh %s task", got, tc.prefix)
			}
			if linked := tc.body != nil; linked != (got.CopiedFromID == src.ID) {
				t.Fatalf("copied_from_id = %q, linked = %v", got.CopiedFromID, linked)
			}
		})
	}
}

func TestCopySurvivesTheClientRoundTrip(t *testing.T) {
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
	got, err := remote.CopyTask(context.Background(), core.TaskRef{ID: src.ID},
		core.CopyTaskInput{ProjectRef: "web", Title: "again", Link: true})
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if !strings.HasPrefix(got.Ref, "web-") || got.Title != "again" || got.CopiedFromID != src.ID {
		t.Fatalf("copy = %+v", got)
	}
}
