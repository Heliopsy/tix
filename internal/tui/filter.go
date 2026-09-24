// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tui implements the tix terminal interface over the core service.
package tui

import (
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
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

// ActivityFilterKeys are the term prefixes the activity filter bar accepts.
var ActivityFilterKeys = query.ActivityKeys

// ActivitySyntaxHint is the line the activity help shows beside its keys.
func ActivitySyntaxHint() string { return query.ActivitySyntaxHint() }

// ParseActivityFilter turns an activity filter expression into the filter the
// audit listing is selected by, refusing the terms a live event tail cannot
// answer. It is the shared parser, so `tix audit ls --filter` and the
// activity bar cannot drift apart; the one difference is stated rather than
// silently ignored, because an event records what happened and not which
// surface asked for it.
func ParseActivityFilter(expr string) (query.ActivityFilter, error) {
	f, err := query.ParseActivity(expr)
	if err != nil {
		return query.ActivityFilter{}, err
	}
	if absent := f.UnsupportedForEvents(); len(absent) > 0 {
		return query.ActivityFilter{}, core.Invalid(
			"the live activity tail carries no %s; filter the audit log with `tix audit ls --filter` instead",
			strings.Join(absent, " or "))
	}
	return f, nil
}

// EventRow reduces a live event to the row the shared activity filter is
// answered against. Its text is what the line on screen says, so a word
// somebody can read in the tail is a word they can filter it by.
func EventRow(e core.Event) query.ActivityRow {
	return query.ActivityRow{
		Actors: []string{e.ActorID, output.EventActor(e)},
		Kind:   e.SubjectType,
		Action: string(e.Type),
		Text: []string{
			string(e.Type), e.SubjectType, e.SubjectID,
			output.EventActor(e), output.EventVerb(e.Type),
			output.EventRef(e), output.EventDetail(e),
		},
	}
}

// MatchesActivity reports whether one event satisfies the activity filter.
func MatchesActivity(f query.ActivityFilter, e core.Event) bool { return f.Matches(EventRow(e)) }

// VisibleEvents keeps the events an activity filter accepts, in order.
func VisibleEvents(events []core.Event, f query.ActivityFilter) []core.Event {
	if !f.Active() {
		return events
	}
	out := make([]core.Event, 0, len(events))
	for _, e := range events {
		if MatchesActivity(f, e) {
			out = append(out, e)
		}
	}
	return out
}
