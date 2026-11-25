// SPDX-License-Identifier: AGPL-3.0-or-later

package query

import "strings"

// Term is one word of a filter expression as it was typed. Raw is the word
// itself, quotes and negation included, so an expression can be rebuilt
// without one term and keep the others byte for byte. Key is the canonical key
// ("" for free text), so "p:web", "@raj" and "a:raj" read as project and
// assignee terms.
type Term struct {
	Raw     string
	Key     string
	Value   string
	Negated bool
	Weak    bool
}

// keyAliases are the short spellings applyTerm accepts for a key.
var keyAliases = map[string]string{"p": "project", "s": "status", "a": "assignee", "prio": "priority", "q": "text"}

// CanonicalKey is the key a term's prefix names, folding the short aliases.
// A custom field keeps its case, as the parser does.
func CanonicalKey(key string) string {
	key = strings.TrimSpace(key)
	if strings.HasPrefix(key, CustomFieldPrefix) {
		return key
	}
	key = strings.ToLower(key)
	if long, ok := keyAliases[key]; ok {
		return long
	}
	return key
}

// Terms splits an expression into its words with the parser's own quoting
// rules. It never fails: an unterminated quote runs to the end of the
// expression as one word, which is what a reader removing it wants to see.
func Terms(expr string) []Term {
	var out []Term
	for _, raw := range splitRaw(expr) {
		out = append(out, termOf(raw))
	}
	return out
}

// Join rebuilds an expression from its terms.
func Join(terms []Term) string {
	parts := make([]string, 0, len(terms))
	for _, t := range terms {
		parts = append(parts, t.Raw)
	}
	return strings.Join(parts, " ")
}

// Quote writes key:value, quoting a value that holds a space or a quote mark.
func Quote(key, value string) string {
	if strings.ContainsAny(value, ` "`) {
		value = `"` + strings.ReplaceAll(value, `"`, "") + `"`
	}
	return key + ":" + value
}

func splitRaw(expr string) []string {
	var out []string
	var cur strings.Builder
	open := false
	for _, r := range expr {
		switch {
		case r == '"':
			open = !open
			cur.WriteRune(r)
		case r == ' ' && !open:
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func termOf(raw string) Term {
	t := Term{Raw: raw}
	if strings.HasPrefix(raw, `"`) {
		t.Value = strings.Trim(raw, `"`)
		return t
	}
	text, negate := strings.CutPrefix(raw, NegationPrefix)
	t.Negated = negate
	if handle, at := atHandle(text); at {
		t.Key, t.Value = "assignee", handle
		return t
	}
	key, value, op, ok := cutTerm(text)
	if !ok {
		t.Value = text
		return t
	}
	t.Key, t.Value, t.Weak = CanonicalKey(key), strings.Trim(value, `"`), op == WeakOperator
	return t
}
