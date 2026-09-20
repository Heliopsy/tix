package web

import (
	"strconv"
	"strings"
	"time"

	"github.com/thereisnotime/tix/internal/core"
)

// filterKeys are the terms a task filter expression accepts. Each one names the
// CLI flag with the same meaning, so an expression and a command line select
// the same tasks.
var filterKeys = []string{
	"project", "status", "tag", "assignee", "creator", "priority",
	"claimed", "blocked", "parent", "deleted", "due-before", "due-after",
}

// filterTimeLayouts are the timestamp forms a filter term may take.
var filterTimeLayouts = []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"}

// ParseFilter reads a task filter expression into the filter the service takes.
// The grammar is the CLI's: space separated "key:value" terms, with bare words
// matching title and body text.
func ParseFilter(expression string) (core.TaskFilter, error) {
	var filter core.TaskFilter
	terms, err := splitTerms(expression)
	if err != nil {
		return filter, err
	}
	var words []string
	for _, term := range terms {
		key, value, found := strings.Cut(term, ":")
		if !found {
			words = append(words, term)
			continue
		}
		if err := applyTerm(&filter, strings.ToLower(strings.TrimSpace(key)), strings.TrimSpace(value)); err != nil {
			return core.TaskFilter{}, err
		}
	}
	filter.Query = strings.Join(words, " ")
	return filter, nil
}

// splitTerms splits an expression on spaces, keeping quoted values whole.
func splitTerms(expression string) ([]string, error) {
	var (
		terms   []string
		current strings.Builder
		quoted  bool
	)
	for _, r := range expression {
		switch {
		case r == '"':
			quoted = !quoted
		case r == ' ' && !quoted:
			if current.Len() > 0 {
				terms = append(terms, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if quoted {
		return nil, core.Invalid("filter has an unclosed quote")
	}
	if current.Len() > 0 {
		terms = append(terms, current.String())
	}
	return terms, nil
}

// applyTerm folds one parsed term into the filter.
func applyTerm(filter *core.TaskFilter, key, value string) error {
	if value == "" {
		return core.Invalid("filter term %q has no value", key)
	}
	switch key {
	case "project":
		filter.ProjectKeys = append(filter.ProjectKeys, value)
	case "status":
		filter.Statuses = append(filter.Statuses, value)
	case "tag":
		filter.Tags = append(filter.Tags, value)
	case "assignee":
		filter.AssigneeIDs = append(filter.AssigneeIDs, value)
	case "creator":
		filter.CreatorIDs = append(filter.CreatorIDs, value)
	case "parent":
		filter.ParentID = value
	case "priority":
		return applyPriority(filter, value)
	case "claimed":
		return applyTriState(&filter.Claimed, key, value)
	case "blocked":
		return applyTriState(&filter.Blocked, key, value)
	case "deleted":
		include, err := parseFilterBool(key, value)
		if err != nil {
			return err
		}
		filter.IncludeDeleted = include
	case "due-before":
		return applyTime(&filter.DueBefore, key, value)
	case "due-after":
		return applyTime(&filter.DueAfter, key, value)
	default:
		return core.Invalid("filter term %q is not one of %s", key, strings.Join(filterKeys, ", "))
	}
	return nil
}

// applyPriority folds a priority term into the filter.
func applyPriority(filter *core.TaskFilter, value string) error {
	n, err := strconv.Atoi(value)
	if err != nil {
		return core.Invalid("filter priority %q must be a number from 1 to 5", value)
	}
	p := core.Priority(n)
	if !p.Valid() {
		return core.Invalid("filter priority %d is out of range", n)
	}
	filter.Priorities = append(filter.Priorities, p)
	return nil
}

// applyTriState folds a yes or no term into the filter.
func applyTriState(target *core.TriState, key, value string) error {
	yes, err := parseFilterBool(key, value)
	if err != nil {
		return err
	}
	if yes {
		*target = core.Yes
		return nil
	}
	*target = core.No
	return nil
}

// applyTime folds a timestamp term into the filter.
func applyTime(target **time.Time, key, value string) error {
	for _, layout := range filterTimeLayouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			utc := parsed.UTC()
			*target = &utc
			return nil
		}
	}
	return core.Invalid("filter term %q takes a date such as 2006-01-02", key)
}

// parseFilterBool reads a yes or no filter value.
func parseFilterBool(key, value string) (bool, error) {
	switch strings.ToLower(value) {
	case "yes", "true", "1":
		return true, nil
	case "no", "false", "0":
		return false, nil
	default:
		return false, core.Invalid("filter term %q takes yes or no, not %q", key, value)
	}
}
