// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/quickadd"
	"github.com/heliopsy/tix/internal/taskio"
)

// valueBook translates the status values of a file into the project's
// workflow states, and keeps what each distinct value became for the report.
type valueBook struct {
	mapping  *taskio.Mapping
	workflow core.WorkflowDefinition
	states   []taskio.StateInfo
	distinct taskio.Distinct

	// suggested holds the states tix guessed for values the mapping did not
	// name, so the next run can be handed them as an explicit mapping.
	suggested map[string]string
	// actors reports whether an assignee name is a tix actor.
	actors func(name string) bool
}

func newValueBook(m *taskio.Mapping, wf core.WorkflowDefinition) valueBook {
	b := valueBook{mapping: m, workflow: wf, suggested: map[string]string{}}
	for _, s := range wf.States {
		b.states = append(b.states, taskio.StateInfo{Key: s.Key, Label: s.Label,
			Category: wf.CategoryOf(s.Key), Initial: s.Key == wf.Initial})
	}
	return b
}

// observe counts a draft's raw values.
func (b *valueBook) observe(d taskio.Draft) { b.distinct.AddDraft(d) }

// state is the workflow state a value names by key or label, or empty.
func (b *valueBook) state(v string) string {
	for _, s := range b.states {
		if strings.EqualFold(s.Key, v) || strings.EqualFold(s.Label, v) {
			return s.Key
		}
	}
	return ""
}

// explicit is the mapping's own translation of a raw status value.
func (b *valueBook) explicit(raw string) (string, bool) {
	for k, v := range b.mapping.Values[taskio.TargetStatus] {
		if strings.EqualFold(k, raw) {
			return v, true
		}
	}
	return "", false
}

// status is the state a row's status becomes, or a reason to skip the row.
// An empty state with no skip means the value names no state and the row is
// refused.
func (b *valueBook) status(value string) (string, string) {
	if value == "" {
		return "", ""
	}
	if key := b.state(value); key != "" {
		return key, ""
	}
	if _, named := b.explicit(value); !named {
		if key := taskio.SuggestStatus(value, b.states); key != "" {
			b.suggested[value] = key
			return key, ""
		}
	}
	switch b.mapping.Policy(taskio.TargetStatus) {
	case taskio.PolicySkipRow:
		return "", "status " + value + " has no mapping"
	case taskio.PolicyDefault:
		return b.defaultStatus(), ""
	default:
		return "", ""
	}
}

// defaultStatus is the state a row without one starts in.
func (b *valueBook) defaultStatus() string {
	if key := b.state(b.mapping.Defaults[taskio.TargetStatus]); key != "" {
		return key
	}
	return b.workflow.Initial
}

// report lists the distinct values of status, priority and assignee and what
// each became.
func (b *valueBook) report() map[string][]core.TaskImportValue {
	out := map[string][]core.TaskImportValue{}
	for _, t := range []taskio.Target{taskio.TargetStatus, taskio.TargetPriority, taskio.TargetAssignee} {
		for _, vc := range b.distinct.Values(t) {
			v := core.TaskImportValue{Value: vc.Value, Count: vc.Count}
			v.Mapped, v.Suggested = b.translated(t, vc.Value)
			out[string(t)] = append(out[string(t)], v)
		}
	}
	return out
}

// translated is what one raw value becomes, and whether that is tix's guess.
func (b *valueBook) translated(t taskio.Target, raw string) (string, bool) {
	for k, v := range b.mapping.Values[t] {
		if strings.EqualFold(k, raw) {
			return v, false
		}
	}
	switch t {
	case taskio.TargetStatus:
		if key := b.state(raw); key != "" {
			return key, false
		}
		if key := taskio.SuggestStatus(raw, b.states); key != "" {
			return key, true
		}
	case taskio.TargetPriority:
		if p, ok := taskio.SuggestPriority(raw); ok && p != 0 {
			return quickadd.PriorityName(p), true
		}
	case taskio.TargetAssignee:
		if b.actors != nil && b.actors(raw) {
			return raw, false
		}
	}
	return "", false
}

// mapped is the mapping's values with tix's guesses made explicit, the form
// a saved mapping carries so the next run needs no guessing.
func (b *valueBook) mapped() map[taskio.Target]map[string]string {
	out := map[taskio.Target]map[string]string{}
	for t, values := range b.mapping.Values {
		out[t] = map[string]string{}
		for k, v := range values {
			out[t][k] = v
		}
	}
	for raw, key := range b.suggested {
		if out[taskio.TargetStatus] == nil {
			out[taskio.TargetStatus] = map[string]string{}
		}
		out[taskio.TargetStatus][raw] = key
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
