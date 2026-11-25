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

// registerEventRoutes binds the plain HTTP poll of the event log.
func (rt *Router) registerEventRoutes() {
	rt.mux.HandleFunc("GET "+wire.RouteEventsPoll, rt.handlePollEvents)
}

// eventPollFrom builds a poll from the query string.
func eventPollFrom(r *http.Request) (core.EventPollInput, error) {
	q := r.URL.Query()
	var in core.EventPollInput
	if raw := strings.TrimSpace(q.Get("since_seq")); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return in, core.Invalid("since_seq %q is not a number", raw)
		}
		in.SinceSeq = n
	}
	for _, t := range commaList(q["types"]) {
		in.Types = append(in.Types, core.EventType(t))
	}
	in.ProjectIDs = commaList(q["project_ids"])
	limit, err := intParam(q.Get("limit"), "limit")
	if err != nil {
		return in, err
	}
	in.Limit = limit
	if in.Wait, err = waitParam(q.Get("wait")); err != nil {
		return in, err
	}
	return in, nil
}

// waitParam reads a duration such as 25s, or a bare number of seconds.
func waitParam(raw string) (core.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	if n, err := strconv.Atoi(raw); err == nil {
		return core.Duration(time.Duration(n) * time.Second), nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, core.Invalid("wait %q is not a duration such as 25s", raw)
	}
	return core.Duration(d), nil
}

// commaList splits repeated, comma-separated values and drops empty ones.
func commaList(values []string) []string {
	var out []string
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

// handlePollEvents answers the events after a cursor, waiting for one if asked to.
func (rt *Router) handlePollEvents(w http.ResponseWriter, r *http.Request) {
	in, err := eventPollFrom(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	if limit := rt.cfg.RequestTimeout; limit > 0 && in.Wait.D() >= limit {
		WriteError(w, core.Invalid("wait %s must be shorter than this server's request timeout of %s", in.Wait, limit))
		return
	}
	page, err := rt.cfg.Service.PollEvents(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, page)
}
