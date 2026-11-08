// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"slices"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/onboard"
)

// scopeOption is one checkbox of the scope picker.
type scopeOption struct {
	Value core.Scope
	// Note says what the scope allows, in one line.
	Note string
	// Held is whether the reader holds the scope and so may grant it. One
	// they do not hold is drawn disabled with the reason, never left out.
	Held    bool
	Checked bool
}

// scopeGroup is a headed set of checkboxes with its own select-all.
type scopeGroup struct {
	ID     string
	Name   string
	Scopes []scopeOption
}

// scopePreset ticks a named bundle of scopes and nothing else.
type scopePreset struct {
	ID     string
	Name   string
	Note   string
	Scopes string
}

// scopePicker is the one scope control every token form draws: the tokens
// screen's dialog and the connect screen.
type scopePicker struct {
	Groups  []scopeGroup
	Presets []scopePreset
	// All is whether the reader holds *, which is then offered as its own
	// choice with a warning. Select all never ticks it.
	All        bool
	AllChecked bool
}

// scopeNotes and scopeGroups are internal/onboard's, so the browser and the
// terminal draw one scope picker.
var (
	scopeNotes  = onboard.ScopeNotes
	scopeGroups = onboard.ScopeGroups
)

// newScopePicker draws the picker for a reader, with chosen ticked.
func newScopePicker(actor *core.Actor, chosen []core.Scope) scopePicker {
	out := scopePicker{All: actor.HasScope(core.ScopeAll), AllChecked: slices.Contains(chosen, core.ScopeAll)}
	for _, g := range scopeGroups {
		group := scopeGroup{ID: g.ID, Name: g.Name}
		for _, s := range g.Scopes {
			group.Scopes = append(group.Scopes, scopeOption{Value: s, Note: scopeNotes[s],
				Held: actor.HasScope(s), Checked: slices.Contains(chosen, s)})
		}
		out.Groups = append(out.Groups, group)
	}
	for _, p := range onboard.Presets {
		if heldAll(actor, p.Scopes) {
			names := make([]string, len(p.Scopes))
			for i, s := range p.Scopes {
				names[i] = string(s)
			}
			out.Presets = append(out.Presets, scopePreset{ID: p.ID, Name: p.Name, Note: p.Note,
				Scopes: strings.Join(names, " ")})
		}
	}
	return out
}

// heldAll reports whether the actor holds every scope.
func heldAll(actor *core.Actor, scopes []core.Scope) bool {
	for _, s := range scopes {
		if !actor.HasScope(s) {
			return false
		}
	}
	return true
}

// heldScopes are the scopes of list the actor holds, in order.
func heldScopes(actor *core.Actor, list []core.Scope) []core.Scope {
	var out []core.Scope
	for _, s := range list {
		if actor.HasScope(s) {
			out = append(out, s)
		}
	}
	return out
}
