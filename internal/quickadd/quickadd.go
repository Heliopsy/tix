// SPDX-License-Identifier: AGPL-3.0-or-later

// Package quickadd reads one line such as "Fix login !high #auth @raj due:fri"
// into a task title and the attributes it names. It is the one parser the web
// add box, the command line, the HTTP API and the terminal interface share, so
// a line means the same task wherever it is typed.
//
// It resolves nothing. Handles, project keys, parents and status names are
// checked by the service exactly as the explicit fields are, which is where an
// unknown one is refused.
package quickadd

import (
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/query"
)

// Kind names the attribute one token sets.
type Kind string

// The attributes a line can set.
const (
	KindPriority Kind = "priority"
	KindTag      Kind = "tag"
	KindAssignee Kind = "assignee"
	KindDue      Kind = "due"
	KindProject  Kind = "project"
	KindParent   Kind = "parent"
	KindStatus   Kind = "status"
)

// Token is one recognised word: what it sets, as typed, and the value it
// stands for in the form a reader would recognise.
type Token struct {
	Kind  Kind   `json:"kind"`
	Raw   string `json:"raw"`
	Value string `json:"value"`

	priority core.Priority
	due      *time.Time
}

// Result is a parsed line.
type Result struct {
	Title    string
	Priority core.Priority
	Tags     []string
	Assignee string
	Due      *time.Time
	Project  string
	Parent   string
	Status   string
	Tokens   []Token
}

// Sigils are the characters that open a token at the start of a word, in the
// order the syntax is documented.
const Sigils = "!#@+^"

// keyed maps each key: form to the attribute it sets. The long forms are the
// filter language's own keys, so a reader who knows one knows the other.
var keyed = map[string]Kind{
	"priority": KindPriority, "tag": KindTag, "assignee": KindAssignee, "due": KindDue,
	"project": KindProject, "parent": KindParent, "status": KindStatus,
}

// quotes pairs every opening quote the tokenizer honours with its closing one.
// The Bulgarian and German low-high pair is here because a Bulgarian title is
// quoted that way far more often than with straight quotes.
var quotes = map[rune]rune{'"': '"', '„': '“', '“': '”', '«': '»'}

// word is one whitespace-separated piece of a line. literal marks a quoted run
// or an escaped token, which is title text whatever it holds.
type word struct {
	text    string
	literal bool
}

// Parse reads a line against now, whose location is the calendar that
// "tomorrow" and "fri" are counted in.
func Parse(line string, now time.Time) (Result, error) {
	var r Result
	var title []string
	for _, w := range split(line) {
		if w.literal {
			title = append(title, w.text)
			continue
		}
		if text, ok := escaped(w.text); ok {
			title = append(title, text)
			continue
		}
		tok, ok, err := classify(w.text, now)
		if err != nil {
			return Result{}, err
		}
		if !ok {
			title = append(title, w.text)
			continue
		}
		if err := r.set(tok); err != nil {
			return Result{}, err
		}
	}
	r.Title = strings.Join(title, " ")
	if r.Title == "" {
		if len(r.Tokens) == 0 {
			return Result{}, core.Invalid("task title is required").WithDetail(core.DetailField, "title")
		}
		return Result{}, core.Invalid("the line has no title left once %s is taken out", r.Tokens[0].Raw).
			WithDetail(core.DetailField, "title")
	}
	return r, nil
}

// split cuts a line into words, keeping a quoted run whole.
func split(line string) []word {
	var out []word
	runes := []rune(line)
	for i := 0; i < len(runes); {
		if unicode.IsSpace(runes[i]) {
			i++
			continue
		}
		if end, ok := quoteEnd(runes, i); ok {
			out = append(out, word{text: string(runes[i : end+1]), literal: true})
			i = end + 1
			continue
		}
		start := i
		for i < len(runes) && !unicode.IsSpace(runes[i]) {
			i++
		}
		out = append(out, word{text: string(runes[start:i])})
	}
	return out
}

// quoteEnd finds the closing quote of a run opened at start: the first one
// that ends a word. An opening quote with no partner is ordinary text.
func quoteEnd(runes []rune, start int) (int, bool) {
	closing, ok := quotes[runes[start]]
	if !ok {
		return 0, false
	}
	for i := start + 1; i < len(runes); i++ {
		if runes[i] == closing && (i+1 == len(runes) || unicode.IsSpace(runes[i+1])) {
			return i, true
		}
	}
	return 0, false
}

