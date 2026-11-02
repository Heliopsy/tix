// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"time"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// housekeepingBatch is how many tasks one run acts on per rule by default.
const housekeepingBatch = 200

// housekeepingDay is the unit every rule is written in.
const housekeepingDay = 24 * time.Hour

// housekeeping is the stored policy and what it applies to.
type housekeeping struct {
	tenant    core.HousekeepingPolicy
	overrides map[string]*core.HousekeepingPolicy
	paused    bool
	project   *core.Project
	projects  []core.Project
}

// housekeepingTarget is one rule applied to one project.
type housekeepingTarget struct {
	rule     core.HousekeepingRule
	project  core.Project
	days     int
	before   time.Time
	finished store.FinishedQuery
}

// loadHousekeeping reads the policy and the projects it covers: the one named,
// or every project when any rule could be on.
func loadHousekeeping(ctx context.Context, tx store.Tx, ref string) (*housekeeping, error) {
	rows, err := tx.HousekeepingPolicies(ctx)
	if err != nil {
		return nil, err
	}
	h := &housekeeping{overrides: map[string]*core.HousekeepingPolicy{}}
	anyOn := false
	for i := range rows {
		p := rows[i]
		if p.ProjectID == "" {
			h.tenant = p
		} else {
			h.overrides[p.ProjectID] = &p
		}
		for _, r := range core.HousekeepingRules() {
			anyOn = anyOn || p.Days(r) > 0
		}
	}
	if h.paused, err = trashOff(ctx, tx); err != nil {
		return nil, err
	}
	if ref != "" {
		if h.project, err = lookupProject(ctx, tx, ref); err != nil {
			return nil, err
		}
		h.projects = []core.Project{*h.project}
		return h, nil
	}
	if anyOn {
		h.projects, err = listAllProjects(ctx, tx)
	}
	return h, err
}

// view describes the policy at the level asked about.
func (h *housekeeping) view() *core.HousekeepingView {
	v := &core.HousekeepingView{Tenant: h.tenant, Paused: h.paused}
	if h.project == nil {
		v.Effective = h.tenant.Resolve(nil)
		return v
	}
	v.Project = h.project.Key
	v.Override = h.overrides[h.project.ID]
	if v.Override == nil {
		v.Override = &core.HousekeepingPolicy{ProjectID: h.project.ID}
	}
	v.Effective = h.tenant.Resolve(v.Override)
	return v
}

// targets lists every rule that is on, per project. Nothing runs while the
// trash is off: a finished task moved to it would be deleted for good, and
// the trash setting promises to keep what it held until it is back on.
func (h *housekeeping) targets(ctx context.Context, tx store.Tx, now time.Time) ([]housekeepingTarget, error) {
	if h.paused {
		return nil, nil
	}
	var out []housekeepingTarget
	workflows := map[string]*core.Workflow{}
	for _, p := range h.projects {
		eff := h.tenant.Resolve(h.overrides[p.ID])
		for _, r := range core.HousekeepingRules() {
			days := eff.Days(r)
			if days <= 0 {
				continue
			}
			t := housekeepingTarget{rule: r, project: p, days: days, before: now.Add(-time.Duration(days) * housekeepingDay)}
			if r != core.RuleTrash {
				wf, ok := workflows[p.WorkflowID]
				if !ok {
					loaded, err := tx.GetWorkflowByID(ctx, p.WorkflowID)
					if err != nil {
						return nil, err
					}
					wf, workflows[p.WorkflowID] = loaded, loaded
				}
				t.finished = store.FinishedQuery{ProjectID: p.ID, Statuses: finishedStates(wf.Definition, r), Before: t.before}
			}
			out = append(out, t)
		}
	}
	return out, nil
}

// finishedStates returns the terminal states of the rule's category.
func finishedStates(d core.WorkflowDefinition, r core.HousekeepingRule) []string {
	want := core.CategoryDone
	if r == core.RuleCancelled {
		want = core.CategoryCancelled
	}
	var out []string
	for _, key := range d.TerminalStates() {
		if d.CategoryOf(key) == want {
			out = append(out, key)
		}
	}
	return out
}

