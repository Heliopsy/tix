// SPDX-License-Identifier: AGPL-3.0-or-later

// Package query parses the tix filter expression into a core.TaskFilter, and
// answers the same expression against a task in memory. It is the one parser
// the CLI and the terminal interface share, so a filter typed into either
// selects the same tasks.
package query

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// DateLayouts are the timestamp forms a filter term may take.
var DateLayouts = []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04", "2006-01-02"}

// CustomFieldPrefix marks a term that filters on a custom field, as in
// "field.severity:high".
const CustomFieldPrefix = "field."

// Keys are the term prefixes the filter language accepts.
var Keys = []string{
	"project", "status", "tag", "assignee", "creator", "claimed-by",
	"priority", "due-before", "due-after", "parent", "is", "sort", "limit",
	"text", "title", "body", "claimed", "blocked", "deleted",
}

// SyntaxHint is the one line that tells somebody the two operators exist. A
// list of keys alone does not: nothing in "status, tag, assignee" suggests
// that a term can be negated or matched weakly, so a surface that offers the
// filter language offers this alongside its key list.
func SyntaxHint() string {
	return "prefix a term with " + NegationPrefix + " to exclude it (" + NegationPrefix +
		"tag:ops); use " + WeakOperator + " instead of : to match anywhere inside title, body or text (title" +
		WeakOperator + "api)"
}

// NegationPrefix marks a term as excluding rather than selecting. It leads the
// whole term, so "-tag:ops" reads as one word and the same prefix negates a
// bare text word. The alternative spellings were rejected deliberately:
// "status:!done" puts the negation inside a value that may legitimately start
// with an exclamation mark, and a "NOT" keyword needs a precedence grammar
// this flat conjunction of terms does not have.
const NegationPrefix = "-"

// WeakOperator separates a key from a value that need only appear inside the
// field, rather than be the whole of it.
const WeakOperator = "~"

// token is one lexed word of a filter expression. quoted marks a token the
// quote opened, which is free text however many colons it holds; a quote that
// opens after a key, as in title:"two words", only groups the value.
type token struct {
	text   string
	quoted bool
}

// Parse turns a filter expression into the filter the service takes.
func Parse(expr string) (core.TaskFilter, error) {
	var f core.TaskFilter
	tokens, err := tokenize(expr)
	if err != nil {
		return core.TaskFilter{}, err
	}
	var words []string
	for _, tok := range tokens {
		if tok.quoted {
			words = append(words, tok.text)
			continue
		}
		text, negate := strings.CutPrefix(tok.text, NegationPrefix)
		key, value, op, ok := cutTerm(text)
		if !ok {
			if err := applyWord(&f, &words, text, negate); err != nil {
				return core.TaskFilter{}, err
			}
			continue
		}
		// A custom field keeps the case it was written in: the name is a
		// key in a JSON document, where "storyPoints" and "storypoints" are
		// two different fields. Every other key is a fixed word and is
		// folded, so Status: and status: mean the same thing.
		trimmed := strings.TrimSpace(key)
		if name, ok := strings.CutPrefix(trimmed, CustomFieldPrefix); ok {
			if err := applyCustomField(&f, name, value, negate); err != nil {
				return core.TaskFilter{}, err
			}
			continue
		}
		if err := applyTerm(&f, strings.ToLower(trimmed), value, op, negate); err != nil {
			return core.TaskFilter{}, err
		}
	}
	f.Query = strings.Join(words, " ")
	return f.Validate()
}

// cutTerm splits a term on the first operator, reporting which one it was.
func cutTerm(text string) (key, value, op string, ok bool) {
	cut := strings.IndexAny(text, ":"+WeakOperator)
	if cut < 0 {
		return "", "", "", false
	}
	return text[:cut], text[cut+1:], string(text[cut]), true
}

// applyWord folds a bare word into the filter: free text when it selects, a
// negated substring term when it excludes, since a negated free-text search
// has no meaning the engines answer the same way.
func applyWord(f *core.TaskFilter, words *[]string, text string, negate bool) error {
	if text == "" {
		return nil
	}
	if !negate {
		*words = append(*words, text)
		return nil
	}
	f.Text = append(f.Text, core.TextTerm{
		Field: core.TextAny, Mode: core.MatchContains, Value: text, Negate: true,
	})
	return nil
}

