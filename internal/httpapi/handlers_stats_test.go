// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

const statsRoute = wire.RouteStats

// finish moves a task to a terminal state through the API.
func finishViaAPI(t *testing.T, f *apiFixture, task core.Task) {
	t.Helper()
	for _, to := range []string{"doing", "done"} {
		path := strings.Replace(wire.RouteTaskTransition, "{ref}", task.Ref, 1)
		resp := f.call(http.MethodPost, path, core.TransitionInput{To: to})
		mustStatus(t, resp, http.StatusOK)
		_ = resp.Body.Close()
	}
}

func TestStatsRouteReportsTheWindow(t *testing.T) {
	f := newFixture(t)
	done := f.createTask("finish me")
	f.createTask("leave me")
	finishViaAPI(t, f, done)

	resp := f.call(http.MethodGet, statsRoute, nil)
	mustStatus(t, resp, http.StatusOK)

	var got core.Stats
	decodeBody(t, resp, &got)
	if got.Completed != 1 {
		t.Errorf("completed = %d, want 1", got.Completed)
	}
	if got.Created != 2 {
		t.Errorf("created = %d, want 2", got.Created)
	}
	if len(got.PerDay) != 1 || got.PerDay[0].Completed != 1 {
		t.Errorf("per day = %+v, want one day with one completion", got.PerDay)
	}
	if len(got.TopActors) != 1 || got.TopActors[0].ActorID != f.actorA.ID {
		t.Errorf("top actors = %+v, want this tenant's actor", got.TopActors)
	}
	if got.LeaderboardMeasure != core.StatsLeaderboardMeasure {
		t.Errorf("leaderboard measure = %q, want %q", got.LeaderboardMeasure, core.StatsLeaderboardMeasure)
	}
	if len(got.Oldest) != 1 || got.Oldest[0].Title != "leave me" {
		t.Errorf("oldest = %+v, want the one open task", got.Oldest)
	}
}

func TestStatsRouteAcceptsTheSameFilters(t *testing.T) {
	f := newFixture(t)
	f.createTask("open")

	resp := f.call(http.MethodGet, statsRoute+"?project=infra&window=1h&top=1&oldest=1", nil)
	mustStatus(t, resp, http.StatusOK)

	var got core.Stats
	decodeBody(t, resp, &got)
	if got.ProjectKey != "infra" {
		t.Errorf("project key = %q, want infra", got.ProjectKey)
	}
	if got.Until.Sub(got.Since) != time.Hour {
		t.Errorf("window = %s, want an hour", got.Until.Sub(got.Since))
	}
}

func TestStatsRouteRejectionsUseTheErrorEnvelope(t *testing.T) {
	f := newFixture(t)

	tests := []struct {
		name   string
		query  string
		status int
	}{
		{"unparseable window", "?window=soon", http.StatusBadRequest},
		{"unparseable since", "?since=yesterday", http.StatusBadRequest},
		{"non-numeric top", "?top=many", http.StatusBadRequest},
		{"window past the bound", "?window=100000h", http.StatusBadRequest},
		{"unknown project", "?project=nosuchproject", http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := f.call(http.MethodGet, statsRoute+tc.query, nil)
			mustStatus(t, resp, tc.status)

			var env wire.ErrorEnvelope
			decodeBody(t, resp, &env)
			if env.Error.Code == "" || env.Error.Message == "" {
				t.Errorf("error envelope = %+v, want a code and a message", env.Error)
			}
		})
	}
}

func TestStatsRouteRequiresAuthentication(t *testing.T) {
	f := newFixture(t)

	resp := f.do(http.MethodGet, statsRoute, f.hostA, "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d: %s", resp.StatusCode, http.StatusUnauthorized, readBody(t, resp))
	}
	_ = resp.Body.Close()
}

// Statistics are answered for the tenant the request resolved to, and carry
// nothing belonging to the other one.
func TestStatsRouteNeverCrossesATenant(t *testing.T) {
	f := newFixture(t)
	done := f.createTask("finish me")
	finishViaAPI(t, f, done)

	resp := f.do(http.MethodGet, statsRoute, f.hostB, f.tokenB, nil)
	mustStatus(t, resp, http.StatusOK)

	var got core.Stats
	decodeBody(t, resp, &got)
	if got.Completed != 0 || got.Created != 0 {
		t.Errorf("the other tenant sees completed %d and created %d, want zero of each",
			got.Completed, got.Created)
	}
	for _, a := range got.TopActors {
		if a.ActorID == f.actorA.ID {
			t.Errorf("the other tenant's leaderboard names actor %q", a.ActorID)
		}
	}
	if len(got.Oldest) != 0 {
		t.Errorf("the other tenant sees ageing tasks %+v", got.Oldest)
	}
}
