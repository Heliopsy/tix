// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import "strings"

// TextField names the task text a TextTerm compares against.
type TextField string

// The text fields a filter term may address.
const (
	TextTitle TextField = "title"
	TextBody  TextField = "body"
	TextAny   TextField = "any"
)

// Valid reports whether f is a known text field.
func (f TextField) Valid() bool { return f == TextTitle || f == TextBody || f == TextAny }

// MatchMode selects how a TextTerm compares its value.
type MatchMode string

// The comparison modes a text term may use. MatchExact is whole-field
// equality; MatchContains is the weak match, a substring anywhere in the
// field. Both are case-insensitive.
const (
	MatchExact    MatchMode = "exact"
	MatchContains MatchMode = "contains"
)

// Valid reports whether m is a known match mode.
func (m MatchMode) Valid() bool { return m == MatchExact || m == MatchContains }

// Operator renders the mode as the expression operator that spells it.
func (m MatchMode) Operator() string {
	if m == MatchContains {
		return "~"
	}
	return ":"
}

// TextTerm is one predicate over a task's title or body. Several terms are
// combined with AND, so "title~api -body~legacy" wants both.
type TextTerm struct {
	Field  TextField `json:"field" yaml:"field"`
	Mode   MatchMode `json:"mode" yaml:"mode"`
	Value  string    `json:"value" yaml:"value"`
	Negate bool      `json:"negate,omitempty" yaml:"negate,omitempty"`
}

// Validate checks the term is one a store can build a predicate for.
func (t TextTerm) Validate() error {
	if !t.Field.Valid() {
		return Invalid("text field %q must be title, body or any", t.Field)
	}
	if !t.Mode.Valid() {
		return Invalid("text match mode %q must be exact or contains", t.Mode)
	}
	if strings.TrimSpace(t.Value) == "" {
		return Invalid("text term on %q has no value", t.Field)
	}
	return nil
}

// String renders the term in the wire form the HTTP API carries it as, which
// is the same spelling the filter expression uses: an optional leading "-",
// the field, the operator, and the value.
func (t TextTerm) String() string {
	prefix := ""
	if t.Negate {
		prefix = "-"
	}
	return prefix + string(t.Field) + t.Mode.Operator() + t.Value
}

// ParseTextTerm reads the form String produces.
func ParseTextTerm(s string) (TextTerm, error) {
	term := TextTerm{}
	rest, negated := strings.CutPrefix(strings.TrimSpace(s), "-")
	term.Negate = negated
	cut := strings.IndexAny(rest, ":~")
	if cut < 0 {
		return TextTerm{}, Invalid("text term %q must be field:value or field~value", s)
	}
	term.Field = TextField(strings.ToLower(rest[:cut]))
	term.Mode = MatchExact
	if rest[cut] == '~' {
		term.Mode = MatchContains
	}
	term.Value = rest[cut+1:]
	if err := term.Validate(); err != nil {
		return TextTerm{}, err
	}
	return term, nil
}

// Matches reports whether a title and body satisfy the term, using the same
// case-insensitive rules the stores apply.
func (t TextTerm) Matches(title, body string) bool {
	hit := t.matchField(title, body)
	if t.Negate {
		return !hit
	}
	return hit
}

func (t TextTerm) matchField(title, body string) bool {
	switch t.Field {
	case TextTitle:
		return t.compare(title)
	case TextBody:
		return t.compare(body)
	case TextAny:
		return t.compare(title) || t.compare(body)
	default:
		return false
	}
}

func (t TextTerm) compare(field string) bool {
	field, value := strings.ToLower(field), strings.ToLower(t.Value)
	if t.Mode == MatchContains {
		return strings.Contains(field, value)
	}
	return field == value
}

// TaskExclude carries the negated form of the list terms in TaskFilter. A task
// matching any value recorded here is removed from the result, so an exclusion
// beats an inclusion of the same value.
type TaskExclude struct {
	ProjectKeys []string   `json:"project_keys,omitempty" yaml:"project_keys,omitempty"`
	Statuses    []string   `json:"statuses,omitempty" yaml:"statuses,omitempty"`
	Tags        []string   `json:"tags,omitempty" yaml:"tags,omitempty"`
	AssigneeIDs []string   `json:"assignee_ids,omitempty" yaml:"assignee_ids,omitempty"`
	CreatorIDs  []string   `json:"creator_ids,omitempty" yaml:"creator_ids,omitempty"`
	ClaimedBy   []string   `json:"claimed_by,omitempty" yaml:"claimed_by,omitempty"`
	Priorities  []Priority `json:"priorities,omitempty" yaml:"priorities,omitempty"`
}

// Empty reports whether the exclusion selects nothing.
func (e TaskExclude) Empty() bool {
	return len(e.ProjectKeys) == 0 && len(e.Statuses) == 0 && len(e.Tags) == 0 &&
		len(e.AssigneeIDs) == 0 && len(e.CreatorIDs) == 0 && len(e.ClaimedBy) == 0 &&
		len(e.Priorities) == 0
}

// Validate checks every priority the exclusion names.
func (e TaskExclude) Validate() error {
	for _, p := range e.Priorities {
		if !p.Valid() {
			return Invalid("priority %d is out of range", p)
		}
	}
	return nil
}
