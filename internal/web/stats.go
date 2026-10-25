// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"strconv"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// statsRoutes serves the numbers screen.
func (h *handler) statsRoutes() []route {
	return []route{
		get(RouteStats, "stats.html", h.showStats, "Stats", "ListProjects"),
	}
}

// statsWindows are the windows the screen offers, in days.
//
// A fixed set rather than a date picker. Every question this screen answers is
// "compared to recently", and a reader who wants an exact interval has
// `tix stats --since`; a pair of date fields would double the width of the
// controls to serve the rarer case.
var statsWindows = []struct {
	Days  int
	Label string
}{
	{7, "7 days"},
	{14, "14 days"},
	{30, "30 days"},
	{90, "90 days"},
}

// statsView is what stats.html is executed against.
type statsView struct {
	Stats    *core.Stats
	Projects []core.Project
	Project  string
	Days     int
	Windows  []struct {
		Days  int
		Label string
	}
	// ProjectsWhole is false when the picker does not offer every project of
	// the tenant, whether the walk was short or the listing failed, so the
	// screen says the control is incomplete rather than letting a reader
	// conclude a project they cannot see has no statistics.
	ProjectsWhole bool
	// MaxDay is the tallest bar in the series, so every other bar can be drawn
	// as a percentage of it. Zero when nothing completed, which the template
	// checks before dividing.
	MaxDay int
	// MaxCategory and MaxActor are the same idea for the two distributions
	// below the chart: the largest count in each, so every bar in it can be
	// drawn as a share of its own series rather than of the page. Zero when
	// the series is empty, which the template checks before dividing.
	MaxCategory int
	MaxActor    int
	// TotalCategory is every task counted by the breakdown, so one divided
	// bar can be drawn as shares of the whole rather than as six bars each
	// drawn against the largest.
	TotalCategory int
	// Measure states what the leaderboard counts. It comes from core rather
	// than the template so the wording cannot drift from the CLI's.
	Measure string
}

// showStats renders throughput, ageing and the leaderboard for one window.
func (h *handler) showStats(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()

	days := 14
	if v := q.Get("days"); v != "" {
		// A window that does not parse falls back rather than erroring: this
		// value arrives from a link, and a broken query string should show the
		// default screen, not a failure.
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 3650 {
			days = n
		}
	}

	in := core.StatsInput{
		ProjectRef: q.Get("project"),
		Window:     core.Duration(time.Duration(days) * 24 * time.Hour),
	}
	stats, err := held(h.svc.Stats(r.Context(), in))
	if err != nil {
		return err
	}

	// The project list is the filter control. A failure to read it leaves the
	// screen without a picker rather than without numbers, and either a
	// failure or a walk that stopped at its bound is said on the screen: one
	// page of it meant a tenant past core.DefaultPageLimit projects could not
	// filter by its fifty-first.
	projects, whole, err := h.allProjects(r, false)
	if err != nil {
		projects, whole = nil, false
	}

	max := 0
	for _, d := range stats.PerDay {
		if d.Completed > max {
			max = d.Completed
		}
	}

	topCategory, allCategories := 0, 0
	for _, c := range stats.ByCategory {
		allCategories += c.Count
		if c.Count > topCategory {
			topCategory = c.Count
		}
	}
	topActor := 0
	for _, a := range stats.TopActors {
		if a.Moved > topActor {
			topActor = a.Moved
		}
	}

	return h.render(w, r, "stats.html", "Statistics", statsView{
		Stats:         stats,
		MaxCategory:   topCategory,
		TotalCategory: allCategories,
		MaxActor:      topActor,
		Projects:      projects,
		Project:       q.Get("project"),
		Days:          days,
		Windows:       statsWindows,
		ProjectsWhole: whole,
		MaxDay:        max,
		Measure:       core.StatsLeaderboardMeasure,
	})
}
