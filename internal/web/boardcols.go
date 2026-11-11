// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
)

// MoreParam names a board column, or the lanes, whose cap the reader raised
// with Show more. Each occurrence raises it by one step.
const MoreParam = "more"

// moreLanes is the MoreParam value that raises the lanes view's cap.
const moreLanes = "lanes"

// columnStep is how many cards a board column draws, and how many more each
// Show more adds; lanesStep is the same for the lanes view as a whole.
const (
	columnStep = 50
	lanesStep  = 200
)

// shownOf is how many tasks the part of the screen named key draws: step,
// plus step for every Show more the address carries for it.
func shownOf(r *http.Request, key string, step int) int {
	n := 1
	for _, v := range r.URL.Query()[MoreParam] {
		if v == key {
			n++
		}
	}
	return min(n*step, core.MaxPageLimit)
}

// moreHref is the address with one more Show more for key.
func moreHref(r *http.Request, route, key string) string {
	query := r.URL.Query()
	query.Del("flash")
	query.Del("flashref")
	query.Del(FlashUndoParam)
	query.Add(MoreParam, key)
	return route + "?" + query.Encode()
}

// columnCount is what a board column says beyond its cards: how many tasks
// the state holds in all, and the Show more that draws the next step.
type columnCount struct {
	Total    int
	More     int
	MoreHref string
}

// boardTasks reads every column of an agreed board on its own, so a column is
// never short because its tasks sit on another page of one shared listing.
// Each column reads up to its cap and reports its true total; tasks in a
// state no column has are read last, so the board is never quietly short.
func (h *handler) boardTasks(r *http.Request, route string, def core.WorkflowDefinition, filter core.TaskFilter) ([]core.Task, map[string]columnCount, error) {
	filter.Tally, filter.PageNumber, filter.Page.Cursor = false, 0, ""
	asked := make(map[string]bool, len(filter.Statuses))
	for _, s := range filter.Statuses {
		asked[s] = true
	}
	counts := make(map[string]columnCount, len(def.States))
	keys := make([]string, 0, len(def.States))
	var out []core.Task
	for _, state := range def.States {
		keys = append(keys, state.Key)
		if len(asked) > 0 && !asked[state.Key] {
			continue
		}
		f := filter
		f.Statuses = []string{state.Key}
		f.Page.Limit = shownOf(r, state.Key, columnStep)
		page, err := h.svc.ListTasks(r.Context(), f)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, page.Tasks...)
		c := columnCount{Total: page.Total}
		if page.Total > len(page.Tasks) && f.Page.Limit < core.MaxPageLimit {
			c.More = min(columnStep, page.Total-len(page.Tasks))
			c.MoreHref = moreHref(r, route, state.Key)
		}
		counts[state.Key] = c
	}
	stray := filter
	stray.Exclude.Statuses = append(append([]string(nil), stray.Exclude.Statuses...), keys...)
	stray.Page.Limit = columnStep
	page, err := h.svc.ListTasks(r.Context(), stray)
	if err != nil {
		return nil, nil, err
	}
	return append(out, page.Tasks...), counts, nil
}

// countColumns writes each column's true total and Show more onto the board.
func countColumns(board *taskBoard, counts map[string]columnCount) {
	for i := range board.Columns {
		c := counts[board.Columns[i].State.Key]
		board.Columns[i].Total = max(c.Total, len(board.Columns[i].Tasks))
		board.Columns[i].More, board.Columns[i].MoreHref = c.More, c.MoreHref
	}
}
