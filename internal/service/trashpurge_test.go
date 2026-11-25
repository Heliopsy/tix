// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// isNotTrashed reports whether err is the refusal a purge of a live task
// answers with, by its detail rather than its wording.
func isNotTrashed(err error) bool {
	var domain *core.Error
	return core.IsKind(err, core.KindPrecondition) && errors.As(err, &domain) &&
		domain.Details["reason"] == core.ErrNotTrashed
}

// trashedPair creates a task with a subtask and deletes both into the trash.
func trashedPair(t *testing.T, l *Local, ctx context.Context) (parent, child *core.Task) {
	t.Helper()
	parent = mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "parent"})
	child = mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "child", ParentRef: parent.Ref})
	if err := l.DeleteTask(ctx, core.TaskRef{ID: parent.ID}, core.DeleteTaskInput{Cascade: true}); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	return parent, child
}

// stillThere reports whether a task's row exists, trashed or not.
func stillThere(t *testing.T, l *Local, scope core.TenantScope, id string) bool {
	t.Helper()
	found := false
	err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		_, err := tx.GetTask(context.Background(), core.TaskRef{ID: id})
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		found = err == nil
		return err
	})
	if err != nil {
		t.Fatalf("reading task %s: %v", id, err)
	}
	return found
}

func TestPurgeDeletesATrashedTaskAndItsTrashedSubtasks(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	parent, child := trashedPair(t, l, ctx)
	beforeEvents, beforeAudits := countRows(t, l, scope)

	if err := l.PurgeTask(ctx, core.TaskRef{ID: parent.ID}); err != nil {
		t.Fatalf("PurgeTask: %v", err)
	}
	for _, task := range []*core.Task{parent, child} {
		if stillThere(t, l, scope, task.ID) {
			t.Errorf("%s survived the purge", task.Title)
		}
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents-beforeEvents != 2 || afterAudits-beforeAudits != 2 {
		t.Fatalf("purging two tasks wrote %d events and %d audit entries, want two of each",
			afterEvents-beforeEvents, afterAudits-beforeAudits)
	}
	err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		evs, err := tx.ReadEvents(context.Background(), 0, 1000)
		if err != nil {
			return err
		}
		purged := map[string]bool{}
		for _, e := range evs[len(evs)-2:] {
			if e.Type == core.EventTaskDeleted && e.Payload["hard"] == true && e.Payload["purged"] == true {
				purged[e.SubjectID] = true
			}
		}
		if !purged[parent.ID] || !purged[child.ID] || len(purged) != 2 {
			t.Errorf("purge events = %+v, want a hard, purged task.deleted for each task", evs[len(evs)-2:])
		}
		return nil
	})
	if err != nil {
		t.Fatalf("reading events: %v", err)
	}
}

func TestPurgeRefusesALiveTask(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "live"})
	if err := l.PurgeTask(ctx, core.TaskRef{ID: task.ID}); !isNotTrashed(err) {
		t.Fatalf("purging a live task = %v, want the not_trashed refusal", err)
	}
	if !stillThere(t, l, scope, task.ID) {
		t.Fatal("a refused purge deleted the live task")
	}
}

func TestPurgeIsRefusedWhileTheTrashIsOff(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	parent, _ := trashedPair(t, l, ctx)
	setTrash(t, l, ctx, false)
	if err := l.PurgeTask(ctx, core.TaskRef{ID: parent.ID}); !isTrashRefusal(err) {
		t.Fatalf("purging with the trash off = %v, want the trash refusal", err)
	}
	if !stillThere(t, l, scope, parent.ID) {
		t.Fatal("a refused purge emptied the trash")
	}
}

func TestPurgeNeedsTheDeletePermission(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	parent, _ := trashedPair(t, l, ctx)
	viewer := taskContext(seedTaskActor(t, l, scope, "vee", core.RoleViewer, core.ScopeTaskRead))
	if err := l.PurgeTask(viewer, core.TaskRef{ID: parent.ID}); !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("a viewer's purge = %v, want forbidden", err)
	}
	if !stillThere(t, l, scope, parent.ID) {
		t.Fatal("a forbidden purge deleted the task")
	}
}

func TestPurgeOfAnUnknownTaskIsNotFound(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	if err := l.PurgeTask(ctx, core.TaskRef{ID: "01J0000000000000000000NONE"}); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("purging an unknown task = %v, want not_found", err)
	}
}
