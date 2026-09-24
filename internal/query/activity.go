// SPDX-License-Identifier: AGPL-3.0-or-later

package query

import (
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// ActivityKeys are the term prefixes the activity filter accepts. They are
// not the task filter's keys: a task row is always a task, so "kind" and
// "source" have nothing to select there, while an activity row is whatever
// the tenant just did and has no status, priority or due date to ask about.
// The lexer, the negation prefix and the weak operator are shared, so the two
// grammars are spelled the same way even where their vocabularies differ.
var ActivityKeys = []string{"actor", "kind", "action", "type", "source", "text", "q"}

// ActivitySyntaxHint is the one line that tells somebody the operators exist,
// the activity counterpart of SyntaxHint.
func ActivitySyntaxHint() string {
	return "prefix a term with " + NegationPrefix + " to exclude it (" + NegationPrefix +
		"kind:webhook); bare words match anywhere in the row"
}

// ActivitySources are the surfaces a recorded change can have arrived from.
var ActivitySources = []core.Source{
	core.SourceWeb, core.SourceCLI, core.SourceAPI, core.SourceTUI, core.SourceSystem,
}

// ActivityFilter selects rows of the activity feed: the audit log the CLI
// reads and the live event tail the terminal interface draws.
type ActivityFilter struct {
	Text    []string
	Actors  []string
	Kinds   []string
	Actions []string
	Sources []core.Source
	Exclude ActivityExclusion
}

// ActivityExclusion holds the terms an activity filter rejects rows for.
type ActivityExclusion struct {
	Text    []string
	Actors  []string
	Kinds   []string
	Actions []string
	Sources []core.Source
}

// ActivityRow is one row of the feed reduced to what a filter asks about, so
// that one matcher answers an audit entry and a live event alike rather than
// each surface growing its own. Actors lists every name a row's actor answers
// to, because an event carries a handle where an audit entry carries only the
// identifier, and "actor:me" should find both.
type ActivityRow struct {
	Actors []string
	Kind   string
	Action string
	Source core.Source
	Text   []string
}

// AuditRow reduces an audit entry to the row a filter is answered against.
// The free text is matched against what the entry itself records rather than
// the sentence a screen renders it as, because the snapshots are where the
// words somebody actually searches for live: a task's ref and title, a
// project's key, a comment's body.
func AuditRow(e core.AuditEntry) ActivityRow {
	return ActivityRow{
		Actors: []string{e.ActorID},
		Kind:   e.SubjectType,
		Action: e.Action,
		Source: e.Source,
		Text: []string{
			e.Action, e.SubjectType, e.SubjectID, string(e.Source),
			string(e.Before), string(e.After),
		},
	}
}

// ParseActivity turns an activity filter expression into the filter both the
// audit listing and the live tail are selected by.
func ParseActivity(expr string) (ActivityFilter, error) {
	var f ActivityFilter
	tokens, err := tokenize(expr)
	if err != nil {
		return ActivityFilter{}, err
	}
	for _, tok := range tokens {
		if tok.quoted {
			f.Text = append(f.Text, tok.text)
			continue
		}
		text, negate := strings.CutPrefix(tok.text, NegationPrefix)
		key, value, _, ok := cutTerm(text)
		if !ok {
			if text != "" {
				appendTo(&f.Text, &f.Exclude.Text, text, negate)
			}
			continue
		}
		if err := f.applyActivityTerm(strings.ToLower(strings.TrimSpace(key)), value, negate); err != nil {
			return ActivityFilter{}, err
		}
	}
	return f, nil
}

// applyActivityTerm folds one term into the filter. The weak operator is
// accepted wherever a value is free text and means exactly what ":" means
// here, because every activity text match is already a substring match: a row
// is a rendered record rather than a field somebody typed the whole of.
func (f *ActivityFilter) applyActivityTerm(key, value string, negate bool) error {
	value = strings.TrimSpace(strings.Trim(value, `"`))
	if value == "" {
		return core.Invalid("activity filter term %q has no value", key)
	}
	switch key {
	case "actor":
		appendTo(&f.Actors, &f.Exclude.Actors, value, negate)
	case "kind":
		appendTo(&f.Kinds, &f.Exclude.Kinds, value, negate)
	case "action", "type":
		appendTo(&f.Actions, &f.Exclude.Actions, value, negate)
	case "source":
		source, err := ParseSource(value)
		if err != nil {
			return err
		}
		if negate {
			f.Exclude.Sources = append(f.Exclude.Sources, source)
			return nil
		}
		f.Sources = append(f.Sources, source)
	case "text", "q":
		appendTo(&f.Text, &f.Exclude.Text, value, negate)
	default:
		return core.Invalid("unknown activity filter key %q; try one of %s",
			key, strings.Join(ActivityKeys, ", "))
	}
	return nil
}

// ParseSource resolves a surface name, naming the alternatives when it cannot.
// An unknown name is refused rather than passed through, because a filter on a
// source nothing records selects nothing and looks like an empty log.
func ParseSource(value string) (core.Source, error) {
	want := core.Source(strings.ToLower(strings.TrimSpace(value)))
	for _, s := range ActivitySources {
		if s == want {
			return s, nil
		}
	}
	return "", core.Invalid("unknown source %q; try one of %s", value, sourceNames())
}

// sourceNames renders the recorded surfaces for a message.
func sourceNames() string {
	out := make([]string, 0, len(ActivitySources))
	for _, s := range ActivitySources {
		out = append(out, string(s))
	}
	return strings.Join(out, ", ")
}

// Active reports whether the filter selects anything out.
func (f ActivityFilter) Active() bool {
	return len(f.Text) > 0 || len(f.Actors) > 0 || len(f.Kinds) > 0 ||
		len(f.Actions) > 0 || len(f.Sources) > 0 ||
		len(f.Exclude.Text) > 0 || len(f.Exclude.Actors) > 0 || len(f.Exclude.Kinds) > 0 ||
		len(f.Exclude.Actions) > 0 || len(f.Exclude.Sources) > 0
}

// UnsupportedForEvents names the terms the live event tail cannot answer. An
// event carries no source: the outbox records what happened, not which surface
// asked for it, so a source term on the tail would silently select nothing.
// Saying so is the honest answer; the audit log is where source lives.
func (f ActivityFilter) UnsupportedForEvents() []string {
	if len(f.Sources) == 0 && len(f.Exclude.Sources) == 0 {
		return nil
	}
	return []string{"source"}
}

// Matches reports whether a row satisfies the filter. Values of one key are
// alternatives, keys are conjunctions, and every free-text word must appear
// somewhere in the row.
func (f ActivityFilter) Matches(r ActivityRow) bool {
	switch {
	case !matchesAny(f.Actors, r.Actors...):
		return false
	case !matchesAny(f.Kinds, r.Kind):
		return false
	case !matchesAny(f.Actions, r.Action):
		return false
	case !matchesAny(sourceStrings(f.Sources), string(r.Source)):
		return false
	case excludes(f.Exclude.Actors, r.Actors...):
		return false
	case excludes(f.Exclude.Kinds, r.Kind):
		return false
	case excludes(f.Exclude.Actions, r.Action):
		return false
	case excludes(sourceStrings(f.Exclude.Sources), string(r.Source)):
		return false
	}
	for _, want := range f.Text {
		if !containsText(r.Text, want) {
			return false
		}
	}
	for _, unwanted := range f.Exclude.Text {
		if containsText(r.Text, unwanted) {
			return false
		}
	}
	return true
}

// matchesAny reports whether any candidate equals one of the wanted values,
// ignoring case. An empty want list selects everything.
func matchesAny(want []string, candidates ...string) bool {
	if len(want) == 0 {
		return true
	}
	for _, w := range want {
		for _, c := range candidates {
			if strings.EqualFold(w, c) {
				return true
			}
		}
	}
	return false
}

// excludes reports whether any candidate is one of the rejected values. An
// empty rejection list excludes nothing, which is the opposite of what an
// empty selection list means, so the two cannot share one helper.
func excludes(unwanted []string, candidates ...string) bool {
	if len(unwanted) == 0 {
		return false
	}
	return matchesAny(unwanted, candidates...)
}

// containsText reports whether any searchable field holds the needle.
func containsText(fields []string, needle string) bool {
	lowered := strings.ToLower(needle)
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), lowered) {
			return true
		}
	}
	return false
}

// sourceStrings renders sources for the shared comparison.
func sourceStrings(sources []core.Source) []string {
	out := make([]string, 0, len(sources))
	for _, s := range sources {
		out = append(out, string(s))
	}
	return out
}

// AuditFilter folds the parts of the filter the store can answer into an
// audit query, leaving base's own terms in place. It is an optimisation and
// never the definition: Matches is applied to whatever comes back, so a term
// the store cannot take still selects the same rows. A kind term is pushed
// only when it is the single positive one, because the store takes one
// subject type, and never over a subject type the caller already named.
func (f ActivityFilter) AuditFilter(base core.AuditFilter) core.AuditFilter {
	out := base
	out.ActorIDs = append(append([]string(nil), out.ActorIDs...), f.Actors...)
	out.Actions = append(append([]string(nil), out.Actions...), f.Actions...)
	out.Sources = append(append([]core.Source(nil), out.Sources...), f.Sources...)
	if len(f.Kinds) == 1 && out.SubjectType == "" {
		out.SubjectType = f.Kinds[0]
	}
	return out
}
