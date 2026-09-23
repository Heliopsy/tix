// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tui implements the tix terminal interface over the core service.
package tui

import (
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/query"
)

// FilterDateLayouts are the timestamp forms a filter term may take.
var FilterDateLayouts = query.DateLayouts

// FilterKeys are the term prefixes the filter bar accepts.
var FilterKeys = query.Keys

// ParseFilter turns a filter expression into the TaskFilter the CLI builds.
// It is the shared parser, so the filter bar and `tix task ls --filter`
// cannot drift apart.
func ParseFilter(expr string) (core.TaskFilter, error) { return query.Parse(expr) }

// ParsePriority accepts a priority name or a number between 1 and 5.
func ParsePriority(value string) (core.Priority, error) { return query.ParsePriority(value) }

// ParseFilterTime accepts a date or a timestamp.
func ParseFilterTime(value string) (*time.Time, error) { return query.ParseTime(value) }

// MatchesFilter reports whether a task satisfies the task-level filter terms.
func MatchesFilter(f core.TaskFilter, t core.Task, projectKey string, now time.Time) bool {
	return query.Matches(f, t, projectKey, now)
}
