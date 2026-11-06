// SPDX-License-Identifier: AGPL-3.0-or-later

package taskio

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestSuggestStatus(t *testing.T) {
	states := []StateInfo{
		{Key: "backlog", Label: "Backlog", Category: core.CategoryTodo},
		{Key: "todo", Label: "To do", Category: core.CategoryTodo, Initial: true},
		{Key: "doing", Label: "Doing", Category: core.CategoryInProgress},
		{Key: "blocked", Label: "Blocked", Category: core.CategoryBlocked},
		{Key: "waiting", Label: "Waiting on others", Category: core.CategoryWaiting},
		{Key: "done", Label: "Готово", Category: core.CategoryDone},
		{Key: "cancelled", Label: "Cancelled", Category: core.CategoryCancelled},
	}
	tests := []struct{ in, want string }{
		{"Backlog", "backlog"}, {"TO-DO", "todo"}, {"готово", "done"}, {"Open", "todo"},
		{"Selected for Development", "todo"}, {"Нова", "todo"}, {"In Progress", "doing"}, {"In review", "doing"},
		{"В процес", "doing"}, {"On hold", "blocked"}, {"waiting", "waiting"}, {"Resolved", "done"},
		{"Closed", "done"}, {"Завършена", "done"}, {"Won't Do", "cancelled"}, {"Duplicate", "cancelled"},
		{"Canceled", "cancelled"}, {"Sprint 4", ""}, {"", ""},
	}
	for _, tt := range tests {
		if got := SuggestStatus(tt.in, states); got != tt.want {
			t.Errorf("SuggestStatus(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
	noInitial := []StateInfo{{Key: "new", Category: core.CategoryTodo}, {Key: "fin", Category: core.CategoryDone}}
	if got := SuggestStatus("open", noInitial); got != "new" {
		t.Errorf("without an initial state, open = %q", got)
	}
	if got := SuggestStatus("blocked", noInitial); got != "" {
		t.Errorf("with no blocked state, blocked = %q", got)
	}
}

func TestDistinct(t *testing.T) {
	var d Distinct
	if got := d.Values(TargetStatus); len(got) != 0 || d.Truncated(TargetStatus) {
		t.Errorf("zero Distinct = %v", got)
	}
	for _, v := range []string{"b", "a", "", "b", "c", "a", "b"} {
		d.Add(TargetStatus, v)
	}
	d.AddDraft(Draft{StatusRaw: "c", PriorityRaw: "High", AssigneeRaw: "ann", Tags: []string{"x", "y"}})
	want := []ValueCount{{"b", 3}, {"a", 2}, {"c", 2}}
	if got := d.Values(TargetStatus); !reflect.DeepEqual(got, want) {
		t.Errorf("Values = %v, want %v", got, want)
	}
	if got := d.Values(TargetTags); !reflect.DeepEqual(got, []ValueCount{{"x", 1}, {"y", 1}}) {
		t.Errorf("tags = %v", got)
	}
	for i := 0; i < DistinctCap+5; i++ {
		d.Add(TargetAssignee, "user"+strconv.Itoa(i))
	}
	d.Add(TargetAssignee, "ann")
	got := d.Values(TargetAssignee)
	if len(got) != DistinctCap || !d.Truncated(TargetAssignee) || got[0] != (ValueCount{"ann", 2}) {
		t.Errorf("capped values: %d, truncated %v, first %v", len(got), d.Truncated(TargetAssignee), got[0])
	}
}

func TestSuggestCustomAndBulgarian(t *testing.T) {
	fields := []FieldInfo{
		{Key: "estimate", Label: "Оценка", Type: core.FieldFloat},
		{Key: "sprint", Label: "Sprint", Type: core.FieldString},
	}
	headers := []string{"Име", "Описание", "Състояние", "Приоритет", "Отговорник", "Тагове", "Краен срок", "Зависи от",
		"Повторение", "Оценка", "Sprint", "Story Points", "Start date", "Parent", "Ref", "Profile URL", "Owner id", "Misc"}
	want := map[string]Target{
		"Име": TargetTitle, "Описание": TargetBody, "Състояние": TargetStatus, "Приоритет": TargetPriority,
		"Отговорник": TargetAssignee, "Тагове": TargetTags, "Краен срок": TargetDue, "Зависи от": TargetDependsOn,
		"Повторение": TargetRecurrence, "Оценка": CustomTarget("estimate"), "Sprint": CustomTarget("sprint"),
		"Story Points": TargetAppendBody, "Start date": TargetAppendBody, "Parent": TargetParent, "Ref": TargetRef,
		"Profile URL": TargetIgnore, "Owner id": TargetIgnore, "Misc": TargetAppendBody,
	}
	if got := Suggest(headers, "", fields); !reflect.DeepEqual(got, want) {
		t.Errorf("Suggest =\n%v\nwant\n%v", got, want)
	}
	tix := []string{"ref", "project", "title", "status", "priority", "assignee", "tags", "due", "created", "updated", "repeat"}
	if src := DetectSource(tix); src != SourceTix {
		t.Fatalf("tix export detected as %q", src)
	}
	got := Suggest(tix, SourceTix, nil)
	for h, w := range map[string]Target{"ref": TargetRef, "project": TargetIgnore, "repeat": TargetRecurrence, "created": TargetIgnore} {
		if got[h] != w {
			t.Errorf("tix %q = %q, want %q", h, got[h], w)
		}
	}
}
