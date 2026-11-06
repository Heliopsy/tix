// SPDX-License-Identifier: AGPL-3.0-or-later

package taskio

import (
	"sort"

	"github.com/heliopsy/tix/internal/core"
)

// StateInfo describes a workflow state of the target project.
type StateInfo struct {
	Key      string
	Label    string
	Category core.StateCategory
	Initial  bool
}

var statusWords = foldKeys(map[string]core.StateCategory{
	"todo": core.CategoryTodo, "to do": core.CategoryTodo, "open": core.CategoryTodo,
	"new": core.CategoryTodo, "backlog": core.CategoryTodo, "selected for development": core.CategoryTodo,
	"triage": core.CategoryTodo, "reopened": core.CategoryTodo, "unstarted": core.CategoryTodo,
	"Нова": core.CategoryTodo, "Нови": core.CategoryTodo, "За правене": core.CategoryTodo,
	"Отворена":    core.CategoryTodo,
	"in progress": core.CategoryInProgress, "doing": core.CategoryInProgress,
	"started": core.CategoryInProgress, "active": core.CategoryInProgress,
	"in review": core.CategoryInProgress, "review": core.CategoryInProgress,
	"code review": core.CategoryInProgress, "in development": core.CategoryInProgress,
	"В процес": core.CategoryInProgress, "В прогрес": core.CategoryInProgress,
	"Работи се": core.CategoryInProgress,
	"blocked":   core.CategoryBlocked, "on hold": core.CategoryBlocked, "Блокирана": core.CategoryBlocked,
	"waiting": core.CategoryWaiting, "waiting for customer": core.CategoryWaiting,
	"Чака": core.CategoryWaiting, "Изчакване": core.CategoryWaiting,
	"done": core.CategoryDone, "closed": core.CategoryDone, "resolved": core.CategoryDone,
	"complete": core.CategoryDone, "completed": core.CategoryDone, "fixed": core.CategoryDone,
	"Готово": core.CategoryDone, "Готова": core.CategoryDone, "Завършена": core.CategoryDone,
	"Затворена": core.CategoryDone,
	"cancelled": core.CategoryCancelled, "canceled": core.CategoryCancelled,
	"won't do": core.CategoryCancelled, "wontfix": core.CategoryCancelled,
	"won't fix": core.CategoryCancelled, "duplicate": core.CategoryCancelled,
	"declined": core.CategoryCancelled, "rejected": core.CategoryCancelled,
	"Отказана": core.CategoryCancelled, "Отменена": core.CategoryCancelled,
})

// SuggestStatus proposes the workflow state key a source status means, empty when nothing fits.
func SuggestStatus(value string, states []StateInfo) string {
	f := fold(value)
	if f == "" {
		return ""
	}
	for _, s := range states {
		if f == fold(s.Key) || (s.Label != "" && f == fold(s.Label)) {
			return s.Key
		}
	}
	cat, ok := statusWords[f]
	if !ok {
		return ""
	}
	if cat == core.CategoryTodo {
		for _, s := range states {
			if s.Initial {
				return s.Key
			}
		}
	}
	for _, s := range states {
		if s.Category == cat {
			return s.Key
		}
	}
	return ""
}

// DistinctCap is how many distinct values Distinct keeps per target.
const DistinctCap = 200

// ValueCount is one distinct source value and how many rows carried it.
type ValueCount struct {
	Value string
	Count int
}

// Distinct collects the distinct non-empty values seen per target, for the value-mapping step.
type Distinct struct {
	counts    map[Target]map[string]int
	truncated map[Target]bool
}

// Add counts one value under a target.
func (d *Distinct) Add(target Target, value string) {
	if value == "" {
		return
	}
	if d.counts == nil {
		d.counts = map[Target]map[string]int{}
		d.truncated = map[Target]bool{}
	}
	seen := d.counts[target]
	if seen == nil {
		seen = map[string]int{}
		d.counts[target] = seen
	}
	if _, ok := seen[value]; !ok && len(seen) >= DistinctCap {
		d.truncated[target] = true
		return
	}
	seen[value]++
}

// Truncated reports whether a target had more distinct values than Distinct keeps.
func (d *Distinct) Truncated(target Target) bool { return d.truncated[target] }

// Values lists a target's distinct values, most frequent first, then by value.
func (d *Distinct) Values(target Target) []ValueCount {
	out := make([]ValueCount, 0, len(d.counts[target]))
	for v, n := range d.counts[target] {
		out = append(out, ValueCount{v, n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Value < out[j].Value
	})
	return out
}

// AddDraft counts a draft's raw status, priority, assignee and tags.
func (d *Distinct) AddDraft(dr Draft) {
	d.Add(TargetStatus, dr.StatusRaw)
	d.Add(TargetPriority, dr.PriorityRaw)
	d.Add(TargetAssignee, dr.AssigneeRaw)
	for _, t := range dr.Tags {
		d.Add(TargetTags, t)
	}
}
