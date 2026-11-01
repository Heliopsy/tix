// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"strings"
	"testing"
	"time"
)

func TestFillPlaceholders(t *testing.T) {
	v := TemplateValues{Date: time.Date(2026, 11, 1, 23, 30, 0, 0, time.FixedZone("x", 3*3600)), N: 7, Project: "ops"}
	cases := []struct{ in, want string }{
		{"plain", "plain"},
		{"{DATE}", "2026-11-01"},
		{"{date} {Date}", "2026-11-01 2026-11-01"},
		{"#{N} of {n}", "#7 of 7"},
		{"{PROJECT}/{project}", "ops/ops"},
		{"{other} {N", "{other} {N"},
		{"{{N}}", "{7}"},
	}
	for _, tc := range cases {
		if got := FillPlaceholders(tc.in, v); got != tc.want {
			t.Errorf("FillPlaceholders(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTemplateInputsValidate(t *testing.T) {
	long := strings.Repeat("x", MaxTemplateNameLength+1)
	neg, big := Duration(-1), MaxTemplateDueIn+1
	blank, prio := " ", Priority(9)
	cases := []struct {
		name  string
		err   error
		field string
	}{
		{"save without name", SaveTemplateInput{TaskRef: "a-1"}.Validate(), "name"},
		{"save with a long name", SaveTemplateInput{Name: long, TaskRef: "a-1"}.Validate(), "name"},
		{"save without task", SaveTemplateInput{Name: "n"}.Validate(), "task_ref"},
		{"save with a negative offset", SaveTemplateInput{Name: "n", TaskRef: "a-1", DueIn: neg}.Validate(), "due_in"},
		{"update to a blank title", UpdateTemplateInput{Title: &blank}.Validate(), "title"},
		{"update to a blank name", UpdateTemplateInput{Name: &blank}.Validate(), "name"},
		{"update to a bad priority", UpdateTemplateInput{Priority: &prio}.Validate(), "priority"},
		{"update past the offset bound", UpdateTemplateInput{DueIn: &big}.Validate(), "due_in"},
		{"use with a long title", UseTemplateInput{Title: strings.Repeat("x", MaxTitleLength+1)}.Validate(), "title"},
		{"use with a negative offset", UseTemplateInput{DueIn: &neg}.Validate(), "due_in"},
	}
	for _, tc := range cases {
		if !IsKind(tc.err, KindInvalid) || FieldOf(tc.err) != tc.field {
			t.Errorf("%s: %v (field %q), want invalid on %q", tc.name, tc.err, FieldOf(tc.err), tc.field)
		}
	}
	if err := (SaveTemplateInput{Name: "n", TaskRef: "a-1", DueIn: Duration(time.Hour)}).Validate(); err != nil {
		t.Errorf("a valid save: %v", err)
	}
	if n := (TemplateTask{Subtasks: []TemplateTask{{}, {Subtasks: []TemplateTask{{}}}}}).Count(); n != 4 {
		t.Errorf("Count = %d, want 4", n)
	}
}
