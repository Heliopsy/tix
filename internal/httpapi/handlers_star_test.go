// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/client"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/wire"
)

func starPath(ref string) string { return strings.Replace(wire.RouteTaskStar, "{ref}", ref, 1) }

func TestStarEndpoints(t *testing.T) {
	f := newFixture(t)
	f.createTask("a")
	f.createTask("b")
	c := f.createTask("c")

	steps := []struct {
		method string
		host   string
		token  string
		want   int
	}{
		{http.MethodPut, f.hostA, f.tokenA, http.StatusNoContent},
		{http.MethodPut, f.hostA, f.tokenA, http.StatusNoContent},
		{http.MethodPut, f.hostB, f.tokenB, http.StatusNotFound},
	}
	for _, step := range steps {
		resp := f.do(step.method, starPath(c.Ref), step.host, step.token, nil)
		mustStatus(t, resp, step.want)
		_ = resp.Body.Close()
	}

	resp := f.call(http.MethodGet, strings.Replace(wire.RouteTask, "{ref}", c.Ref, 1), nil)
	var got core.Task
	decodeBody(t, resp, &got)
	if !got.Starred {
		t.Fatal("a read after PUT star does not carry starred")
	}

	lists := []struct {
		query string
		want  []string
	}{
		{"", []string{"c", "a", "b"}},
		{"?starred=true", []string{"c"}},
		{"?starred=false", []string{"a", "b"}},
	}
	for _, l := range lists {
		if titles := f.listTitles(l.query); !slices.Equal(titles, l.want) {
			t.Errorf("list %q = %v, want %v", l.query, titles, l.want)
		}
	}

	for range 2 {
		resp := f.call(http.MethodDelete, starPath(c.Ref), nil)
		mustStatus(t, resp, http.StatusNoContent)
		_ = resp.Body.Close()
	}
	if titles := f.listTitles(""); !slices.Equal(titles, []string{"a", "b", "c"}) {
		t.Errorf("after unstar = %v", titles)
	}
}

func TestStarSurvivesTheClientRoundTrip(t *testing.T) {
	f := newFixtureWith(t, func(fx *apiFixture, cfg *httpapi.Config) {
		cfg.DefaultTenantID = fx.tenantA.ID
	})
	f.createTask("plain")
	watched := f.createTask("watched")

	remote, err := client.New(f.server.URL, f.tokenA)
	if err != nil {
		t.Fatalf("building client: %v", err)
	}
	t.Cleanup(func() { _ = remote.Close() })
	ctx := context.Background()

	if err := remote.StarTask(ctx, core.TaskRef{ID: watched.ID}); err != nil {
		t.Fatalf("star: %v", err)
	}
	page, err := remote.ListTasks(ctx, core.TaskFilter{Starred: core.Yes})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page.Tasks) != 1 || page.Tasks[0].ID != watched.ID || !page.Tasks[0].Starred {
		t.Fatalf("starred listing = %+v", page.Tasks)
	}
	if err := remote.UnstarTask(ctx, core.TaskRef{ID: watched.ID}); err != nil {
		t.Fatalf("unstar: %v", err)
	}
	page, err = remote.ListTasks(ctx, core.TaskFilter{Starred: core.Yes})
	if err != nil || len(page.Tasks) != 0 {
		t.Fatalf("after unstar: %+v, %v", page.Tasks, err)
	}
}
