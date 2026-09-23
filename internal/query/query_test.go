// SPDX-License-Identifier: AGPL-3.0-or-later

package query

import (
	"reflect"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestParseNegationAndWeakMatch(t *testing.T) {
	tests := []struct {
		name  string
		expr  string
		check func(t *testing.T, f core.TaskFilter)
	}{
		{"a leading minus excludes a tag", "-tag:ops", func(t *testing.T, f core.TaskFilter) {
			if !reflect.DeepEqual(f.Exclude.Tags, []string{"ops"}) {
				t.Fatalf("excluded tags %v", f.Exclude.Tags)
			}
			if len(f.Tags) != 0 {
				t.Fatalf("a negated term must not also select: %v", f.Tags)
			}
		}},
		{"a leading minus excludes a status", "-status:done", func(t *testing.T, f core.TaskFilter) {
			if !reflect.DeepEqual(f.Exclude.Statuses, []string{"done"}) {
				t.Fatalf("excluded statuses %v", f.Exclude.Statuses)
			}
		}},
		{"selection and exclusion coexist", "status:todo -tag:ops", func(t *testing.T, f core.TaskFilter) {
			if !reflect.DeepEqual(f.Statuses, []string{"todo"}) ||
				!reflect.DeepEqual(f.Exclude.Tags, []string{"ops"}) {
				t.Fatalf("filter %+v", f)
			}
		}},
		{"a negated priority excludes", "-priority:high", func(t *testing.T, f core.TaskFilter) {
			if !reflect.DeepEqual(f.Exclude.Priorities, []core.Priority{core.PriorityHigh}) {
				t.Fatalf("excluded priorities %v", f.Exclude.Priorities)
			}
		}},
		{"a negated project is lowercased like a selected one", "-project:INFRA", func(t *testing.T, f core.TaskFilter) {
			if !reflect.DeepEqual(f.Exclude.ProjectKeys, []string{"infra"}) {
				t.Fatalf("excluded projects %v", f.Exclude.ProjectKeys)
			}
		}},
		{"tilde is a weak title match", "title~api", func(t *testing.T, f core.TaskFilter) {
			wantTerm(t, f, core.TextTerm{Field: core.TextTitle, Mode: core.MatchContains, Value: "api"})
		}},
		{"colon is an exact title match", "title:api", func(t *testing.T, f core.TaskFilter) {
			wantTerm(t, f, core.TextTerm{Field: core.TextTitle, Mode: core.MatchExact, Value: "api"})
		}},
		{"a weak body match", "body~gateway", func(t *testing.T, f core.TaskFilter) {
			wantTerm(t, f, core.TextTerm{Field: core.TextBody, Mode: core.MatchContains, Value: "gateway"})
		}},
		{"a weak text match spans both fields", "text~deploy", func(t *testing.T, f core.TaskFilter) {
			wantTerm(t, f, core.TextTerm{Field: core.TextAny, Mode: core.MatchContains, Value: "deploy"})
		}},
		{"a negated weak match", "-title~api", func(t *testing.T, f core.TaskFilter) {
			wantTerm(t, f, core.TextTerm{Field: core.TextTitle, Mode: core.MatchContains, Value: "api", Negate: true})
		}},
		{"a negated bare word becomes a negated substring", "-spam", func(t *testing.T, f core.TaskFilter) {
			wantTerm(t, f, core.TextTerm{Field: core.TextAny, Mode: core.MatchContains, Value: "spam", Negate: true})
			if f.Query != "" {
				t.Fatalf("a negated word must not feed the native search: %q", f.Query)
			}
		}},
		{"a plain word still feeds the native search", "deploy", func(t *testing.T, f core.TaskFilter) {
			if f.Query != "deploy" {
				t.Fatalf("query %q", f.Query)
			}
			if len(f.Text) != 0 {
				t.Fatalf("a plain word must not become an explicit term: %+v", f.Text)
			}
		}},
		{"a quoted value keeps its spaces", `title:"rotate api keys"`, func(t *testing.T, f core.TaskFilter) {
			wantTerm(t, f, core.TextTerm{Field: core.TextTitle, Mode: core.MatchExact, Value: "rotate api keys"})
		}},
		{"a quote that opens the token is free text", `"status:todo"`, func(t *testing.T, f core.TaskFilter) {
			if f.Query != "status:todo" || len(f.Statuses) != 0 {
				t.Fatalf("filter %+v", f)
			}
		}},
		{"negated is: inverts the tri-state", "-is:claimed", func(t *testing.T, f core.TaskFilter) {
			if f.Claimed != core.No {
				t.Fatalf("claimed %v, want No", f.Claimed)
			}
		}},
		{"negated is:deleted keeps deleted tasks out", "-is:deleted", func(t *testing.T, f core.TaskFilter) {
			if f.IncludeDeleted {
				t.Fatal("-is:deleted must not include deleted tasks")
			}
		}},
		{"two weak terms both survive", "title~api body~roll", func(t *testing.T, f core.TaskFilter) {
			if len(f.Text) != 2 {
				t.Fatalf("terms %+v, want two", f.Text)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := Parse(tt.expr)
			if err != nil {
				t.Fatalf("parsing %q: %v", tt.expr, err)
			}
			tt.check(t, f)
		})
	}
}

func TestParseRejects(t *testing.T) {
	tests := []struct {
		name string
		expr string
	}{
		{"the weak operator on a key that has no text", "status~todo"},
		{"the weak operator on a tag", "tag~ops"},
		{"negating a sort", "-sort:title"},
		{"negating a limit", "-limit:10"},
		{"negating a due bound", "-due-before:2026-01-01"},
		{"negating a parent", "-parent:infra-1"},
		{"negating is:root", "-is:root"},
		{"an unknown key", "colour:red"},
		{"an unknown is: value", "is:purple"},
		{"an empty value", "status:"},
		{"an unterminated quote", `title:"unclosed`},
		{"a priority out of range", "priority:9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Parse(tt.expr); !core.IsKind(err, core.KindInvalid) {
				t.Fatalf("parsing %q = %v, want invalid", tt.expr, err)
			}
		})
	}
}

