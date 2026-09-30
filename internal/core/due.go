// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import "time"

// DueState classifies a deadline against the present. Every surface asks this
// question — a card marker, a row badge, a filter shorthand — and each one
// comparing timestamps for itself is how "overdue" came to mean three
// different things in three places.
type DueState uint8

// The due states. DueNone is the zero value because most of the interface has
// to answer "nothing to say" for a task carrying no deadline at all.
const (
	DueNone DueState = iota
	DueLater
	DueSoon
	DueOverdue
)

// DueSoonWindow is how near a deadline has to be before it is news. Beyond it
// a date is a plan rather than a deadline, and marking every dated task is
// what made the board's old due marker worthless: it appeared on nearly every
// card, so it distinguished nothing.
const DueSoonWindow = 7 * 24 * time.Hour

// DueStateOf classifies a deadline against now. A deadline exactly at now is
// overdue, which is the same boundary the stores filter on.
func DueStateOf(due *time.Time, now time.Time) DueState {
	switch {
	case due == nil:
		return DueNone
	case !due.After(now):
		return DueOverdue
	case due.Sub(now) <= DueSoonWindow:
		return DueSoon
	default:
		return DueLater
	}
}

// Notable reports whether a state is worth drawing. A task with no deadline
// and one due next quarter both have nothing to say right now.
func (d DueState) Notable() bool { return d == DueSoon || d == DueOverdue }

// String names the state.
func (d DueState) String() string {
	switch d {
	case DueOverdue:
		return "overdue"
	case DueSoon:
		return "due soon"
	case DueLater:
		return "due later"
	default:
		return "no due date"
	}
}

// DueState classifies this task's deadline against now.
func (t Task) DueState(now time.Time) DueState { return DueStateOf(t.DueAt, now) }
