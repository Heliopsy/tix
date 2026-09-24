// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// RouteStats is the statistics route, defined in wire beside every other one
// so the capability registry can name it without importing this package.
const RouteStats = wire.RouteStats

// registerStatsRoutes binds the statistics read.
func (rt *Router) registerStatsRoutes() {
	rt.mux.HandleFunc("GET "+RouteStats, rt.handleStats)
}

// statsInputFrom builds a statistics input from the query string.
func statsInputFrom(r *http.Request) (core.StatsInput, error) {
	q := r.URL.Query()
	in := core.StatsInput{ProjectRef: q.Get("project")}

	since, err := timeParam(q.Get("since"))
	if err != nil {
		return core.StatsInput{}, err
	}
	if since != nil {
		in.Since = *since
	}
	if raw := strings.TrimSpace(q.Get("window")); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return core.StatsInput{}, core.Invalid("window %q is not a duration such as 168h", raw)
		}
		in.Window = core.Duration(d)
	}
	if in.TopActors, err = intParam(q.Get("top"), "top"); err != nil {
		return core.StatsInput{}, err
	}
	if in.Oldest, err = intParam(q.Get("oldest"), "oldest"); err != nil {
		return core.StatsInput{}, err
	}
	return in, nil
}

// intParam reads an optional whole-number parameter.
func intParam(v, name string) (int, error) {
	if strings.TrimSpace(v) == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, core.Invalid("%s %q is not a number", name, v)
	}
	return n, nil
}

// handleStats returns throughput, ageing and the leaderboard for a window.
func (rt *Router) handleStats(w http.ResponseWriter, r *http.Request) {
	in, err := statsInputFrom(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	stats, err := rt.cfg.Service.Stats(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, stats)
}