// tokenize splits an expression on spaces, honouring double quotes.
func tokenize(expr string) ([]token, error) {
	var out []token
	var cur strings.Builder
	seen, quoted, open := false, false, false
	for _, r := range expr {
		switch {
		case r == '"':
			if !open && cur.Len() == 0 {
				quoted = true
			}
			seen, open = true, !open
		case r == ' ' && !open:
			if cur.Len() > 0 || seen {
				out = append(out, token{text: cur.String(), quoted: quoted})
				cur.Reset()
				seen, quoted = false, false
			}
		default:
			cur.WriteRune(r)
		}
	}
	if open {
		return nil, core.Invalid("filter has an unterminated quote")
	}
	if cur.Len() > 0 || seen {
		out = append(out, token{text: cur.String(), quoted: quoted})
	}
	return out, nil
}

// applyTerm folds one term into the filter.
func applyTerm(f *core.TaskFilter, key, value, op string, negate bool) error {
	value = strings.TrimSpace(strings.Trim(value, `"`))
	if value == "" {
		return core.Invalid("filter term %q has no value", key)
	}
	weak := op == WeakOperator
	if weak && !textKey(key) {
		return core.Invalid("%q takes an exact value; the weak operator %q only applies to title, body or text",
			key, WeakOperator)
	}
	if negate && !negatable(key) {
		return core.Invalid("filter term %q cannot be negated", key)
	}
	switch key {
	case "project", "p":
		appendTo(&f.ProjectKeys, &f.Exclude.ProjectKeys, strings.ToLower(value), negate)
	case "status", "s":
		appendTo(&f.Statuses, &f.Exclude.Statuses, value, negate)
	case "tag":
		appendTo(&f.Tags, &f.Exclude.Tags, value, negate)
	case "assignee", "a":
		appendTo(&f.AssigneeIDs, &f.Exclude.AssigneeIDs, value, negate)
	case "creator":
		appendTo(&f.CreatorIDs, &f.Exclude.CreatorIDs, value, negate)
	case "claimed-by":
		appendTo(&f.ClaimedBy, &f.Exclude.ClaimedBy, value, negate)
	case "priority", "prio":
		return applyPriority(f, value, negate)
	case "due-before":
		return applyDue(&f.DueBefore, value)
	case "due-after":
		return applyDue(&f.DueAfter, value)
	case "parent":
		return applyParent(f, value)
	case "is":
		return applyIs(f, strings.ToLower(value), negate)
	case "sort":
		f.Page.Sort = value
	case "limit":
		return applyLimit(f, value)
	case "title", "body":
		f.Text = append(f.Text, textTerm(core.TextField(key), value, weak, negate))
	case "text", "q":
		return applyText(f, value, weak, negate)
	// The three boolean forms the web filter bar has always accepted. They
	// say the same thing as is:, and dropping them would break every saved
	// link and bookmark that spells it the old way for no gain.
	case "claimed", "blocked", "deleted":
		state, err := boolTerm(key, value)
		if err != nil {
			return err
		}
		return applyIs(f, state, negate)
	default:
		return core.Invalid("unknown filter key %q; try one of %s", key, strings.Join(Keys, ", "))
	}
	return nil
}

// boolTerm turns claimed:true and blocked:false into the is: state they mean.
func boolTerm(key, value string) (string, error) {
	switch strings.ToLower(value) {
	case "true", "yes", "1", "":
		return key, nil
	case "false", "no", "0":
		switch key {
		case "claimed":
			return "unclaimed", nil
		case "blocked":
			return "unblocked", nil
		default:
			return "", core.Invalid("filter term %q cannot be negated with a value; use -%s", key, key)
		}
	default:
		return "", core.Invalid("filter term %q takes true or false, not %q", key, value)
	}
}

// applyCustomField folds a field.<name>:<value> term into the filter. The
// value is read as JSON when it parses as one, so field.count:3 filters on a
// number rather than the string "3", and falls back to the literal text.
func applyCustomField(f *core.TaskFilter, name, value string, negate bool) error {
	if negate {
		return core.Invalid("custom field term %q cannot be negated yet", CustomFieldPrefix+name)
	}
	if value == "" {
		return core.Invalid("filter term %q has no value", CustomFieldPrefix+name)
	}
	if !validFieldKey(name) {
		return core.Invalid("custom field key %q must hold only letters, digits, underscores and dashes", name)
	}
	var decoded any
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		decoded = value
	}
	switch decoded.(type) {
	case string, bool, float64:
	default:
		return core.Invalid("custom field filter %q takes a string, number or boolean", name)
	}
	if f.CustomFields == nil {
		f.CustomFields = map[string]any{}
	}
	f.CustomFields[name] = decoded
	return nil
}

