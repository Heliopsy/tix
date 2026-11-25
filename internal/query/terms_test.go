// SPDX-License-Identifier: AGPL-3.0-or-later

package query

import (
	"slices"
	"testing"
)

func TestAtHandleIsAnAssigneeTerm(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		expr    string
		include []string
		exclude []string
		text    string
	}{
		{expr: "@raj tag:ops", include: []string{"raj"}},
		{expr: "-@raj", exclude: []string{"raj"}},
		{expr: `"@raj"`, text: "@raj"},
		{expr: "@", text: "@"},
		{expr: "@raj @lena", include: []string{"raj", "lena"}},
	} {
		f, err := Parse(tc.expr)
		if err != nil {
			t.Fatalf("%q: %v", tc.expr, err)
		}
		if !slices.Equal(f.AssigneeIDs, tc.include) || !slices.Equal(f.Exclude.AssigneeIDs, tc.exclude) || f.Query != tc.text {
			t.Errorf("%q: assignees %v, excluded %v, text %q; want %v, %v, %q",
				tc.expr, f.AssigneeIDs, f.Exclude.AssigneeIDs, f.Query, tc.include, tc.exclude, tc.text)
		}
	}
}

func TestTermsKeepTheRawWordsAndCanonicalKeys(t *testing.T) {
	t.Parallel()
	got := Terms(`p:web  -tag:chore title:"two words" @raj s:todo "free text" title~api word`)
	want := []Term{
		{Raw: "p:web", Key: "project", Value: "web"},
		{Raw: "-tag:chore", Key: "tag", Value: "chore", Negated: true},
		{Raw: `title:"two words"`, Key: "title", Value: "two words"},
		{Raw: "@raj", Key: "assignee", Value: "raj"},
		{Raw: "s:todo", Key: "status", Value: "todo"},
		{Raw: `"free text"`, Value: "free text"},
		{Raw: "title~api", Key: "title", Value: "api", Weak: true},
		{Raw: "word", Value: "word"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Terms =\n%+v\nwant\n%+v", got, want)
	}
	if j := Join(got[1:3]); j != `-tag:chore title:"two words"` {
		t.Errorf("Join = %q", j)
	}
}

func TestTermsSurviveAnUnterminatedQuote(t *testing.T) {
	t.Parallel()
	got := Terms(`tag:ops "open end`)
	if len(got) != 2 || got[1].Raw != `"open end` {
		t.Fatalf("Terms = %+v", got)
	}
}

func TestQuoteWrapsAValueWithASpace(t *testing.T) {
	t.Parallel()
	if q := Quote("tag", "two words"); q != `tag:"two words"` {
		t.Errorf("Quote = %q", q)
	}
	if q := Quote("tag", "ops"); q != "tag:ops" {
		t.Errorf("Quote = %q", q)
	}
}
