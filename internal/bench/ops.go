package bench

import (
	"context"
	"fmt"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// PageLimit is the page size every benchmarked listing asks for.
const PageLimit = 50

// FilteredPage reads one filtered, keyset-paginated page of the primary
// tenant's tasks. An empty cursor reads the first page.
func (f *Fixture) FilteredPage(ctx context.Context, cursor string, limit int) (int, error) {
	t := f.Primary()
	filter := core.TaskFilter{
		Statuses:   []string{"todo", "doing", "review"},
		Priorities: []core.Priority{core.PriorityHighest, core.PriorityHigh, core.PriorityNormal},
		Page: core.Page{
			Limit:     limit,
			Cursor:    cursor,
			Sort:      core.SortCreatedAt,
			Direction: core.Ascending,
		},
	}
	return f.count(ctx, t.Scope, filter)
}

// ProjectPage reads one page of a single project, the shape a board column has.
func (f *Fixture) ProjectPage(ctx context.Context, projectID, status string, limit int) (int, error) {
	t := f.Primary()
	filter := core.TaskFilter{
		ProjectIDs: []string{projectID},
		Statuses:   []string{status},
		Page:       core.Page{Limit: limit, Sort: core.SortPriority, Direction: core.Ascending},
	}
	return f.count(ctx, t.Scope, filter)
}

// Board reads one column per workflow state for a single project, which is what
// the grouped view costs.
func (f *Fixture) Board(ctx context.Context, projectID string, limit int) (int, error) {
	total := 0
	for _, status := range Statuses {
		n, err := f.ProjectPage(ctx, projectID, status, limit)
		if err != nil {
			return 0, err
		}
		total += n
	}
	return total, nil
}

// Search reads one page matching a body substring.
func (f *Fixture) Search(ctx context.Context, term string, limit int) (int, error) {
	t := f.Primary()
	filter := core.TaskFilter{
		Query: term,
		Page:  core.Page{Limit: limit, Sort: core.SortCreatedAt, Direction: core.Ascending},
	}
	return f.count(ctx, t.Scope, filter)
}

// UnblockedPage reads the tasks no unfinished dependency blocks, the predicate
// ClaimNext shares.
func (f *Fixture) UnblockedPage(ctx context.Context, limit int) (int, error) {
	t := f.Primary()
	filter := core.TaskFilter{
		Statuses: []string{"todo"},
		Blocked:  core.No,
		Claimed:  core.No,
		Page:     core.Page{Limit: limit, Sort: core.SortPriority, Direction: core.Ascending},
	}
	return f.count(ctx, t.Scope, filter)
}

func (f *Fixture) count(ctx context.Context, scope core.TenantScope, filter core.TaskFilter) (int, error) {
	n := 0
	err := f.Store.View(ctx, scope, func(tx store.Tx) error {
		tasks, err := tx.ListTasks(ctx, filter)
		if err != nil {
			return err
		}
		n = len(tasks)
		return nil
	})
	return n, err
}

// ClaimNext takes the highest-priority unblocked task and immediately gives it
// back, so the fixture stays reusable across iterations.
func (f *Fixture) ClaimNext(ctx context.Context, token string) (bool, error) {
	t := f.Primary()
	now := time.Now().UTC()
	claimed := false
	err := f.Store.Update(ctx, t.Scope, func(tx store.Tx) error {
		id, ok, err := tx.ClaimNextTask(ctx, store.ClaimNextRow{
			Statuses:       []string{"todo"},
			TerminalStates: TerminalStatuses,
			ActorID:        t.Actor.ID,
			Now:            now,
			Until:          now.Add(time.Minute),
			LeaseToken:     token,
		})
		if err != nil {
			return err
		}
		claimed = ok
		if !ok {
			return nil
		}
		if _, err := tx.ReleaseLease(ctx, id, token); err != nil {
			return fmt.Errorf("releasing the lease just taken on %q: %w", id, err)
		}
		return nil
	})
	return claimed, err
}
