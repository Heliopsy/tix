// Package tui implements the tix terminal interface over the core service.
package tui

import (
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// FilterDateLayouts are the timestamp forms a filter term may take.
var FilterDateLayouts = []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04", "2006-01-02"}

// FilterKeys are the term prefixes the filter bar accepts.
var FilterKeys = []string{
	"project", "status", "tag", "assignee", "creator", "claimed-by",
	"priority", "due-before", "due-after", "parent", "is", "sort", "limit", "text",
}

// token is one lexed word of a filter expression.
type token struct {
	text   string
	quoted bool
}

// ParseFilter turns a filter expression into the TaskFilter the CLI builds.
func ParseFilter(expr string) (core.TaskFilter, error) {
	var f core.TaskFilter
	tokens, err := tokenize(expr)
	if err != nil {
		return core.TaskFilter{}, err
	}
	words := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		key, value, ok := strings.Cut(tok.text, ":")
		if !ok || tok.quoted {
			words = append(words, tok.text)
			continue
		}
		if err := applyTerm(&f, strings.ToLower(strings.TrimSpace(key)), value); err != nil {
			return core.TaskFilter{}, err
		}
	}
	f.Query = strings.Join(words, " ")
	return f.Validate()
}

// tokenize splits an expression on spaces, honouring double quotes.
func tokenize(expr string) ([]token, error) {
	var out []token
	var cur strings.Builder
	quoted, open := false, false
	for _, r := range expr {
		switch {
		case r == '"':
			quoted, open = true, !open
		case r == ' ' && !open:
			if cur.Len() > 0 || quoted {
				out = append(out, token{text: cur.String(), quoted: quoted})
				cur.Reset()
				quoted = false
			}
		default:
			cur.WriteRune(r)
		}
	}
	if open {
		return nil, core.Invalid("filter has an unterminated quote")
	}
	if cur.Len() > 0 || quoted {
		out = append(out, token{text: cur.String(), quoted: quoted})
	}
	return out, nil
}

// applyTerm folds one key:value term into the filter.
func applyTerm(f *core.TaskFilter, key, value string) error {
	value = strings.TrimSpace(strings.Trim(value, `"`))
	if value == "" {
		return core.Invalid("filter term %q has no value", key)
	}
	switch key {
	case "project", "p":
		f.ProjectKeys = append(f.ProjectKeys, strings.ToLower(value))
	case "status", "s":
		f.Statuses = append(f.Statuses, value)
	case "tag":
		f.Tags = append(f.Tags, value)
	case "assignee", "a":
		f.AssigneeIDs = append(f.AssigneeIDs, value)
	case "creator":
		f.CreatorIDs = append(f.CreatorIDs, value)
	case "claimed-by":
		f.ClaimedBy = append(f.ClaimedBy, value)
	case "priority", "prio":
		return applyPriority(f, value)
	case "due-before":
		return applyDue(&f.DueBefore, value)
	case "due-after":
		return applyDue(&f.DueAfter, value)
	case "parent":
		return applyParent(f, value)
	case "is":
		return applyIs(f, strings.ToLower(value))
	case "sort":
		f.Page.Sort = value
	case "limit":
		return applyLimit(f, value)
	case "text", "q":
		f.Query = strings.TrimSpace(f.Query + " " + value)
	default:
		return core.Invalid("unknown filter key %q; try one of %s", key, strings.Join(FilterKeys, ", "))
	}
	return nil
}

// applyPriority parses a priority name or number into the filter.
func applyPriority(f *core.TaskFilter, value string) error {
	p, err := ParsePriority(value)
	if err != nil {
		return err
	}
	f.Priorities = append(f.Priorities, p)
	return nil
}

// applyDue parses a date term into a bound.
func applyDue(dst **time.Time, value string) error {
	t, err := ParseFilterTime(value)
	if err != nil {
		return err
	}
	*dst = t
	return nil
}

// applyParent selects a parent, or the roots.
func applyParent(f *core.TaskFilter, value string) error {
	if strings.EqualFold(value, "none") {
		f.ParentIsNull = true
		return nil
	}
	f.ParentID = value
	return nil
}

// applyIs folds a boolean term into the filter.
func applyIs(f *core.TaskFilter, value string) error {
	switch value {
	case "claimed":
		f.Claimed = core.Yes
	case "unclaimed", "free":
		f.Claimed = core.No
	case "blocked":
		f.Blocked = core.Yes
	case "unblocked", "ready":
		f.Blocked = core.No
	case "deleted":
		f.IncludeDeleted = true
	case "root":
		f.ParentIsNull = true
	default:
		return core.Invalid("unknown is: value %q; try claimed, unclaimed, blocked, unblocked, deleted or root", value)
	}
	return nil
}

// applyLimit parses a page limit term.
func applyLimit(f *core.TaskFilter, value string) error {
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return core.Invalid("limit %q must be a non-negative number", value)
	}
	f.Page.Limit = n
	return nil
}

// ParsePriority accepts a priority name or a number between 1 and 5.
func ParsePriority(value string) (core.Priority, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "highest":
		return core.PriorityHighest, nil
	case "high":
		return core.PriorityHigh, nil
	case "normal", "medium":
		return core.PriorityNormal, nil
	case "low":
		return core.PriorityLow, nil
	case "lowest":
		return core.PriorityLowest, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, core.Invalid("priority %q must be a name or a number from 1 to 5", value)
	}
	p := core.Priority(n)
	if !p.Valid() {
		return 0, core.Invalid("priority %d is out of range", n)
	}
	return p, nil
}

// ParseFilterTime accepts a date or a timestamp.
func ParseFilterTime(value string) (*time.Time, error) {
	trimmed := strings.TrimSpace(value)
	for _, layout := range FilterDateLayouts {
		if t, err := time.Parse(layout, trimmed); err == nil {
			utc := t.UTC()
			return &utc, nil
		}
	}
	return nil, core.Invalid("time %q must be RFC3339 or YYYY-MM-DD", value)
}

// MatchesFilter reports whether a task satisfies the task-level filter terms.
func MatchesFilter(f core.TaskFilter, t core.Task, projectKey string, now time.Time) bool {
	if t.DeletedAt != nil && !f.IncludeDeleted {
		return false
	}
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
	if !matchTags(f.Tags, t.Tags) || !matchPriorities(f.Priorities, t.Priority) {
		return false
	}
	if !matchDue(f, t) || !matchParent(f, t) {
		return false
	}
	if !f.Claimed.Match(t.ClaimedAtTime(now)) || !f.Blocked.Match(t.Blocked) {
		return false
	}
	return matchQuery(f.Query, t)
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
