// SPDX-License-Identifier: AGPL-3.0-or-later

package taskio

import "strings"

// Target is the tix field a source column maps to.
type Target string

// Built-in targets.
const (
	TargetIgnore     Target = "ignore"
	TargetAppendBody Target = "append_body"
	TargetExternalID Target = "external_id"
	TargetAlias      Target = "alias"
	TargetRef        Target = "ref"
	TargetTitle      Target = "title"
	TargetBody       Target = "body"
	TargetStatus     Target = "status"
	TargetPriority   Target = "priority"
	TargetAssignee   Target = "assignee"
	TargetTags       Target = "tags"
	TargetDue        Target = "due"
	TargetParent     Target = "parent"
	TargetDependsOn  Target = "depends_on"
	TargetRecurrence Target = "recurrence"
)

const customPrefix = "custom:"

// Targets lists the built-in targets in display order.
var Targets = []Target{
	TargetTitle, TargetBody, TargetStatus, TargetPriority, TargetAssignee,
	TargetTags, TargetDue, TargetParent, TargetDependsOn, TargetRecurrence,
	TargetExternalID, TargetAlias, TargetRef, TargetAppendBody, TargetIgnore,
}

// CustomTarget is the target for the custom field with the given key.
func CustomTarget(key string) Target { return Target(customPrefix + key) }

// CustomKey reports the custom field key a target names, if it names one.
func (t Target) CustomKey() (string, bool) {
	key, ok := strings.CutPrefix(string(t), customPrefix)
	if !ok || key == "" {
		return "", false
	}
	return key, true
}

// Valid reports whether t is a built-in target or a well-formed custom one.
func (t Target) Valid() bool {
	if key, ok := t.CustomKey(); ok {
		return strings.TrimSpace(key) == key && !strings.ContainsAny(key, " \t\r\n")
	}
	for _, b := range Targets {
		if t == b {
			return true
		}
	}
	return false
}

// Multi reports whether several columns may map to t at once.
func (t Target) Multi() bool {
	switch t {
	case TargetTags, TargetAppendBody, TargetDependsOn, TargetIgnore, TargetAlias:
		return true
	}
	return false
}
