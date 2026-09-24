// SPDX-License-Identifier: AGPL-3.0-or-later

package query_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/query"
)

func TestParseActivityReadsEveryKey(t *testing.T) {
	t.Parallel()
	f, err := query.ParseActivity(`actor:u1 kind:task action:task.created type:task.updated source:web q:hello "two words" bare`)
	if err != nil {
		t.Fatalf("ParseActivity: %v", err)
	}
	if !reflect.DeepEqual(f.Actors, []string{"u1"}) {
		t.Errorf("actors = %v", f.Actors)
	}
	if !reflect.DeepEqual(f.Kinds, []string{"task"}) {
		t.Errorf("kinds = %v", f.Kinds)
	}
	if !reflect.DeepEqual(f.Actions, []string{"task.created", "task.updated"}) {
		t.Errorf("actions = %v", f.Actions)
	}
	if !reflect.DeepEqual(f.Sources, []core.Source{core.SourceWeb}) {
		t.Errorf("sources = %v", f.Sources)
	}
	if !reflect.DeepEqual(f.Text, []string{"hello", "two words", "bare"}) {
		t.Errorf("text = %v", f.Text)
	}
	if !f.Active() {
		t.Error("a filter with terms reported itself inactive")
	}
}

func TestParseActivityNegatesEveryKey(t *testing.T) {
	t.Parallel()
	f, err := query.ParseActivity(`-actor:u1 -kind:webhook -action:task.deleted -source:system -q:noise -bare`)
	if err != nil {
		t.Fatalf("ParseActivity: %v", err)
	}
	if !reflect.DeepEqual(f.Exclude.Actors, []string{"u1"}) {
		t.Errorf("excluded actors = %v", f.Exclude.Actors)
	}
	if !reflect.DeepEqual(f.Exclude.Kinds, []string{"webhook"}) {
		t.Errorf("excluded kinds = %v", f.Exclude.Kinds)
	}
	if !reflect.DeepEqual(f.Exclude.Actions, []string{"task.deleted"}) {
		t.Errorf("excluded actions = %v", f.Exclude.Actions)
	}
	if !reflect.DeepEqual(f.Exclude.Sources, []core.Source{core.SourceSystem}) {
		t.Errorf("excluded sources = %v", f.Exclude.Sources)
	}
	if !reflect.DeepEqual(f.Exclude.Text, []string{"noise", "bare"}) {
		t.Errorf("excluded text = %v", f.Exclude.Text)
	}
	if len(f.Actors)+len(f.Kinds)+len(f.Actions)+len(f.Sources)+len(f.Text) != 0 {
		t.Error("a wholly negated expression also selected something")
	}
}

func TestParseActivityRejectsBadExpressions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, expr, want string
	}{
		{"unknown key", "status:todo", `unknown activity filter key "status"`},
		{"empty value", "kind:", `has no value`},
		{"unknown source", "source:carrier-pigeon", `unknown source "carrier-pigeon"`},
		{"unterminated quote", `q:"open`, "unterminated quote"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := query.ParseActivity(tc.expr)
			if err == nil {
				t.Fatalf("ParseActivity(%q) was accepted", tc.expr)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("ParseActivity(%q) = %v, want it to mention %q", tc.expr, err, tc.want)
			}
			if core.KindOf(err) != core.KindInvalid {
				t.Errorf("ParseActivity(%q) reported kind %v, want invalid", tc.expr, core.KindOf(err))
			}
		})
	}
}

func TestParseActivityNamesTheKeysItAccepts(t *testing.T) {
	t.Parallel()
	_, err := query.ParseActivity("nope:1")
	if err == nil {
		t.Fatal("an unknown key was accepted")
	}
	for _, key := range query.ActivityKeys {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("the refusal does not offer %q: %v", key, err)
		}
	}
}

func TestParseSourceAcceptsEveryRecordedSurface(t *testing.T) {
	t.Parallel()
	for _, want := range query.ActivitySources {
		got, err := query.ParseSource(strings.ToUpper(string(want)))
		if err != nil {
			t.Fatalf("ParseSource(%q): %v", want, err)
		}
		if got != want {
			t.Errorf("ParseSource(%q) = %q", want, got)
		}
	}
}

// row is one activity row with every field set, which each case narrows.
func row() query.ActivityRow {
	return query.ActivityRow{
		Actors: []string{"u1", "ada"},
		Kind:   "task",
		Action: "task.created",
		Source: core.SourceCLI,
		Text:   []string{"task.created", "infra-3", "ship the thing"},
	}
}

func TestActivityFilterMatches(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, expr string
		want       bool
	}{
		{"empty selects everything", "", true},
		{"actor by id", "actor:u1", true},
		{"actor by handle", "actor:ada", true},
		{"actor ignores case", "actor:ADA", true},
		{"another actor", "actor:bob", false},
		{"either actor", "actor:bob actor:ada", true},
		{"kind", "kind:task", true},
		{"other kind", "kind:comment", false},
		{"action", "action:task.created", true},
		{"type alias", "type:task.created", true},
		{"source", "source:cli", true},
		{"other source", "source:web", false},
		{"text", "ship", true},
		{"text is case insensitive", "SHIP", true},
		{"every word must appear", "ship missing", false},
		{"quoted phrase", `"ship the thing"`, true},
		{"excluded actor", "-actor:ada", false},
		{"excluded other actor", "-actor:bob", true},
		{"excluded kind", "-kind:task", false},
		{"excluded action", "-action:task.created", false},
		{"excluded source", "-source:cli", false},
		{"excluded text", "-ship", false},
		{"excluded text that is absent", "-missing", true},
		{"terms are conjunctions", "kind:task source:web", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f, err := query.ParseActivity(tc.expr)
			if err != nil {
				t.Fatalf("ParseActivity(%q): %v", tc.expr, err)
			}
			if got := f.Matches(row()); got != tc.want {
				t.Errorf("Matches(%q) = %v, want %v", tc.expr, got, tc.want)
			}
		})
	}
}