// count reports how many tasks a target would act on now.
func (t housekeepingTarget) count(ctx context.Context, tx store.Tx) (int, error) {
	if t.rule == core.RuleTrash {
		return tx.CountExpiredTrash(ctx, t.project.ID, t.before)
	}
	return tx.CountFinishedTasks(ctx, t.finished)
}

// pending counts what every target would act on now.
func pending(ctx context.Context, tx store.Tx, targets []housekeepingTarget) (core.HousekeepingCounts, error) {
	var c core.HousekeepingCounts
	for _, t := range targets {
		n, err := t.count(ctx, tx)
		if err != nil {
			return c, err
		}
		c.Add(t.rule, n)
	}
	return c, nil
}

// GetHousekeeping returns the housekeeping policy and what it would do now.
func (l *Local) GetHousekeeping(ctx context.Context, project string) (*core.HousekeepingView, error) {
	actor, err := l.authorize(ctx, authz.ActionTenantAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out *core.HousekeepingView
	err = l.read(ctx, actor, func(tx store.Tx) error {
		h, err := loadHousekeeping(ctx, tx, project)
		if err != nil {
			return err
		}
		out, err = h.viewWithPending(ctx, tx, l.clock.Now())
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// viewWithPending is the view and its counts.
func (h *housekeeping) viewWithPending(ctx context.Context, tx store.Tx, now time.Time) (*core.HousekeepingView, error) {
	v := h.view()
	targets, err := h.targets(ctx, tx, now)
	if err != nil {
		return nil, err
	}
	v.Pending, err = pending(ctx, tx, targets)
	return v, err
}

// PutHousekeeping changes the tenant default, or one project's override when
// the input names a project. Turning on a finished-task rule is refused while
// the trash is off, because those tasks would be deleted for good.
func (l *Local) PutHousekeeping(ctx context.Context, in core.HousekeepingInput) (*core.HousekeepingView, error) {
	actor, err := l.authorize(ctx, authz.ActionRetentionWrite, authz.Resource{})
	if err != nil {
		return nil, err
	}
	isProject := in.Project != ""
	given := map[core.HousekeepingRule]*int{}
	for _, r := range core.HousekeepingRules() {
		raw := in.Value(r)
		if raw == "" {
			continue
		}
		days, err := core.ParseHousekeepingValue(r, raw, isProject)
		if err != nil {
			return nil, err
		}
		given[r] = days
	}
	if len(given) == 0 {
		return nil, core.Invalid("at least one of done, cancelled and trash is required")
	}
	var out *core.HousekeepingView
	err = l.write(ctx, actor, func(m *mutation) error {
		h, err := loadHousekeeping(ctx, m.tx, in.Project)
		if err != nil {
			return err
		}
		level, subject, id := h.tenant, "tenant", actor.TenantID
		if h.project != nil {
			level = core.HousekeepingPolicy{ProjectID: h.project.ID}
			if o := h.overrides[h.project.ID]; o != nil {
				level = *o
			}
			subject, id = "project", h.project.ID
		}
		before := level
		for r, days := range given {
			if h.paused && r != core.RuleTrash && days != nil && *days > 0 {
				return core.TrashIsOff("finished tasks cannot be moved to it; housekeeping would delete them for good")
			}
			level.SetRule(r, days)
		}
		if err := m.tx.PutHousekeepingPolicy(ctx, &level); err != nil {
			return err
		}
		if h.project != nil {
			h.overrides[h.project.ID] = &level
		} else {
			h.tenant = level
		}
		if h.project == nil && len(h.projects) == 0 {
			if h.projects, err = listAllProjects(ctx, m.tx); err != nil {
				return err
			}
		}
		if err := m.Audit("housekeeping.update", subject, id, before, level); err != nil {
			return err
		}
		out, err = h.viewWithPending(ctx, m.tx, m.now)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RunHousekeeping applies the policy once, acting on at most Limit tasks per
// rule, or previews it. A task another worker acted on first is skipped
// without a record, so replicas running side by side never double up.
func (l *Local) RunHousekeeping(ctx context.Context, in core.HousekeepingRunInput) (*core.HousekeepingResult, error) {
	actor, err := l.authorize(ctx, authz.ActionRetentionWrite, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if in.Limit < 0 {
		return nil, core.Invalid("housekeeping limit must not be negative")
	}
	if in.Limit == 0 {
		in.Limit = housekeepingBatch
	}
	res := &core.HousekeepingResult{DryRun: in.DryRun}
	if in.DryRun {
		err = l.read(ctx, actor, func(tx store.Tx) error {
			h, err := loadHousekeeping(ctx, tx, in.Project)
			if err != nil {
				return err
			}
			res.Paused = h.paused
			targets, err := h.targets(ctx, tx, l.clock.Now())
			if err != nil {
				return err
			}
			res.HousekeepingCounts, err = pending(ctx, tx, targets)
			return err
		})
		if err != nil {
			return nil, err
		}
		return res, nil
	}
	err = l.write(ctx, actor, func(m *mutation) error {
		h, err := loadHousekeeping(ctx, m.tx, in.Project)
		if err != nil {
			return err
		}
		res.Paused = h.paused
		targets, err := h.targets(ctx, m.tx, m.now)
		if err != nil {
			return err
		}
		for _, t := range targets {
			left := in.Limit - res.Of(t.rule)
			if left <= 0 {
				continue
			}
			n, err := l.applyTarget(ctx, m, t, left)
			if err != nil {
				return err
			}
			res.Add(t.rule, n)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// applyTarget acts on up to limit tasks of one target.
func (l *Local) applyTarget(ctx context.Context, m *mutation, t housekeepingTarget, limit int) (int, error) {
	if t.rule == core.RuleTrash {
		return purgeExpired(ctx, m, t, limit)
	}
	tasks, err := m.tx.FinishedTasks(ctx, t.finished, limit)
	if err != nil {
		return 0, err
	}
	n := 0
	for i := range tasks {
		task := &tasks[i]
		ok, err := m.tx.TrashFinished(ctx, task.ID, t.finished)
		if err != nil {
			return n, err
		}
		if !ok {
			continue
		}
		n++
		if err := recordCleanup(m, task, t, false); err != nil {
			return n, err
		}
	}
	return n, nil
}

// purgeExpired permanently deletes expired trash, each task after its own
// expired descendants.
func purgeExpired(ctx context.Context, m *mutation, t housekeepingTarget, limit int) (int, error) {
	roots, err := m.tx.ExpiredTrash(ctx, t.project.ID, t.before, limit)
	if err != nil {
		return 0, err
	}
	n := 0
	for i := range roots {
		if n >= limit {
			break
		}
		subtree, err := trashedSubtree(ctx, m.tx, roots[i], t.before)
		if err != nil {
			return n, err
		}
		for j := range subtree {
			task := &subtree[j]
			ok, err := m.tx.PurgeTrashed(ctx, task.ID, t.before)
			if err != nil {
				return n, err
			}
			if !ok {
				continue
			}
			n++
			if err := recordCleanup(m, task, t, true); err != nil {
				return n, err
			}
		}
	}
	return n, nil
}

// trashedSubtree returns a trashed task and its expired trashed descendants,
// deepest first, bounded like a cascading delete.
func trashedSubtree(ctx context.Context, tx store.Tx, root core.Task, before time.Time) ([]core.Task, error) {
	out := []core.Task{root}
	seen := map[string]bool{root.ID: true}
	for i := 0; i < len(out) && len(out) < maxSubtreeSize; i++ {
		children, err := tx.TrashedChildren(ctx, out[i].ID, before)
		if err != nil {
			return nil, err
		}
		for _, c := range children {
			if !seen[c.ID] {
				seen[c.ID] = true
				out = append(out, c)
			}
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// recordCleanup writes the audit entry and event a delete by hand would.
func recordCleanup(m *mutation, task *core.Task, t housekeepingTarget, hard bool) error {
	return m.Record("task.delete", core.EventTaskDeleted, "task", task.ID, task.ProjectID,
		task, nil, map[string]any{"ref": task.Ref, "hard": hard, "cleanup": string(t.rule), "after_days": t.days})
}
