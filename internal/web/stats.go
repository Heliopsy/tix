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
	// MaxDay is the tallest bar in the series, so every other bar can be drawn
	// as a percentage of it. Zero when nothing completed, which the template
	// checks before dividing.
	MaxDay int
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
	stats, err := h.svc.Stats(r.Context(), in)
	if err != nil {
		return err
	}

	// The project list is the filter control. A failure to read it leaves the
	// screen without a picker rather than without numbers.
	projects, _, _ := h.svc.ListProjects(r.Context(), core.ProjectFilter{Page: core.Page{Limit: core.DefaultPageLimit}})

	max := 0
	for _, d := range stats.PerDay {
		if d.Completed > max {
			max = d.Completed
		}
	}

	return h.render(w, r, "stats.html", "Statistics", statsView{
		Stats:    stats,
		Projects: projects,
		Project:  q.Get("project"),
		Days:     days,
		Windows:  statsWindows,
		MaxDay:   max,
		Measure:  core.StatsLeaderboardMeasure,
	})
}
