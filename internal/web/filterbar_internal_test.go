// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestFoldPicksReplacesOnlyWhatChanged(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		q    string
		form url.Values
		want string
	}{
		{"a changed priority replaces it", "tag:ops priority:low",
			url.Values{"pick_priority": {"high"}, "was_priority": {"low"}}, "tag:ops priority:high"},
		{"an untouched picker leaves a typed term", "status:done",
			url.Values{"pick_status": {"todo"}, "was_status": {"todo"}}, "status:done"},
		{"the empty choice removes the terms", "s:todo tag:ops -status:done",
			url.Values{"pick_status": {""}, "was_status": {"todo"}}, "tag:ops -status:done"},
		{"as typed keeps several", "status:todo status:doing",
			url.Values{"pick_status": {"*"}, "was_status": {"*"}}, "status:todo status:doing"},
		{"an assignee replaces an at sign", "@raj tag:ops",
			url.Values{"pick_assignee": {"@lena"}, "was_assignee": {"raj"}}, "tag:ops assignee:lena"},
		{"a tag is added once", "tag:ops",
			url.Values{"pick_tag": {"ops"}}, "tag:ops"},
		{"a new tag is added", "tag:ops",
			url.Values{"pick_tag": {"#two words"}}, `tag:ops tag:"two words"`},
		{"nothing picked", "free text", url.Values{"pick_tag": {""}}, "free text"},
	} {
		if got := foldPicks(tc.q, tc.form); got != tc.want {
			t.Errorf("%s: foldPicks(%q) = %q, want %q", tc.name, tc.q, got, tc.want)
		}
	}
}

func TestEachChipRemovesExactlyItsOwnTerm(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodGet, "/tasks?"+url.Values{
		"q": {`tag:ops -tag:chore title:"two words" @raj`}, "due": {"week"}, "project": {"web,ops"},
		"all": {"1"}, "sort": {"title"}, "order": {"desc"}, "view": {"board"}, "page": {"3"}, "cursor": {"abc"},
	}.Encode(), nil)
	chips := filterChips(r, "/tasks", false)
	want := []struct {
		label string
		q     url.Values
	}{
		{"due: week", url.Values{"q": {`tag:ops -tag:chore title:"two words" @raj`}, "project": {"web,ops"}, "all": {"1"}}},
		{"project: web", url.Values{"q": {`tag:ops -tag:chore title:"two words" @raj`}, "due": {"week"}, "project": {"ops"}, "all": {"1"}}},
		{"project: ops", url.Values{"q": {`tag:ops -tag:chore title:"two words" @raj`}, "due": {"week"}, "project": {"web"}, "all": {"1"}}},
		{"tag: ops", url.Values{"q": {`-tag:chore title:"two words" @raj`}, "due": {"week"}, "project": {"web,ops"}, "all": {"1"}}},
		{"-tag: chore", url.Values{"q": {`tag:ops title:"two words" @raj`}, "due": {"week"}, "project": {"web,ops"}, "all": {"1"}}},
		{"title: two words", url.Values{"q": {`tag:ops -tag:chore @raj`}, "due": {"week"}, "project": {"web,ops"}, "all": {"1"}}},
		{"assignee: raj", url.Values{"q": {`tag:ops -tag:chore title:"two words"`}, "due": {"week"}, "project": {"web,ops"}, "all": {"1"}}},
		{"all: ", url.Values{"q": {`tag:ops -tag:chore title:"two words" @raj`}, "due": {"week"}, "project": {"web,ops"}}},
	}
	if len(chips) != len(want) {
		t.Fatalf("got %d chips, want %d: %+v", len(chips), len(want), chips)
	}
	for i, w := range want {
		c := chips[i]
		if c.Key != "all" && c.Label() != w.label {
			t.Errorf("chip %d reads %q, want %q", i, c.Label(), w.label)
		}
		u, err := url.Parse(c.Href)
		if err != nil || u.Path != "/tasks" {
			t.Fatalf("chip %d links to %q", i, c.Href)
		}
		got := u.Query()
		for _, kept := range []string{"sort", "order", "view"} {
			if got.Get(kept) == "" {
				t.Errorf("chip %d drops %s: %s", i, kept, c.Href)
			}
		}
		for _, gone := range []string{"page", "cursor"} {
			if got.Has(gone) {
				t.Errorf("chip %d keeps %s, so the filter change resumes mid-walk: %s", i, gone, c.Href)
			}
		}
		for _, k := range []string{"q", "due", "project", "all"} {
			if got.Get(k) != w.q.Get(k) {
				t.Errorf("chip %d (%s): %s = %q, want %q", i, w.label, k, got.Get(k), w.q.Get(k))
			}
		}
	}
}

func TestAProjectPageHasNoProjectChip(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodGet, "/projects/infra?project=web&q=tag:ops", nil)
	chips := filterChips(r, "/projects/infra", true)
	if len(chips) != 1 || chips[0].Key != "tag" {
		t.Fatalf("project page chips = %+v, want only the tag", chips)
	}
}

func TestPickStatesShowOneTermOrSeveral(t *testing.T) {
	t.Parallel()
	s := pickStates("status:todo priority:high priority:low -status:done @raj")
	if s["status"] != (pickState{Value: "todo"}) || !s["priority"].Several || s["assignee"].Value != "raj" {
		t.Errorf("pickStates = %+v", s)
	}
}
