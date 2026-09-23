// SPDX-License-Identifier: AGPL-3.0-or-later

package query

import (
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// Matches reports whether a task satisfies the filter, answering in memory
// what the store answers in SQL. The terminal interface filters an already
// loaded page with it, so the two must agree: a task the store would have
// returned must match here too.
func Matches(f core.TaskFilter, t core.Task, projectKey string, now time.Time) bool {
	if t.DeletedAt != nil && !f.IncludeDeleted {
		return false
	}
	if !matchIncludes(f, t, projectKey) || !matchExcludes(f.Exclude, t, projectKey) {
		return false
	}
	if !matchDue(f, t) || !matchParent(f, t) {
		return false
	}
	if !f.Claimed.Match(t.ClaimedAtTime(now)) || !f.Blocked.Match(t.Blocked) {
		return false
	}
	for _, term := range f.Text {
		if !term.Matches(t.Title, t.Body) {
			return false
		}
	}
	return matchQuery(f.Query, t)
}

// matchIncludes checks the selecting list terms.
func matchIncludes(f core.TaskFilter, t core.Task, projectKey string) bool {
	if len(f.ProjectKeys) > 0 && !containsFold(f.ProjectKeys, projectKey) {
		return false
	}
	if len(f.Statuses) > 0 && !containsFold(f.Statuses, t.Status) {
		return false
	}
	if len(f.AssigneeIDs) > 0 && !containsFold(f.AssigneeIDs, t.AssigneeActorID) {
		return false
	}
	if len(f.CreatorIDs) > 0 && !containsFold(f.CreatorIDs, t.CreatorActorID) {
		return false
	}
	if len(f.ClaimedBy) > 0 && !containsFold(f.ClaimedBy, t.ClaimedByActorID) {
		return false
	}
	return matchTags(f.Tags, t.Tags) && matchPriorities(f.Priorities, t.Priority)
}

// matchExcludes drops a task naming any excluded value.
func matchExcludes(e core.TaskExclude, t core.Task, projectKey string) bool {
	if containsFold(e.ProjectKeys, projectKey) || containsFold(e.Statuses, t.Status) {
		return false
	}
	if containsFold(e.AssigneeIDs, t.AssigneeActorID) || containsFold(e.CreatorIDs, t.CreatorActorID) {
		return false
	}
	if containsFold(e.ClaimedBy, t.ClaimedByActorID) {
		return false
	}
	if len(e.Tags) > 0 && matchTags(e.Tags, t.Tags) {
		return false
	}
	return len(e.Priorities) == 0 || !matchPriorities(e.Priorities, t.Priority)
}

// matchTags reports whether the task carries any of the wanted tags.
func matchTags(wanted, have []string) bool {
	if len(wanted) == 0 {
		return true
	}
	for _, w := range wanted {
		if containsFold(have, w) {
			return true
		}
	}
	return false
}

// matchPriorities reports whether the priority is among those wanted.
func matchPriorities(wanted []core.Priority, p core.Priority) bool {
	if len(wanted) == 0 {
		return true
	}
	for _, w := range wanted {
		if w == p {
			return true
		}
	}
	return false
}

// matchDue reports whether the task falls inside the due bounds.
func matchDue(f core.TaskFilter, t core.Task) bool {
	if f.DueBefore == nil && f.DueAfter == nil {
		return true
	}
	if t.DueAt == nil {
		return false
	}
	if f.DueBefore != nil && !t.DueAt.Before(*f.DueBefore) {
		return false
	}
	return f.DueAfter == nil || t.DueAt.After(*f.DueAfter)
}

// matchParent reports whether the task satisfies the parent terms.
func matchParent(f core.TaskFilter, t core.Task) bool {
	if f.ParentIsNull {
		return t.ParentID == ""
	}
	return f.ParentID == "" || f.ParentID == t.ParentID
}

// matchQuery reports whether the free text appears in the task.
func matchQuery(query string, t core.Task) bool {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return true
	}
	for _, field := range []string{t.Title, t.Body, t.Ref} {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return false
}

// containsFold reports whether haystack holds needle, ignoring case.
func containsFold(haystack []string, needle string) bool {
	for _, s := range haystack {
		if strings.EqualFold(s, needle) {
			return true
		}
	}
	return false
}
