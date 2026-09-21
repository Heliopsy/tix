package tui

import (
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

func TestParseFilter(t *testing.T) {
	due := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		expr  string
		check func(t *testing.T, f core.TaskFilter)
	}{
		{"empty", "", func(t *testing.T, f core.TaskFilter) {
			if f.Query != "" || f.Page.Sort != core.SortCreatedAt {
				t.Fatalf("unexpected filter %+v", f)
			}
		}},
		{"project and status", "project:infra status:todo", func(t *testing.T, f core.TaskFilter) {
			if len(f.ProjectKeys) != 1 || f.ProjectKeys[0] != "infra" {
				t.Fatalf("projects %v", f.ProjectKeys)
			}
			if len(f.Statuses) != 1 || f.Statuses[0] != "todo" {
				t.Fatalf("statuses %v", f.Statuses)
			}
		}},
		{"free text", "deploy the thing", func(t *testing.T, f core.TaskFilter) {
			if f.Query != "deploy the thing" {
				t.Fatalf("query %q", f.Query)
			}
		}},
		{"quoted text is never a term", `"status:todo"`, func(t *testing.T, f core.TaskFilter) {
			if f.Query != "status:todo" || len(f.Statuses) != 0 {
				t.Fatalf("filter %+v", f)
			}
		}},
		{"is claimed", "is:claimed", func(t *testing.T, f core.TaskFilter) {
			if f.Claimed != core.Yes {
				t.Fatalf("claimed %v", f.Claimed)
			}
		}},
		{"is unclaimed", "is:unclaimed", func(t *testing.T, f core.TaskFilter) {
			if f.Claimed != core.No {
				t.Fatalf("claimed %v", f.Claimed)
			}
		}},
		{"is blocked", "is:blocked", func(t *testing.T, f core.TaskFilter) {
			if f.Blocked != core.Yes {
				t.Fatalf("blocked %v", f.Blocked)
			}
		}},
		{"priority name", "priority:high", func(t *testing.T, f core.TaskFilter) {
			if len(f.Priorities) != 1 || f.Priorities[0] != core.PriorityHigh {
				t.Fatalf("priorities %v", f.Priorities)
			}
		}},
		{"priority number", "priority:1", func(t *testing.T, f core.TaskFilter) {
			if f.Priorities[0] != core.PriorityHighest {
				t.Fatalf("priorities %v", f.Priorities)
			}
		}},
		{"due bound", "due-before:2026-05-01", func(t *testing.T, f core.TaskFilter) {
			if f.DueBefore == nil || !f.DueBefore.Equal(due) {
				t.Fatalf("due %v", f.DueBefore)
			}
		}},
		{"parent none", "parent:none", func(t *testing.T, f core.TaskFilter) {
			if !f.ParentIsNull {
				t.Fatal("expected roots only")
			}
		}},
		{"sort and limit", "sort:priority limit:10", func(t *testing.T, f core.TaskFilter) {
			if f.Page.Sort != core.SortPriority || f.Page.Limit != 10 {
				t.Fatalf("page %+v", f.Page)
			}
		}},
		{"terms and text mix", `tag:ops "urgent fix"`, func(t *testing.T, f core.TaskFilter) {
			if len(f.Tags) != 1 || f.Query != "urgent fix" {
				t.Fatalf("filter %+v", f)
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, err := ParseFilter(tc.expr)
			if err != nil {
				t.Fatalf("ParseFilter(%q): %v", tc.expr, err)
			}
			tc.check(t, f)
		})
	}
}

func TestParseFilterRejects(t *testing.T) {
	tests := []struct {
		name string
		expr string
		kind core.Kind
	}{
		{"unknown key", "colour:red", core.KindInvalid},
		{"empty value", "status:", core.KindInvalid},
		{"bad priority", "priority:urgent", core.KindInvalid},
		{"priority out of range", "priority:9", core.KindInvalid},
		{"bad time", "due-before:soon", core.KindInvalid},
		{"bad limit", "limit:-3", core.KindInvalid},
		{"unknown is", "is:purple", core.KindInvalid},
		{"unterminated quote", `text:"open`, core.KindInvalid},
		{"unsortable", "sort:colour", core.KindInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseFilter(tc.expr); err == nil {
				t.Fatalf("ParseFilter(%q) accepted a malformed expression", tc.expr)
			} else if core.KindOf(err) != tc.kind {
				t.Fatalf("kind %v, want %v", core.KindOf(err), tc.kind)
			}
		})
	}
}

func TestMatchesFilter(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	future := now.Add(time.Hour)
	task := core.Task{
		ID: "t1", Ref: "infra-1", Title: "Rotate keys", Status: "todo",
		Priority: core.PriorityHigh, Tags: []string{"ops"},
		AssigneeActorID: "a1", CreatorActorID: "c1",
	}
	claimed := task
	claimed.ClaimedByActorID, claimed.LeaseExpiresAt = "a2", &future

	tests := []struct {
		name string
		expr string
		task core.Task
		want bool
	}{
		{"no filter matches", "", task, true},
		{"status hit", "status:todo", task, true},
		{"status miss", "status:done", task, false},
		{"tag hit", "tag:ops", task, true},
		{"tag miss", "tag:sec", task, false},
		{"project hit", "project:infra", task, true},
		{"project miss", "project:web", task, false},
		{"priority hit", "priority:high", task, true},
		{"priority miss", "priority:low", task, false},
		{"text hit", "rotate", task, true},
		{"text miss", "nothing", task, false},
		{"assignee hit", "assignee:a1", task, true},
		{"unclaimed matches free task", "is:unclaimed", task, true},
		{"unclaimed rejects claimed task", "is:unclaimed", claimed, false},
		{"claimed matches claimed task", "is:claimed", claimed, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, err := ParseFilter(tc.expr)
			if err != nil {
				t.Fatalf("ParseFilter: %v", err)
			}
			if got := MatchesFilter(f, tc.task, "infra", now); got != tc.want {
				t.Fatalf("MatchesFilter = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMatchesFilterHidesDeletedUnlessAsked(t *testing.T) {
	deleted := time.Now()
	task := core.Task{ID: "t1", Status: "todo", DeletedAt: &deleted}
	plain, _ := ParseFilter("")
	if MatchesFilter(plain, task, "infra", time.Now()) {
		t.Fatal("deleted task matched a plain filter")
	}
	withDeleted, _ := ParseFilter("is:deleted")
	if !MatchesFilter(withDeleted, task, "infra", time.Now()) {
		t.Fatal("deleted task was hidden from is:deleted")
	}
}