// validFieldKey mirrors what the stores accept in a JSON path.
func validFieldKey(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_', c == '-':
		default:
			return false
		}
	}
	return true
}

// textKey reports whether a key addresses task text, the only place the weak
// operator has a meaning.
func textKey(key string) bool {
	switch key {
	case "title", "body", "text", "q":
		return true
	default:
		return false
	}
}

// negatable reports whether a key carries a negated form. sort, limit and the
// date bounds select a shape of the listing rather than a set of tasks, so
// excluding one is not a question the store can answer.
func negatable(key string) bool {
	switch key {
	case "sort", "limit", "due-before", "due-after", "parent":
		return false
	default:
		return true
	}
}

// appendTo records a value as an inclusion or an exclusion.
func appendTo(include, exclude *[]string, value string, negate bool) {
	if negate {
		*exclude = append(*exclude, value)
		return
	}
	*include = append(*include, value)
}

// textTerm builds one title or body predicate.
func textTerm(field core.TextField, value string, weak, negate bool) core.TextTerm {
	mode := core.MatchExact
	if weak {
		mode = core.MatchContains
	}
	return core.TextTerm{Field: field, Mode: mode, Value: value, Negate: negate}
}

// applyText folds a free-text term in. Only the plain selecting form feeds
// Query, the engine-native search; everything else becomes an explicit term
// both engines answer identically.
func applyText(f *core.TaskFilter, value string, weak, negate bool) error {
	if !weak && !negate {
		f.Query = strings.TrimSpace(f.Query + " " + value)
		return nil
	}
	f.Text = append(f.Text, textTerm(core.TextAny, value, weak || negate, negate))
	return nil
}

// applyPriority parses a priority name or number into the filter.
func applyPriority(f *core.TaskFilter, value string, negate bool) error {
	p, err := ParsePriority(value)
	if err != nil {
		return err
	}
	if negate {
		f.Exclude.Priorities = append(f.Exclude.Priorities, p)
		return nil
	}
	f.Priorities = append(f.Priorities, p)
	return nil
}

// applyDue parses a date term into a bound.
func applyDue(dst **time.Time, value string) error {
	t, err := ParseTime(value)
	if err != nil {
		return err
	}
	*dst = t
	return nil
}

// applyParent selects a parent, or the roots.
func applyParent(f *core.TaskFilter, value string) error {
	if strings.EqualFold(value, "none") {
		f.ParentIsNull = true
		return nil
	}
	f.ParentID = value
	return nil
}

// applyIs folds a boolean term into the filter, inverting the sense of the
// tri-state ones when the term was negated.
func applyIs(f *core.TaskFilter, value string, negate bool) error {
	yes, no := core.Yes, core.No
	if negate {
		yes, no = core.No, core.Yes
	}
	switch value {
	case "claimed":
		f.Claimed = yes
	case "unclaimed", "free":
		f.Claimed = no
	case "blocked":
		f.Blocked = yes
	case "unblocked", "ready":
		f.Blocked = no
	case "deleted":
		f.IncludeDeleted = !negate
	case "root":
		if negate {
			return core.Invalid("is:root cannot be negated; name a parent instead")
		}
		f.ParentIsNull = true
	default:
		return core.Invalid("unknown is: value %q; try claimed, unclaimed, blocked, unblocked, deleted or root", value)
	}
	return nil
}

// applyLimit parses a page limit term.
func applyLimit(f *core.TaskFilter, value string) error {
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return core.Invalid("limit %q must be a non-negative number", value)
	}
	f.Page.Limit = n
	return nil
}

// ParsePriority accepts a priority name or a number between 1 and 5.
func ParsePriority(value string) (core.Priority, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "highest":
		return core.PriorityHighest, nil
	case "high":
		return core.PriorityHigh, nil
	case "normal", "medium":
		return core.PriorityNormal, nil
	case "low":
		return core.PriorityLow, nil
	case "lowest":
		return core.PriorityLowest, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, core.Invalid("priority %q must be a name or a number from 1 to 5", value)
	}
	p := core.Priority(n)
	if !p.Valid() {
		return 0, core.Invalid("priority %d is out of range", n)
	}
	return p, nil
}

// ParseTime accepts a date or a timestamp.
func ParseTime(value string) (*time.Time, error) {
	trimmed := strings.TrimSpace(value)
	for _, layout := range DateLayouts {
		if t, err := time.Parse(layout, trimmed); err == nil {
			utc := t.UTC()
			return &utc, nil
		}
	}
	return nil, core.Invalid("time %q must be RFC3339 or YYYY-MM-DD", value)
}
