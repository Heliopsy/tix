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

func projectStarPath(ref string) string {
	return strings.Replace(wire.RouteProjectStar, "{ref}", ref, 1)
}

// starredList reads the caller's starred projects through the API.
func starredList(t *testing.T, f *apiFixture, token string) []core.Project {
	t.Helper()
	resp := f.do(http.MethodGet, wire.RouteStarredProjects, f.hostA, token, nil)
	mustStatus(t, resp, http.StatusOK)
	var body struct {
		Items []core.Project `json:"items"`
	}
	decodeBody(t, resp, &body)
	return body.Items
}

func TestProjectStarEndpointsArePerCaller(t *testing.T) {
	f := newFixture(t)
	bob := seedActor(t, f.store, f.tenantA.ID, "bob-a")
	bobToken := mintToken(t, f.clock, f.lookup, f.tenantA.ID, bob.ID)

	for range 2 {
		resp := f.call(http.MethodPut, projectStarPath("infra"), nil)
		mustStatus(t, resp, http.StatusNoContent)
		_ = resp.Body.Close()
	}
	resp := f.do(http.MethodPut, projectStarPath("infra"), f.hostB, f.tokenB, nil)
	mustStatus(t, resp, http.StatusNotFound)
	_ = resp.Body.Close()

	if got := starredList(t, f, f.tokenA); len(got) != 1 || got[0].Key != "infra" || !got[0].Starred {
		t.Fatalf("the starrer's list = %+v", got)
	}
	if got := starredList(t, f, bobToken); len(got) != 0 {
		t.Fatalf("another caller's list = %+v", got)
	}
	resp = f.do(http.MethodGet, strings.Replace(wire.RouteProject, "{ref}", "infra", 1), f.hostA, bobToken, nil)
	mustStatus(t, resp, http.StatusOK)
	var theirs core.Project
	decodeBody(t, resp, &theirs)
	if theirs.Starred {
		t.Fatal("another caller's read of the project carries the star")
	}
	resp = f.call(http.MethodGet, strings.Replace(wire.RouteProject, "{ref}", "infra", 1), nil)
	var mine core.Project
	decodeBody(t, resp, &mine)
	if !mine.Starred {
		t.Fatal("the starrer's read of the project does not carry the star")
	}

	for range 2 {
		resp := f.call(http.MethodDelete, projectStarPath("infra"), nil)
		mustStatus(t, resp, http.StatusNoContent)
		_ = resp.Body.Close()
	}
	if got := starredList(t, f, f.tokenA); len(got) != 0 {
		t.Fatalf("after unstar = %+v", got)
	}
}

func TestProjectStarSurvivesTheClientRoundTrip(t *testing.T) {
	f := newFixtureWith(t, func(fx *apiFixture, cfg *httpapi.Config) {
		cfg.DefaultTenantID = fx.tenantA.ID
	})
	remote, err := client.New(f.server.URL, f.tokenA)
	if err != nil {
		t.Fatalf("building client: %v", err)
	}
	t.Cleanup(func() { _ = remote.Close() })
	ctx := context.Background()
	if err := remote.StarProject(ctx, "infra"); err != nil {
		t.Fatalf("star: %v", err)
	}
	got, err := remote.ListStarredProjects(ctx)
	if err != nil || len(got) != 1 || got[0].Key != "infra" {
		t.Fatalf("starred = %+v, %v", got, err)
	}
	if err := remote.UnstarProject(ctx, "infra"); err != nil {
		t.Fatalf("unstar: %v", err)
	}
	if got, err := remote.ListStarredProjects(ctx); err != nil || len(got) != 0 {
		t.Fatalf("after unstar = %+v, %v", got, err)
	}
}
