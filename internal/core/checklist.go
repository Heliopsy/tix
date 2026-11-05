// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import "strings"

// Checklist is how many of a description's checklist items are ticked.
type Checklist struct {
	// Done counts the ticked items.
	Done int `json:"done" yaml:"done"`
	// Total counts every item.
	Total int `json:"total" yaml:"total"`
}

// CheckItemInput ticks or clears one checklist item. Item counts from one in
// reading order; Version, when set, must be the task's current version.
type CheckItemInput struct {
	// Item is the checklist item to change, counted from 1 in reading order.
	Item int `json:"item" yaml:"item"`
	// Done ticks the item when true and clears it when false.
	Done bool `json:"done" yaml:"done"`
	// Version is the task version the change was based on; a stale one is a
	// conflict, and omitted skips the check.
	Version int `json:"version,omitempty" yaml:"version,omitempty"`
}

// ChecklistOpenMarkers are the unticked item markers a checklist filter looks
// for, and ChecklistDoneMarkers the ticked ones. A filter reads the raw text,
// so a marker inside a code block counts there although it renders as code.
var (
	ChecklistOpenMarkers = []string{"- [ ] ", "* [ ] ", "+ [ ] "}
	ChecklistDoneMarkers = []string{"- [x] ", "* [x] ", "+ [x] ", "- [X] ", "* [X] ", "+ [X] "}
)

// HasChecklistMarker reports whether body carries any checklist marker.
func HasChecklistMarker(body string) bool {
	return containsAny(body, ChecklistOpenMarkers) || containsAny(body, ChecklistDoneMarkers)
}

// HasOpenChecklistMarker reports whether body carries an unticked marker.
func HasOpenChecklistMarker(body string) bool { return containsAny(body, ChecklistOpenMarkers) }

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