func TestAuditRowCarriesWhatTheFilterAsksAbout(t *testing.T) {
	t.Parallel()
	entry := core.AuditEntry{
		ActorID: "u7", Action: "task.updated", SubjectType: "task", SubjectID: "abc",
		Source: core.SourceTUI, After: []byte(`{"title":"rotate the certificates"}`),
	}
	r := query.AuditRow(entry)
	if r.Kind != "task" || r.Action != "task.updated" || r.Source != core.SourceTUI {
		t.Fatalf("AuditRow lost a field: %+v", r)
	}
	f, err := query.ParseActivity("actor:u7 kind:task source:tui certificates")
	if err != nil {
		t.Fatalf("ParseActivity: %v", err)
	}
	if !f.Matches(r) {
		t.Error("an entry the filter describes did not match")
	}
	if snapshot, _ := query.ParseActivity("rotate"); !snapshot.Matches(r) {
		t.Error("free text did not reach the entry's snapshot")
	}
}

func TestActivityFilterPushesWhatTheStoreCanAnswer(t *testing.T) {
	t.Parallel()
	f, err := query.ParseActivity("actor:u1 kind:task action:task.created source:web -kind:comment")
	if err != nil {
		t.Fatalf("ParseActivity: %v", err)
	}
	got := f.AuditFilter(core.AuditFilter{ActorIDs: []string{"u9"}})
	if !reflect.DeepEqual(got.ActorIDs, []string{"u9", "u1"}) {
		t.Errorf("actor ids = %v, want the flag's and the filter's", got.ActorIDs)
	}
	if got.SubjectType != "task" {
		t.Errorf("subject type = %q, want the single kind pushed down", got.SubjectType)
	}
	if !reflect.DeepEqual(got.Actions, []string{"task.created"}) {
		t.Errorf("actions = %v", got.Actions)
	}
	if !reflect.DeepEqual(got.Sources, []core.Source{core.SourceWeb}) {
		t.Errorf("sources = %v", got.Sources)
	}
}

func TestActivityFilterLeavesASubjectTypeTheCallerNamed(t *testing.T) {
	t.Parallel()
	f, err := query.ParseActivity("kind:task")
	if err != nil {
		t.Fatalf("ParseActivity: %v", err)
	}
	got := f.AuditFilter(core.AuditFilter{SubjectType: "comment"})
	if got.SubjectType != "comment" {
		t.Errorf("subject type = %q, want the caller's flag kept", got.SubjectType)
	}
	entry := core.AuditEntry{SubjectType: "comment"}
	if f.Matches(query.AuditRow(entry)) {
		t.Error("a row the filter contradicts was matched anyway")
	}
}

func TestActivityFilterDoesNotPushTwoKinds(t *testing.T) {
	t.Parallel()
	f, err := query.ParseActivity("kind:task kind:comment")
	if err != nil {
		t.Fatalf("ParseActivity: %v", err)
	}
	if got := f.AuditFilter(core.AuditFilter{}); got.SubjectType != "" {
		t.Errorf("subject type = %q, want nothing pushed for two kinds", got.SubjectType)
	}
	for _, kind := range []string{"task", "comment"} {
		if !f.Matches(query.AuditRow(core.AuditEntry{SubjectType: kind})) {
			t.Errorf("kind %q was not matched in memory", kind)
		}
	}
}

func TestUnsupportedForEventsNamesSource(t *testing.T) {
	t.Parallel()
	for _, expr := range []string{"source:web", "-source:web"} {
		f, err := query.ParseActivity(expr)
		if err != nil {
			t.Fatalf("ParseActivity(%q): %v", expr, err)
		}
		if got := f.UnsupportedForEvents(); !reflect.DeepEqual(got, []string{"source"}) {
			t.Errorf("UnsupportedForEvents(%q) = %v, want [source]", expr, got)
		}
	}
	f, err := query.ParseActivity("kind:task actor:u1 word")
	if err != nil {
		t.Fatalf("ParseActivity: %v", err)
	}
	if got := f.UnsupportedForEvents(); got != nil {
		t.Errorf("UnsupportedForEvents = %v, want nothing", got)
	}
	if (query.ActivityFilter{}).Active() {
		t.Error("an empty filter reported itself active")
	}
}

func TestActivitySyntaxHintNamesTheOperators(t *testing.T) {
	t.Parallel()
	hint := query.ActivitySyntaxHint()
	if !strings.Contains(hint, query.NegationPrefix) {
		t.Errorf("the hint does not show the negation prefix: %q", hint)
	}
}
