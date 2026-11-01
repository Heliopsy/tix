// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// setTrash turns the fixture tenant's trash on or off.
func setTrash(t *testing.T, l *Local, ctx context.Context, on bool) {
	t.Helper()
	off := !on
	if _, err := l.UpdateTenant(ctx, "", core.UpdateTenantInput{TrashOff: &off}); err != nil {
		t.Fatalf("turning the trash %s: %v", trashWord(on), err)
	}
}

// isTrashRefusal reports whether err is the refusal a tenant with no trash
// answers with, by its detail rather than its wording.
func isTrashRefusal(err error) bool {
	var domain *core.Error
	return core.IsKind(err, core.KindPrecondition) && errors.As(err, &domain) &&
		domain.Details["reason"] == core.ErrTrashOff
}

func TestTrashIsOnUntilSomebodyTurnsItOff(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	status, err := l.TrashStatus(ctx)
	if err != nil {
		t.Fatalf("TrashStatus: %v", err)
	}
	if !status.Enabled || status.Held != 0 {
		t.Errorf("a new tenant's trash = %+v, want on and empty", status)
	}
}

func TestWithTheTrashOffADeleteIsPermanent(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	setTrash(t, l, ctx, false)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "gone"})
	ref := core.TaskRef{ID: task.ID}

	if err := l.DeleteTask(ctx, ref, core.DeleteTaskInput{}); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	page, err := l.ListTasks(ctx, core.TaskFilter{IncludeDeleted: true})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	for _, got := range page.Tasks {
		if got.ID == task.ID {
			t.Fatalf("a delete with the trash off left the row behind: %+v", got)
		}
	}
	if _, err := l.RestoreTask(ctx, ref); err == nil {
		t.Error("restoring a task deleted with the trash off succeeded")
	}
}

func TestWithTheTrashOffRestoreAndTheTrashAreRefused(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "kept"})
	ref := core.TaskRef{ID: task.ID}
	if err := l.DeleteTask(ctx, ref, core.DeleteTaskInput{}); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	setTrash(t, l, ctx, false)

	if _, err := l.RestoreTask(ctx, ref); !isTrashRefusal(err) {
		t.Errorf("RestoreTask with the trash off = %v, want the trash refusal", err)
	}
	if _, err := l.ListTasks(ctx, core.TaskFilter{Trashed: core.Yes}); !isTrashRefusal(err) {
		t.Errorf("listing the trash with it off = %v, want the trash refusal", err)
	}
	if _, err := l.ListTasks(ctx, core.TaskFilter{}); err != nil {
		t.Errorf("listing live tasks with the trash off = %v, want it answered", err)
	}
	status, err := l.TrashStatus(ctx)
	if err != nil {
		t.Fatalf("TrashStatus: %v", err)
	}
	if status.Enabled || status.Held != 1 {
		t.Errorf("trash after turning it off = %+v, want off and still holding the one task", status)
	}
}

func TestTurningTheTrashBackOnReturnsWhatItHeld(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "returns"})
	ref := core.TaskRef{ID: task.ID}
	if err := l.DeleteTask(ctx, ref, core.DeleteTaskInput{}); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	setTrash(t, l, ctx, false)
	setTrash(t, l, ctx, true)

	page, err := l.ListTasks(ctx, core.TaskFilter{Trashed: core.Yes})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(page.Tasks) != 1 || page.Tasks[0].ID != task.ID {
		t.Fatalf("trash after turning it back on = %+v, want the task deleted before", page.Tasks)
	}
	if _, err := l.RestoreTask(ctx, ref); err != nil {
		t.Errorf("RestoreTask after turning the trash back on: %v", err)
	}
}

func TestChangingTheTrashIsAuditedOnce(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	beforeEvents, beforeAudits := countRows(t, l, scope)
	setTrash(t, l, ctx, false)
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != beforeEvents+1 || afterAudits != beforeAudits+1 {
		t.Fatalf("turning the trash off wrote %d events and %d audit entries, want one of each",
			afterEvents-beforeEvents, afterAudits-beforeAudits)
	}
	err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		entries, err := tx.ListAudit(context.Background(), core.AuditFilter{
			Actions: []string{auditTenantUpdate}, Page: core.Page{Limit: 10, Sort: defaultAuditSort}})
		if err != nil {
			return err
		}
		for _, e := range entries {
			if strings.Contains(string(e.Before), `"trash_off":false`) &&
				strings.Contains(string(e.After), `"trash_off":true`) {
				return nil
			}
		}
		t.Errorf("no tenant.update entry says the trash went off: %+v", entries)
		return nil
	})
	if err != nil {
		t.Fatalf("reading the audit log: %v", err)
	}
}
