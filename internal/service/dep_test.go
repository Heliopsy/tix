package service

import (
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestDependenciesAddListAndRemove(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	a := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "a"})
	b := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "b"})
	refB, refA := core.TaskRef{ID: b.ID}, core.TaskRef{ID: a.ID}

	beforeEvents, beforeAudits := countRows(t, l, scope)
	if err := l.AddDependency(ctx, refB, refA); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != beforeEvents+1 || afterAudits != beforeAudits+1 {
		t.Errorf("adding a dependency wrote %d events and %d audit entries, want one of each",
			afterEvents-beforeEvents, afterAudits-beforeAudits)
	}

	deps, err := l.ListDependencies(ctx, refB)
	if err != nil {
		t.Fatalf("ListDependencies: %v", err)
	}
	if len(deps) != 1 || deps[0].DependsOn != a.ID {
		t.Fatalf("dependencies = %+v, want one edge to %q", deps, a.ID)
	}

	blocked, err := l.GetTask(ctx, refB)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if !blocked.Blocked {
		t.Error("a task with an open dependency must report blocked")
	}

	if err := l.RemoveDependency(ctx, refB, refA); err != nil {
		t.Fatalf("RemoveDependency: %v", err)
	}
	deps, err = l.ListDependencies(ctx, refB)
	if err != nil {
		t.Fatalf("ListDependencies: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("dependencies = %+v, want none", deps)
	}
	unblocked, err := l.GetTask(ctx, refB)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if unblocked.Blocked {
		t.Error("removing the last dependency must clear the blocked state")
	}
}

func TestDependencyCyclesAreRejected(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	a := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "a"})
	b := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "b"})
	c := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "c"})

	if err := l.AddDependency(ctx, core.TaskRef{ID: a.ID}, core.TaskRef{ID: b.ID}); err != nil {
		t.Fatalf("a depends on b: %v", err)
	}
	if err := l.AddDependency(ctx, core.TaskRef{ID: b.ID}, core.TaskRef{ID: c.ID}); err != nil {
		t.Fatalf("b depends on c: %v", err)
	}

	if err := l.AddDependency(ctx, core.TaskRef{ID: a.ID}, core.TaskRef{ID: a.ID}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("self dependency = %v, want invalid", err)
	}
	if err := l.AddDependency(ctx, core.TaskRef{ID: c.ID}, core.TaskRef{ID: a.ID}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("transitive cycle = %v, want invalid", err)
	}

	deps, err := l.ListDependencies(ctx, core.TaskRef{ID: c.ID})
	if err != nil {
		t.Fatalf("ListDependencies: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("a rejected cycle created %d edges", len(deps))
	}
}

func TestDependencyOnAnUnknownTaskIsRejected(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	a := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "a"})

	if err := l.AddDependency(ctx, core.TaskRef{ID: a.ID}, core.MustParseTaskRef("infra-99")); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("dependency on a missing task = %v, want not found", err)
	}
	if err := l.RemoveDependency(ctx, core.TaskRef{ID: a.ID}, core.MustParseTaskRef("infra-99")); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("removing a missing dependency = %v, want not found", err)
	}
}

// A dependency stops blocking once it reaches a terminal state, with no further
// action on the waiting task.
func TestBlockedClearsWhenDependenciesReachTerminalStates(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	blocker := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "blocker"})
	waiter := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "waiter"})
	if err := l.AddDependency(ctx, core.TaskRef{ID: waiter.ID}, core.TaskRef{ID: blocker.ID}); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}

	page, err := l.ListTasks(ctx, core.TaskFilter{Blocked: core.Yes})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(page.Tasks) != 1 || page.Tasks[0].ID != waiter.ID {
		t.Fatalf("blocked listing = %v, want only the waiting task", taskTitles(page.Tasks))
	}

	if _, err := l.TransitionTask(ctx, core.TaskRef{ID: blocker.ID}, core.TransitionInput{To: "doing"}); err != nil {
		t.Fatalf("todo to doing: %v", err)
	}
	if _, err := l.TransitionTask(ctx, core.TaskRef{ID: blocker.ID}, core.TransitionInput{To: "done"}); err != nil {
		t.Fatalf("doing to done: %v", err)
	}

	got, err := l.GetTask(ctx, core.TaskRef{ID: waiter.ID})
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Blocked {
		t.Error("a task whose dependencies are all terminal must report unblocked")
	}
	if len(got.DependsOn) != 1 {
		t.Errorf("dependencies = %v, want the edge to remain", got.DependsOn)
	}
}

func TestDependencyOperationsRequireAnActor(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	a := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "a"})
	ref := core.TaskRef{ID: a.ID}

	if err := l.AddDependency(t.Context(), ref, ref); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("AddDependency unauthenticated = %v", err)
	}
	if err := l.RemoveDependency(t.Context(), ref, ref); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("RemoveDependency unauthenticated = %v", err)
	}
	if _, err := l.ListDependencies(t.Context(), ref); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("ListDependencies unauthenticated = %v", err)
	}
}
