// SPDX-License-Identifier: AGPL-3.0-or-later

package taskio

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

func TestSuggestPriority(t *testing.T) {
	tests := []struct {
		in   string
		want core.Priority
		ok   bool
	}{
		{"Highest", core.PriorityHighest, true}, {"high", core.PriorityHigh, true}, {"Medium", core.PriorityNormal, true},
		{"normal", core.PriorityNormal, true}, {"LOW", core.PriorityLow, true}, {"lowest", core.PriorityLowest, true},
		{"1", core.PriorityHighest, true}, {"5", core.PriorityLowest, true}, {"6", 0, false}, {"0", 0, false},
		{"P0", core.PriorityHighest, true}, {"p1", core.PriorityHigh, true}, {"P2", core.PriorityNormal, true},
		{"P3", core.PriorityLow, true}, {"P4", core.PriorityLowest, true},
		{"Blocker", core.PriorityHighest, true}, {"Critical", core.PriorityHighest, true}, {"Major", core.PriorityHigh, true},
		{"Minor", core.PriorityLow, true}, {"Trivial", core.PriorityLowest, true}, {"Urgent", core.PriorityHighest, true},
		{"No priority", 0, true}, {"Висок", core.PriorityHigh, true}, {"", 0, false}, {"soon", 0, false},
	}
	for _, tt := range tests {
		got, ok := SuggestPriority(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Errorf("SuggestPriority(%q) = %d, %v; want %d, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestParseDate(t *testing.T) {
	utc := func(y int, m time.Month, d, h, min, s int) time.Time {
		return time.Date(y, m, d, h, min, s, 0, time.UTC)
	}
	tests := []struct {
		in, format string
		want       time.Time
		err        string
	}{
		{"2026-03-04T10:00:00+02:00", "", utc(2026, 3, 4, 8, 0, 0), ""},
		{"2026-02-01T12:00:00.000Z", "", utc(2026, 2, 1, 12, 0, 0), ""},
		{"2026-03-04T10:30:15", "", utc(2026, 3, 4, 10, 30, 15), ""},
		{"2026-03-04 10:30", "", utc(2026, 3, 4, 10, 30, 0), ""},
		{"2026-03-04", "", utc(2026, 3, 4, 0, 0, 0), ""},
		{"02/Jan/26 3:04 PM", "", utc(2026, 1, 2, 15, 4, 0), ""},
		{"2/Jan/26", "", utc(2026, 1, 2, 0, 0, 0), ""},
		{"25/03/2026", "", utc(2026, 3, 25, 0, 0, 0), ""},
		{"03/25/2026", "", utc(2026, 3, 25, 0, 0, 0), ""},
		{"04/04/2026 09:15", "", utc(2026, 4, 4, 9, 15, 0), ""},
		{"03.04.2026", "", utc(2026, 4, 3, 0, 0, 0), ""},
		{"03/04/2026", "", time.Time{}, "could be day/month or month/day; set date_format"},
		{"31/02/2026", "", time.Time{}, "is not a date tix understands"},
		{"next week", "", time.Time{}, "is not a date tix understands"},
		{"03/04/2026", "DD/MM/YYYY", utc(2026, 4, 3, 0, 0, 0), ""},
		{"03/04/2026", "MM/DD/YYYY", utc(2026, 3, 4, 0, 0, 0), ""},
		{"3.4.26 14:05", "D.M.YY HH:mm", utc(2026, 4, 3, 14, 5, 0), ""},
		{"2026-03-04", "DD/MM/YYYY", utc(2026, 3, 4, 0, 0, 0), ""},
		{"03/04/2026", "iso", time.Time{}, "does not match date_format iso"},
		{"x", "DD/MM/YYYY", time.Time{}, "does not match date_format DD/MM/YYYY"},
		{"x", "Q", time.Time{}, "not one of the tokens"},
	}
	for _, tt := range tests {
		got, err := ParseDate(tt.in, tt.format)
		if tt.err != "" {
			if err == nil || !strings.Contains(err.Error(), tt.err) {
				t.Errorf("ParseDate(%q, %q) error = %v, want %q", tt.in, tt.format, err, tt.err)
			}
			continue
		}
		if err != nil || !got.Equal(tt.want) || got.Location() != time.UTC {
			t.Errorf("ParseDate(%q, %q) = %v, %v; want %v", tt.in, tt.format, got, err, tt.want)
		}
	}
}

func TestDraft(t *testing.T) {
	headers := []string{"Title", "Desc", "Notes", "State", "Prio", "Owner", "Tags", "Tags (2)", "Due", "Parent", "Deps", "Repeat", "Key", "Ref", "Old id", "Pts", "Pts (2)", "Sprint", "Skip"}
	columns := map[string]Target{
		"Title": TargetTitle, "Desc": TargetBody, "Notes": TargetBody, "State": TargetStatus, "Prio": TargetPriority,
		"Owner": TargetAssignee, "Tags": TargetTags, "Tags (2)": TargetTags, "Due": TargetDue, "Parent": TargetParent,
		"Deps": TargetDependsOn, "Repeat": TargetRecurrence, "Key": TargetExternalID, "Ref": TargetRef,
		"Old id": TargetAlias, "Pts": CustomTarget("pts"), "Pts (2)": CustomTarget("pts"), "Sprint": TargetAppendBody,
		"Skip": TargetIgnore,
	}
	row := func(vals map[string]string) Row { return Row{Line: 7, Values: vals, Headers: headers} }
	full := map[string]string{
		"Title": " Ship it ", "Desc": "First.", "Notes": "Second.", "State": "in progress", "Prio": "P1",
		"Owner": "Jane.Doe@example.com, other", "Tags": "a; b | c", "Tags (2)": "b, Legacy", "Due": "2026-05-01",
		"Parent": "K-1", "Deps": "K-2, K-3;K-4 K-2", "Repeat": "weekly", "Key": "K-9", "Ref": "P-9", "Old id": "1009",
		"Pts": "3", "Pts (2)": "large", "Sprint": "S1", "Skip": "noise",
	}
	tests := []struct {
		name  string
		m     Mapping
		vals  map[string]string
		check func(t *testing.T, d Draft)
	}{
		{
			name: "every target", vals: full,
			m: Mapping{
				Values: map[Target]map[string]string{
					TargetStatus:        {"In Progress": "doing"},
					TargetAssignee:      {"jane.doe@example.com, other": "jane"},
					TargetTags:          {"legacy": "", "a": "alpha"},
					CustomTarget("pts"): {"LARGE": "8"},
				},
			},
			check: func(t *testing.T, d Draft) {
				want := Draft{
					Line: 7, ExternalID: "K-9", Aliases: []string{"1009"}, Ref: "P-9", Title: "Ship it",
					Body: "First.\n\nSecond.\n\n**Sprint:** S1", Status: "doing", StatusRaw: "in progress",
					Priority: core.PriorityHigh, PriorityRaw: "P1", Assignee: "jane", AssigneeRaw: "Jane.Doe@example.com, other",
					Tags: []string{"alpha", "b", "c"}, Due: day(2026, 5, 1, 0, 0), Parent: "K-1",
					DependsOn: []string{"K-2", "K-3", "K-4"}, Recurrence: "weekly", Custom: map[string]string{"pts": "3, 8"},
				}
				if !reflect.DeepEqual(d, want) {
					t.Errorf("draft =\n%+v\nwant\n%+v", d, want)
				}
			},
		},
		{
			name: "defaults and first assignee", vals: map[string]string{"Title": "T", "Owner": "ann, bob"},
			m: Mapping{Defaults: map[Target]string{TargetStatus: "todo", TargetPriority: "low"}},
			check: func(t *testing.T, d Draft) {
				if d.Status != "todo" || d.Priority != core.PriorityLow || d.Assignee != "ann" || d.Body != "" || d.Errors != nil {
					t.Errorf("draft = %+v", d)
				}
			},
		},
		{
			name: "empty title and bad date", vals: map[string]string{"Due": "03/04/2026", "Sprint": "S2"},
			check: func(t *testing.T, d Draft) {
				if len(d.Errors) != 2 || d.Errors[0] != "title is empty" || !strings.Contains(d.Errors[1], "due date \"03/04/2026\" could be") {
					t.Errorf("errors = %q", d.Errors)
				}
				if d.Body != "**Sprint:** S2" || d.Due != nil {
					t.Errorf("draft = %+v", d)
				}
			},
		},
		{
			name: "custom tags separator", vals: map[string]string{"Title": "T", "Tags": "a b, c/d", "Tags (2)": " / x"},
			m: Mapping{TagsSeparator: "/"},
			check: func(t *testing.T, d Draft) {
				if !reflect.DeepEqual(d.Tags, []string{"a b, c", "d", "x"}) {
					t.Errorf("tags = %q", d.Tags)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, tt.m.Draft(row(tt.vals), columns))
		})
	}
}

func TestDraftUnknownPriority(t *testing.T) {
	columns := map[string]Target{"t": TargetTitle, "p": TargetPriority}
	row := Row{Line: 3, Values: map[string]string{"t": "T", "p": "soon"}}
	tests := []struct {
		policy   UnknownPolicy
		defaults map[Target]string
		priority core.Priority
		errors   int
		skip     bool
	}{
		{"", nil, 0, 0, false},
		{PolicyDefault, map[Target]string{TargetPriority: "high"}, core.PriorityHigh, 0, false},
		{PolicyError, nil, 0, 1, false},
		{PolicyKeep, nil, 0, 1, false},
		{PolicySkipRow, nil, 0, 0, true},
	}
	for _, tt := range tests {
		m := Mapping{Defaults: tt.defaults}
		if tt.policy != "" {
			m.Unknown = map[Target]UnknownPolicy{TargetPriority: tt.policy}
		}
		d := m.Draft(row, columns)
		if d.Priority != tt.priority || len(d.Errors) != tt.errors || (d.Skip != "") != tt.skip {
			t.Errorf("policy %q: draft = %+v", tt.policy, d)
		}
		if tt.skip && !strings.Contains(d.Skip, `priority "soon" is not recognised`) {
			t.Errorf("skip reason = %q", d.Skip)
		}
	}
	mapped := Mapping{Values: map[Target]map[string]string{TargetPriority: {"SOON": "2"}}}
	if d := mapped.Draft(row, columns); d.Priority != core.PriorityHigh || d.Errors != nil {
		t.Errorf("mapped priority draft = %+v", d)
	}
}

func TestDraftWithoutHeadersUsesSortedColumns(t *testing.T) {
	columns := map[string]Target{"b": TargetBody, "a": TargetBody, "t": TargetTitle, "x": "custom:"}
	d := (&Mapping{}).Draft(Row{Values: map[string]string{"a": "A", "b": "B", "t": "T", "x": "X"}}, columns)
	if d.Body != "A\n\nB" || d.Custom != nil {
		t.Errorf("draft = %+v", d)
	}
}
