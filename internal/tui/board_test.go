package tui

import (
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

func testWorkflow() *core.WorkflowDefinition {
	return &core.WorkflowDefinition{
		Initial: "todo",
		States: []core.State{
			{Key: "todo", Label: "To do"},
			{Key: "doing", Label: "Doing", Category: core.CategoryInProgress},
			{Key: "review", Label: "Review"},
			{Key: "done", Label: "Done", Terminal: true, Category: core.CategoryDone},
		},
		Transitions: []core.Transition{
			{From: "todo", To: "doing"},
			{From: "doing", To: "review"},
			{From: "doing", To: "todo"},
			{From: "review", To: "done"},
		},
	}
}

func task(id, status string, seq int64, priority core.Priority) core.Task {
	return core.Task{ID: id, Ref: "infra-" + id, Title: "task " + id, Status: status, Seq: seq, Priority: priority}
}

func TestBuildColumnsFollowsWorkflowOrder(t *testing.T) {
	cols := BuildColumns(testWorkflow(), nil)
	want := []string{"todo", "doing", "review", "done"}
	if len(cols) != len(want) {
		t.Fatalf("got %d columns, want %d", len(cols), len(want))
	}
	for i, key := range want {
		if cols[i].Key != key {
			t.Fatalf("column %d is %q, want %q", i, cols[i].Key, key)
		}
	}
	if cols[0].Label != "To do" || !cols[3].Terminal {
		t.Fatalf("labels or terminal flag lost: %+v", cols)
	}
}

func TestBuildColumnsPlacesTasksAndKeepsOrderStable(t *testing.T) {
	tasks := []core.Task{
		task("c", "todo", 3, core.PriorityNormal),
		task("a", "todo", 1, core.PriorityHighest),
		task("b", "doing", 2, core.PriorityNormal),
	}
	first := BuildColumns(testWorkflow(), tasks)
	if len(first[0].Tasks) != 2 || len(first[1].Tasks) != 1 {
		t.Fatalf("tasks landed wrong: %+v", first)
	}
	if first[0].Tasks[0].ID != "a" {
		t.Fatalf("highest priority is not first: %+v", first[0].Tasks)
	}
	second := BuildColumns(testWorkflow(), tasks)
	for i := range first {
		for j := range first[i].Tasks {
			if first[i].Tasks[j].ID != second[i].Tasks[j].ID {
				t.Fatal("redrawing the same data reordered the board")
			}
		}
	}
}

func TestBuildColumnsKeepsUnknownStatusVisible(t *testing.T) {
	cols := BuildColumns(testWorkflow(), []core.Task{task("z", "archived", 9, core.PriorityNormal)})
	last := cols[len(cols)-1]
	if last.Key != "archived" || len(last.Tasks) != 1 {
		t.Fatalf("task with an unknown status was hidden: %+v", cols)
	}
}

func TestSelectionHelpers(t *testing.T) {
	cols := BuildColumns(testWorkflow(), []core.Task{
		task("a", "todo", 1, core.PriorityNormal),
		task("b", "todo", 2, core.PriorityNormal),
		task("c", "doing", 3, core.PriorityNormal),
	})

	if sel, ok := FindTask(cols, "c"); !ok || sel.Col != 1 || sel.Row != 0 {
		t.Fatalf("FindTask = %+v, %v", sel, ok)
	}
	if _, ok := FindTask(cols, "missing"); ok {
		t.Fatal("FindTask found a task that is not on the board")
	}

	tests := []struct {
		name          string
		start         Selection
		dCol, dRow    int
		wantCol, want int
	}{
		{"down within column", Selection{0, 0}, 0, 1, 0, 1},
		{"down past the end clamps", Selection{0, 1}, 0, 1, 0, 1},
		{"up past the start clamps", Selection{0, 0}, 0, -1, 0, 0},
		{"right moves column", Selection{0, 1}, 1, 0, 1, 0},
		{"right past the end clamps", Selection{3, 0}, 1, 0, 3, 0},
		{"left past the start clamps", Selection{0, 0}, -1, 0, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MoveSelection(cols, tc.start, tc.dCol, tc.dRow)
			if got.Col != tc.wantCol || got.Row != tc.want {
				t.Fatalf("MoveSelection = %+v, want {%d %d}", got, tc.wantCol, tc.want)
			}
		})
	}
}

func TestPreserveSelectionKeepsTheTaskAndClampsWhenItIsGone(t *testing.T) {
	before := BuildColumns(testWorkflow(), []core.Task{
		task("a", "todo", 1, core.PriorityNormal),
		task("b", "todo", 2, core.PriorityNormal),
	})
	sel, _ := FindTask(before, "b")

	moved := BuildColumns(testWorkflow(), []core.Task{
		task("a", "todo", 1, core.PriorityNormal),
		task("b", "doing", 2, core.PriorityNormal),
	})
	if got := PreserveSelection(moved, sel, "b"); got.Col != 1 || got.Row != 0 {
		t.Fatalf("selection did not follow the task: %+v", got)
	}

	gone := BuildColumns(testWorkflow(), []core.Task{task("a", "todo", 1, core.PriorityNormal)})
	if got := PreserveSelection(gone, sel, "b"); got.Col != 0 || got.Row != 0 {
		t.Fatalf("selection was not clamped: %+v", got)
	}
}

func TestNextStates(t *testing.T) {
	got := NextStates(testWorkflow(), "doing")
	if len(got) != 2 || got[0].Key != "review" || got[1].Key != "todo" {
		t.Fatalf("NextStates = %+v", got)
	}
	if NextStates(testWorkflow(), "done") != nil {
		t.Fatal("a terminal state offered transitions")
	}
	if NextStates(nil, "todo") != nil {
		t.Fatal("a missing workflow offered transitions")
	}
}

func TestTaskBadgesDescribeStateInText(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	t.Run("free task", func(t *testing.T) {
		got := TaskBadges(task("a", "todo", 1, core.PriorityHighest), now)
		if len(got) != 1 || got[0] != "P1" {
			t.Fatalf("badges %v", got)
		}
	})
	t.Run("claimed and blocked task", func(t *testing.T) {
		tk := task("a", "todo", 1, core.PriorityNormal)
		tk.ClaimedByActorID, tk.LeaseExpiresAt, tk.Blocked = "worker-99", &future, true
		got := TaskBadges(tk, now)
		if len(got) != 3 || got[1] != "@worker-9" || got[2] != "!blocked" {
			t.Fatalf("badges %v", got)
		}
	})
}
