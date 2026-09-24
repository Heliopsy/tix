// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"sort"
	"time"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// statsCategoryOrder is the order the state categories are reported in: where
// work waits, where it is moving, and where it has landed.
var statsCategoryOrder = []core.StateCategory{
	core.CategoryTodo, core.CategoryInProgress, core.CategoryDone,
}

// Stats reports throughput, ageing and who closed what over a window ending
// now, for this tenant and optionally one of its projects.
func (l *Local) Stats(ctx context.Context, in core.StatsInput) (*core.Stats, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	until := l.clock.Now()
	since, err := statsSince(in, until)
	if err != nil {
		return nil, err
	}

	out := &core.Stats{
		Since:              since,
		Until:              until,
		PerDay:             []core.StatsDay{},
		ByCategory:         []core.StatsCategory{},
		TopActors:          []core.StatsActor{},
		Oldest:             []core.StatsAgeing{},
		LeaderboardMeasure: core.StatsLeaderboardMeasure,
	}
	err = l.read(ctx, actor, func(tx store.Tx) error {
		q := store.StatsQuery{Since: since, Until: until, Oldest: statsOldest(in)}
		workflowID := ""
		if in.ProjectRef != "" {
			project, err := tx.GetProject(ctx, in.ProjectRef)
			if err != nil {
				return err
			}
			out.ProjectID, out.ProjectKey, workflowID = project.ID, project.Key, project.WorkflowID
			q.ProjectID = project.ID
		}
		categories, err := statsCategories(ctx, tx, workflowID)
		if err != nil {
			return err
		}
		for status := range categories {
			q.Statuses = append(q.Statuses, status)
		}
		sort.Strings(q.Statuses)

		rows, err := tx.TaskStats(ctx, q)
		if err != nil {
			return err
		}
		applyStatsRows(out, rows, categories, until, statsTop(in))
		return nameActors(ctx, tx, out.TopActors)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// statsSince resolves the start of the window. An explicit instant wins over a
// length, and a window that has not begun yet is a mistake rather than an empty
// answer, because it reads as "nothing happened" for as long as nobody notices.
func statsSince(in core.StatsInput, until time.Time) (time.Time, error) {
	if !in.Since.IsZero() {
		since := in.Since.UTC()
		if since.After(until) {
			return time.Time{}, core.Invalid("statistics window starts in the future")
		}
		return since, nil
	}
	window := in.Window
	if window == 0 {
		window = core.DefaultStatsWindow
	}
	return until.Add(-window.D()), nil
}

// statsOldest resolves how many ageing tasks to report.
func statsOldest(in core.StatsInput) int {
	if in.Oldest == 0 {
		return core.DefaultStatsOldest
	}
	return in.Oldest
}

// statsTop resolves how many actors the leaderboard names.
func statsTop(in core.StatsInput) int {
	if in.TopActors == 0 {
		return core.DefaultStatsTopActors
	}
	return in.TopActors
}

// statsCategories maps every status the relevant workflows define onto the
// category it counts towards. A state that names no category is placed by the
// only other things the definition says about it, so a workflow written before
// categories existed still reports somewhere rather than nowhere.
func statsCategories(ctx context.Context, tx store.Tx, workflowID string) (map[string]core.StateCategory, error) {
	defs, err := statsDefinitions(ctx, tx, workflowID)
	if err != nil {
		return nil, err
	}
	out := map[string]core.StateCategory{}
	for _, def := range defs {
		for _, state := range def.States {
			if _, seen := out[state.Key]; seen {
				continue
			}
			out[state.Key] = statsCategoryOf(def, state)
		}
	}
	return out, nil
}

// statsDefinitions returns the workflows a read covers: the named project's
// one, or every workflow in the tenant.
func statsDefinitions(ctx context.Context, tx store.Tx, workflowID string) ([]core.WorkflowDefinition, error) {
	if workflowID != "" {
		wf, err := tx.GetWorkflowByID(ctx, workflowID)
		if err != nil {
			return nil, err
		}
		return []core.WorkflowDefinition{wf.Definition}, nil
	}
	workflows, err := tx.ListWorkflows(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]core.WorkflowDefinition, 0, len(workflows))
	for _, wf := range workflows {
		out = append(out, wf.Definition)
	}
	return out, nil
}

func statsCategoryOf(def core.WorkflowDefinition, state core.State) core.StateCategory {
	switch {
	case state.Category != "":
		return state.Category
	case state.Terminal:
		return core.CategoryDone
	case state.Key == def.Initial:
		return core.CategoryTodo
	default:
		return core.CategoryInProgress
	}
}

// applyStatsRows derives every reported figure from the rows the store read.
func applyStatsRows(out *core.Stats, rows *store.StatsRows, categories map[string]core.StateCategory, until time.Time, top int) {
	out.Completed = len(rows.Completed)
	out.Created = rows.Created
	out.PerDay = statsPerDay(rows.Completed)
	out.MedianLeadTime, out.SlowestLeadTime = statsLeadTimes(rows.Completed)
	out.ByCategory = statsByCategory(rows.StatusCounts, categories)
	out.TopActors = statsTopActors(rows.Completed, top)
	out.Oldest = statsAgeing(rows.Oldest, until)
}

// statsPerDay counts completions against the UTC day they happened on. Days
// with nothing on them are absent: a window in which nothing happened reports
// an empty series rather than a run of zeroes.
func statsPerDay(completed []store.CompletedTask) []core.StatsDay {
	byDay := map[string]int{}
	for _, row := range completed {
		byDay[row.CompletedAt.UTC().Format(core.StatsDayLayout)]++
	}
	out := make([]core.StatsDay, 0, len(byDay))
	for date, n := range byDay {
		out = append(out, core.StatsDay{Date: date, Completed: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}

// statsLeadTimes reports the median and the worst creation-to-terminal time.
// A negative interval is impossible from the domain and would only come from a
// clock that moved backwards, so it is clamped rather than reported.
func statsLeadTimes(completed []store.CompletedTask) (median, slowest core.Duration) {
	if len(completed) == 0 {
		return 0, 0
	}
	leads := make([]time.Duration, 0, len(completed))
	for _, row := range completed {
		lead := row.CompletedAt.Sub(row.CreatedAt)
		if lead < 0 {
			lead = 0
		}
		leads = append(leads, lead)
		if core.Duration(lead) > slowest {
			slowest = core.Duration(lead)
		}
	}
	sort.Slice(leads, func(i, j int) bool { return leads[i] < leads[j] })
	mid := len(leads) / 2
	if len(leads)%2 == 1 {
		return core.Duration(leads[mid]), slowest
	}
	return core.Duration((leads[mid-1] + leads[mid]) / 2), slowest
}

// statsByCategory folds the per-status counts into the three categories, always
// reporting all three: a board with nothing in progress is a fact worth showing,
// and a category that disappeared when it emptied would read as a missing figure.
func statsByCategory(counts map[string]int, categories map[string]core.StateCategory) []core.StatsCategory {
	totals := map[core.StateCategory]int{}
	for status, n := range counts {
		totals[categories[status]] += n
	}
	out := make([]core.StatsCategory, 0, len(statsCategoryOrder))
	for _, category := range statsCategoryOrder {
		out = append(out, core.StatsCategory{Category: category, Count: totals[category]})
	}
	return out
}

// statsTopActors ranks the actors by the terminal moves attributed to them.
// Ties break on the identifier so two reads of the same data agree.
func statsTopActors(completed []store.CompletedTask, top int) []core.StatsActor {
	moved := map[string]int{}
	for _, row := range completed {
		if row.ActorID == "" {
			continue
		}
		moved[row.ActorID]++
	}
	out := make([]core.StatsActor, 0, len(moved))
	for actorID, n := range moved {
		out = append(out, core.StatsActor{ActorID: actorID, Moved: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Moved != out[j].Moved {
			return out[i].Moved > out[j].Moved
		}
		return out[i].ActorID < out[j].ActorID
	})
	if len(out) > top {
		out = out[:top]
	}
	return out
}

// statsAgeing reports how long each open task has been waiting.
func statsAgeing(open []core.Task, until time.Time) []core.StatsAgeing {
	out := make([]core.StatsAgeing, 0, len(open))
	for _, task := range open {
		age := until.Sub(task.CreatedAt)
		if age < 0 {
			age = 0
		}
		out = append(out, core.StatsAgeing{
			Ref: task.Ref, Title: task.Title, Status: task.Status,
			CreatedAt: task.CreatedAt, Age: core.Duration(age),
		})
	}
	return out
}

// nameActors puts a handle beside each identifier on the leaderboard. An actor
// that has since been removed keeps its identifier and loses only its name.
func nameActors(ctx context.Context, tx store.Tx, actors []core.StatsActor) error {
	for i := range actors {
		actor, err := tx.GetActor(ctx, actors[i].ActorID)
		if err != nil {
			if core.IsKind(err, core.KindNotFound) {
				continue
			}
			return err
		}
		actors[i].Handle = actor.Handle
	}
	return nil
}
