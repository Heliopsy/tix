// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// seedStatsWork creates two tasks and moves one of them to a terminal state.
func seedStatsWork(t *testing.T, c *cli) {
	t.Helper()
	c.mustRun("project", "create", "infra", "Infrastructure")
	c.mustRun("task", "add", "-p", "infra", "finish me")
	c.mustRun("task", "add", "-p", "infra", "leave me")
	c.mustRun("task", "mv", "infra-1", "doing")
	c.mustRun("task", "mv", "infra-1", "done")
}

func TestStatsTableNamesEveryFigureAndTheMeasure(t *testing.T) {
	c := newCLI(t)
	seedStatsWork(t, c)

	got := c.mustRun("stats", "-p", "infra")

	for _, want := range []string{
		"WINDOW", "project infra",
		"COMPLETED", "CREATED", "MEDIAN LEAD TIME", "SLOWEST LEAD TIME",
		"COMPLETED PER DAY", "TASKS BY STATE CATEGORY",
		"MOST ACTIVE ACTORS", core.StatsLeaderboardMeasure,
		"OLDEST TASKS NOT YET IN A TERMINAL STATE",
		"infra-2", "leave me",
	} {
		if !strings.Contains(got.out, want) {
			t.Errorf("table output is missing %q\n%s", want, got.out)
		}
	}
	if strings.Contains(got.out, "\x1b[") {
		t.Error("table output carries colour sequences")
	}
}

// The leaderboard is never shown without the line saying what it counts.
func TestStatsTableAlwaysStatesWhatTheLeaderboardCounts(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")

	got := c.mustRun("stats")

	if !strings.Contains(got.out, core.StatsLeaderboardMeasure) {
		t.Errorf("an empty leaderboard dropped the measure\n%s", got.out)
	}
}

func TestStatsJSONCarriesEveryFigureTheScreenShows(t *testing.T) {
	c := newCLI(t)
	seedStatsWork(t, c)

	got := c.mustRun("stats", "-p", "infra", "-o", "json")

	var stats core.Stats
	if err := json.Unmarshal([]byte(got.out), &stats); err != nil {
		t.Fatalf("not json: %v\n%s", err, got.out)
	}
	if stats.ProjectKey != "infra" {
		t.Errorf("project key = %q, want infra", stats.ProjectKey)
	}
	if stats.Completed != 1 || stats.Created != 2 {
		t.Errorf("completed = %d and created = %d, want 1 and 2", stats.Completed, stats.Created)
	}
	if len(stats.PerDay) != 1 || stats.PerDay[0].Completed != 1 {
		t.Errorf("per day = %+v, want one day with one completion", stats.PerDay)
	}
	if len(stats.ByCategory) == 0 {
		t.Error("json carries no state category counts")
	}
	if len(stats.TopActors) != 1 || stats.TopActors[0].Moved != 1 {
		t.Errorf("top actors = %+v, want one actor with one move", stats.TopActors)
	}
	if stats.LeaderboardMeasure != core.StatsLeaderboardMeasure {
		t.Errorf("leaderboard measure = %q, want %q", stats.LeaderboardMeasure, core.StatsLeaderboardMeasure)
	}
	if len(stats.Oldest) != 1 || stats.Oldest[0].Ref != "infra-2" {
		t.Errorf("oldest = %+v, want the one open task", stats.Oldest)
	}
	if stats.Since.After(stats.Until) {
		t.Errorf("window %s .. %s runs backwards", stats.Since, stats.Until)
	}
}

func TestStatsFlagRejections(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")

	tests := []struct {
		name string
		args []string
		code int
	}{
		{"since and window together", []string{"stats", "--since", "2026-01-01", "--window", "24h"},
			core.KindInvalid.ExitCode()},
		{"unparseable window", []string{"stats", "--window", "soon"}, core.KindInvalid.ExitCode()},
		{"unparseable since", []string{"stats", "--since", "yesterday"}, core.KindInvalid.ExitCode()},
		{"unknown project", []string{"stats", "-p", "nosuchproject"}, core.KindNotFound.ExitCode()},
		{"positional argument", []string{"stats", "extra"}, core.KindInvalid.ExitCode()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := c.run(tc.args...)
			if got.code != tc.code {
				t.Errorf("exit = %d, want %d\nstdout: %s\nstderr: %s",
					got.code, tc.code, got.out, got.err)
			}
			if got.out != "" && !strings.HasPrefix(got.out, "Usage") {
				t.Errorf("a rejected command wrote to standard output: %q", got.out)
			}
		})
	}
}