// A round trip through the wire form is what lets the HTTP API carry a term
// without a parameter per field and mode.
func TestTextTermRoundTripsThroughItsWireForm(t *testing.T) {
	terms := []core.TextTerm{
		{Field: core.TextTitle, Mode: core.MatchContains, Value: "api"},
		{Field: core.TextBody, Mode: core.MatchExact, Value: "done"},
		{Field: core.TextAny, Mode: core.MatchContains, Value: "two words", Negate: true},
		{Field: core.TextTitle, Mode: core.MatchContains, Value: "a:colon~and~tildes"},
	}
	for _, want := range terms {
		got, err := core.ParseTextTerm(want.String())
		if err != nil {
			t.Fatalf("parsing %q: %v", want.String(), err)
		}
		if got != want {
			t.Errorf("round trip of %+v gave %+v via %q", want, got, want.String())
		}
	}
}

func TestParseTextTermRejectsGarbage(t *testing.T) {
	for _, raw := range []string{"", "nofield", "colour~red", "title~", "-title~"} {
		if _, err := core.ParseTextTerm(raw); !core.IsKind(err, core.KindInvalid) {
			t.Errorf("parsing %q = %v, want invalid", raw, err)
		}
	}
}

func wantTerm(t *testing.T, f core.TaskFilter, want core.TextTerm) {
	t.Helper()
	if len(f.Text) != 1 {
		t.Fatalf("text terms %+v, want exactly one", f.Text)
	}
	if f.Text[0] != want {
		t.Fatalf("term %+v, want %+v", f.Text[0], want)
	}
}

// A key list alone never suggests that a term can be negated or matched
// weakly, so the hint has to name both operators for a surface to be able to
// show them.
func TestSyntaxHintNamesBothOperators(t *testing.T) {
	hint := SyntaxHint()
	for _, want := range []string{NegationPrefix + "tag:ops", "title" + WeakOperator + "api"} {
		if !strings.Contains(hint, want) {
			t.Errorf("hint %q does not show %q", hint, want)
		}
	}
}