// escaped reports whether a word is a backslash before a token, answering the
// word without it. A backslash before anything else is left alone, so a path
// such as C:\temp survives.
func escaped(text string) (string, bool) {
	rest, ok := strings.CutPrefix(text, `\`)
	if !ok || rest == "" {
		return "", false
	}
	if strings.ContainsRune(Sigils, firstRune(rest)) {
		return rest, true
	}
	if key, _, ok := strings.Cut(rest, ":"); ok {
		if _, known := keyed[strings.ToLower(key)]; known {
			return rest, true
		}
	}
	return "", false
}

// classify decides what one word sets. A sigil whose value does not fit is not
// a token; a keyed form whose value does not fit is refused, because the key
// says the writer meant it.
func classify(text string, now time.Time) (Token, bool, error) {
	if key, value, ok := strings.Cut(text, ":"); ok {
		if kind, known := keyed[strings.ToLower(key)]; known && value != "" {
			tok, err := keyedToken(kind, text, value, now)
			return tok, err == nil, err
		}
	}
	if utf8.RuneCountInString(text) < 2 {
		return Token{}, false, nil
	}
	rest := text[1:]
	switch text[0] {
	case '!':
		if p, ok := sigilPriority(rest); ok {
			return Token{Kind: KindPriority, Raw: text, Value: PriorityName(p), priority: p}, true, nil
		}
	case '#':
		if validTag(rest) {
			return Token{Kind: KindTag, Raw: text, Value: rest}, true, nil
		}
	case '@':
		if validHandle(rest) {
			return Token{Kind: KindAssignee, Raw: text, Value: rest}, true, nil
		}
	case '+':
		if core.ValidateProjectKey(rest) == nil {
			return Token{Kind: KindProject, Raw: text, Value: strings.ToLower(rest)}, true, nil
		}
	case '^':
		if validParent(rest) {
			return Token{Kind: KindParent, Raw: text, Value: rest}, true, nil
		}
	}
	return Token{}, false, nil
}

// keyedToken reads the value of a key: form, refusing one that does not fit.
func keyedToken(kind Kind, raw, value string, now time.Time) (Token, error) {
	tok := Token{Kind: kind, Raw: raw, Value: value}
	switch kind {
	case KindPriority:
		p, err := query.ParsePriority(value)
		if err != nil {
			return Token{}, onTitle(err)
		}
		tok.Value, tok.priority = PriorityName(p), p
	case KindDue:
		due, err := ParseDue(value, now)
		if err != nil {
			return Token{}, err
		}
		tok.Value, tok.due = FormatDue(*due), due
	case KindProject:
		if err := core.ValidateProjectKey(value); err != nil {
			return Token{}, onTitle(err)
		}
		tok.Value = strings.ToLower(value)
	case KindParent:
		if _, err := core.ParseTaskRef(value); err != nil {
			return Token{}, onTitle(err)
		}
	}
	return tok, nil
}

// set records one token, refusing a second value for a single-valued field.
func (r *Result) set(tok Token) error {
	switch tok.Kind {
	case KindTag:
		if !slices.Contains(r.Tags, tok.Value) {
			r.Tags = append(r.Tags, tok.Value)
		}
	case KindPriority:
		if err := once(r.Tokens, tok, r.Priority != 0 && r.Priority != tok.priority); err != nil {
			return err
		}
		r.Priority = tok.priority
	case KindDue:
		if err := once(r.Tokens, tok, r.Due != nil && !r.Due.Equal(*tok.due)); err != nil {
			return err
		}
		r.Due = tok.due
	case KindAssignee:
		if err := once(r.Tokens, tok, r.Assignee != "" && r.Assignee != tok.Value); err != nil {
			return err
		}
		r.Assignee = tok.Value
	case KindProject:
		if err := once(r.Tokens, tok, r.Project != "" && r.Project != tok.Value); err != nil {
			return err
		}
		r.Project = tok.Value
	case KindParent:
		if err := once(r.Tokens, tok, r.Parent != "" && !strings.EqualFold(r.Parent, tok.Value)); err != nil {
			return err
		}
		r.Parent = tok.Value
	case KindStatus:
		if err := once(r.Tokens, tok, r.Status != "" && r.Status != tok.Value); err != nil {
			return err
		}
		r.Status = tok.Value
	}
	r.Tokens = append(r.Tokens, tok)
	return nil
}

// once refuses a token naming a different value for a field an earlier token
// already set.
func once(seen []Token, tok Token, clash bool) error {
	if !clash {
		return nil
	}
	for _, prior := range seen {
		if prior.Kind == tok.Kind {
			return core.Invalid("the line sets %s twice, %s and %s", tok.Kind, prior.Raw, tok.Raw).
				WithDetail(core.DetailField, "title")
		}
	}
	return nil
}

// Apply folds the result into an input the caller may already have filled.
// The title is replaced, tags are combined, and a field the caller set to a
// different value than the line is refused rather than silently overridden.
func (r Result) Apply(in core.CreateTaskInput) (core.CreateTaskInput, error) {
	in.Title = r.Title
	for _, tag := range r.Tags {
		if !slices.Contains(in.Tags, tag) {
			in.Tags = append(in.Tags, tag)
		}
	}
	if r.Priority != 0 {
		if in.Priority != 0 && in.Priority != r.Priority {
			return in, contradiction("priority", PriorityName(r.Priority), PriorityName(in.Priority))
		}
		in.Priority = r.Priority
	}
	if r.Due != nil {
		if in.DueAt != nil && !in.DueAt.Equal(*r.Due) {
			return in, contradiction("due_at", FormatDue(*r.Due), FormatDue(*in.DueAt))
		}
		in.DueAt = r.Due
	}
	var err error
	if in.ProjectRef, err = merge("project_ref", in.ProjectRef, r.Project, strings.EqualFold); err != nil {
		return in, err
	}
	if in.AssigneeActorID, err = merge("assignee_actor_id", in.AssigneeActorID, r.Assignee, eq); err != nil {
		return in, err
	}
	if in.ParentRef, err = merge("parent_ref", in.ParentRef, r.Parent, strings.EqualFold); err != nil {
		return in, err
	}
	if in.Status, err = merge("status", in.Status, r.Status, eq); err != nil {
		return in, err
	}
	return in, nil
}

// merge takes the line's value for a string field unless the caller already
// set a different one.
func merge(field, explicit, line string, same func(a, b string) bool) (string, error) {
	switch {
	case line == "":
		return explicit, nil
	case explicit == "" || same(explicit, line):
		return line, nil
	default:
		return explicit, contradiction(field, line, explicit)
	}
}

// eq is string equality, for the fields whose case matters.
func eq(a, b string) bool { return a == b }

// contradiction refuses a line that disagrees with an explicit field.
func contradiction(field, line, explicit string) error {
	return core.Invalid("the line sets %s to %s but it was also given as %s", field, line, explicit).
		WithDetail(core.DetailField, field)
}

// onTitle attributes a value error to the line, which is the title field.
func onTitle(err error) error { return core.OnField(err, "title") }

// sigilPriority reads the word after !: a priority name, or one digit from 1
// to 5. Anything else, "!important" or "!+3", is title text.
func sigilPriority(s string) (core.Priority, bool) {
	if len(s) == 1 && s[0] >= '1' && s[0] <= '5' {
		return core.Priority(s[0] - '0'), true
	}
	for _, c := range s {
		if !unicode.IsLetter(c) {
			return 0, false
		}
	}
	p, err := query.ParsePriority(s)
	return p, err == nil
}

// validTag reports whether a word after # is a tag: letters, digits and a few
// joiners, with at least one letter so an issue number such as #42 stays text.
func validTag(s string) bool {
	letter := false
	for i, c := range s {
		switch {
		case unicode.IsLetter(c):
			letter = true
		case unicode.IsDigit(c):
		case strings.ContainsRune("_-/.", c) && i > 0:
		default:
			return false
		}
	}
	return letter && !strings.ContainsRune("-/.", lastRune(s))
}

// validHandle reports whether a word after @ is a handle. A trailing dot is
// refused so "ask @raj." does not ask somebody called "raj.".
func validHandle(s string) bool {
	for i, c := range s {
		switch {
		case unicode.IsLetter(c), unicode.IsDigit(c):
		case strings.ContainsRune("_.-+", c) && i > 0:
		default:
			return false
		}
	}
	return !strings.ContainsRune("._-+", lastRune(s))
}

// validParent reports whether a word after ^ is a task reference in the
// key-number form a reader writes.
func validParent(s string) bool {
	ref, err := core.ParseTaskRef(s)
	return err == nil && ref.ProjectKey != ""
}

// firstRune is the first character of a non-empty string.
func firstRune(s string) rune {
	r, _ := utf8.DecodeRuneInString(s)
	return r
}

// lastRune is the last character of a non-empty string.
func lastRune(s string) rune {
	r, _ := utf8.DecodeLastRuneInString(s)
	return r
}

// PriorityName is the word the syntax spells a priority with.
func PriorityName(p core.Priority) string {
	switch p {
	case core.PriorityHighest:
		return "highest"
	case core.PriorityHigh:
		return "high"
	case core.PriorityNormal:
		return "normal"
	case core.PriorityLow:
		return "low"
	case core.PriorityLowest:
		return "lowest"
	default:
		return ""
	}
}

// SyntaxHint is the one line a surface offering quick add shows to list it.
func SyntaxHint() string {
	return "!high or !1-!5 priority, #tag, @handle, due:fri|tomorrow|+3d|2026-11-20, " +
		"+project, ^parent-ref, status:doing; \\ or quotes keep text literal"
}
