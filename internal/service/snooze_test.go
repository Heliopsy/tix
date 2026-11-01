// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func titlesOf(t *testing.T, l *Local, ctx context.Context, f core.TaskFilter) []string {
	t.Helper()
	f.Page.Sort = core.SortSeq
	page, err := l.ListTasks(ctx, f)
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	out := []string{}
	for _, task := range page.Tasks {
		out = append(out, task.Title)
	}
	return out
}

func snooze(t *testing.T, f *claimFixture, ctx context.Context, task *core.Task, d time.Duration) *core.Task {
	t.Helper()
	out, err := f.local.SnoozeTask(ctx, ref(task), core.SnoozeInput{Until: f.clock.Now().Add(d)})
	if err != nil {
		t.Fatalf("snoozing %s: %v", task.Title, err)
	}
	return out
}

func TestASnoozeHidesTheTaskFromEveryReaderUntilItWakes(t *testing.T) {
	f := newClaimFixture(t, 0)
	asleep := f.seedTask(t, "asleep", "todo", core.PriorityNormal)
	f.seedTask(t, "awake", "todo", core.PriorityNormal)

	got := snooze(t, f, f.ctx, asleep, time.Hour)
	if got.SnoozedUntil == nil || got.SnoozedByActorID != f.actor.ID {
		t.Fatalf("snoozed task = until %v by %q, want a wake time by %q", got.SnoozedUntil, got.SnoozedByActorID, f.actor.ID)
	}

	cases := []struct {
		name string
		ctx  context.Context
		f    core.TaskFilter
		want []string
	}{
		{"the snoozer's default listing", f.ctx, core.TaskFilter{}, []string{"awake"}},
		{"another reader's default listing", f.asOther(), core.TaskFilter{}, []string{"awake"}},
		{"is:snoozed", f.asOther(), core.TaskFilter{Snoozed: core.Yes}, []string{"asleep"}},
		{"is:awake", f.ctx, core.TaskFilter{Snoozed: core.No}, []string{"awake"}},
		{"snoozed:any", f.ctx, core.TaskFilter{IncludeSnoozed: true}, []string{"asleep", "awake"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := titlesOf(t, f.local, tc.ctx, tc.f); !slices.Equal(got, tc.want) {
				t.Fatalf("listing = %v, want %v", got, tc.want)
			}
		})
	}

	shown, err := f.local.GetTask(f.asOther(), ref(asleep))
	if err != nil || !shown.SnoozedAt(f.clock.Now()) {
		t.Fatalf("GetTask on a snoozed task: %v, snoozed = %v", err, shown != nil && shown.SnoozedAt(f.clock.Now()))
	}

	f.clock.Advance(time.Hour + time.Second)
	if got := titlesOf(t, f.local, f.ctx, core.TaskFilter{}); !slices.Equal(got, []string{"asleep", "awake"}) {
		t.Fatalf("after the wake time with no wake pass, listing = %v, want both", got)
	}
	if got := titlesOf(t, f.local, f.ctx, core.TaskFilter{Snoozed: core.Yes}); len(got) != 0 {
		t.Fatalf("is:snoozed after the wake time = %v, want none", got)
	}
}

func TestASnoozeIsRecordedWithWhoSetIt(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "later", "todo", core.PriorityNormal)
	snooze(t, f, f.asOther(), task, 24*time.Hour)

	events := f.events(t, core.EventTaskSnoozed)
	if len(events) != 1 || events[0].ActorID != f.other.ID || events[0].Payload["ref"] != task.Ref {
		t.Fatalf("task.snoozed events = %+v, want one by bob naming %s", events, task.Ref)
	}
	entries, _, err := f.local.ListAudit(f.ctx, core.AuditFilter{SubjectType: "task", SubjectID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	var entry *core.AuditEntry
	for i := range entries {
		if entries[i].Action == "task.snooze" {
			entry = &entries[i]
		}
	}
	if entry == nil || entry.ActorID != f.other.ID || !strings.Contains(string(entry.After), "snoozed_until") ||
		strings.Contains(string(entry.Before), "snoozed_until") {
		t.Fatalf("task.snooze audit = %+v, want one by bob with the wake time after and none before", entry)
	}
}

func TestASnoozeMustWakeInTheFuture(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "now", "todo", core.PriorityNormal)
	deleted := f.seedTask(t, "gone", "todo", core.PriorityNormal)
	if err := f.local.DeleteTask(f.ctx, ref(deleted), core.DeleteTaskInput{}); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		task  *core.Task
		until time.Time
		want  core.Kind
	}{
		{"no time", task, time.Time{}, core.KindInvalid},
		{"the past", task, f.clock.Now().Add(-time.Minute), core.KindInvalid},
		{"this very second", task, f.clock.Now(), core.KindInvalid},
		{"a deleted task", deleted, f.clock.Now().Add(time.Hour), core.KindNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.local.SnoozeTask(f.ctx, ref(tc.task), core.SnoozeInput{Until: tc.until})
			if !core.IsKind(err, tc.want) {
				t.Fatalf("err = %v, want %s", err, tc.want)
			}
		})
	}
	if n := len(f.events(t, core.EventTaskSnoozed)); n != 0 {
		t.Fatalf("%d task.snoozed events after refusals, want none", n)
	}
}

func TestSnoozingNeedsTaskWrite(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "x"})
	reader := taskContext(seedTaskActor(t, l, scope, "reader", core.RoleViewer, core.ScopeTaskRead))
	until := time.Now().Add(time.Hour)
	if _, err := l.SnoozeTask(reader, core.TaskRef{ID: task.ID}, core.SnoozeInput{Until: until}); !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("snooze with task:read = %v, want forbidden", err)
	}
	if _, err := l.UnsnoozeTask(reader, core.TaskRef{ID: task.ID}); !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("unsnooze with task:read = %v, want forbidden", err)
	}
}

func TestUnsnoozeBringsATaskBackAndWritesOnlyWhenItChanges(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "x", "todo", core.PriorityNormal)
	snooze(t, f, f.ctx, task, time.Hour)

	back, err := f.local.UnsnoozeTask(f.ctx, ref(task))
	if err != nil || back.SnoozedUntil != nil || back.SnoozedByActorID != "" {
		t.Fatalf("unsnoozed task = %+v, %v; want no wake time and no snoozer", back, err)
	}
	if got := titlesOf(t, f.local, f.ctx, core.TaskFilter{}); !slices.Equal(got, []string{"x"}) {
		t.Fatalf("listing after unsnooze = %v", got)
	}
	if _, err := f.local.UnsnoozeTask(f.ctx, ref(task)); err != nil {
		t.Fatal(err)
	}
	if n := len(f.events(t, core.EventTaskUnsnoozed)); n != 1 {
		t.Fatalf("%d task.unsnoozed events, want one: the second unsnooze changed nothing", n)
	}
}

func TestSnoozingKeepsTheVersion(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "x", "todo", core.PriorityNormal)
	got := snooze(t, f, f.ctx, task, time.Hour)
	if got.Version != task.Version {
		t.Fatalf("version %d -> %d; an edit form read before the snooze would be refused", task.Version, got.Version)
	}
	title := "renamed"
	if _, err := f.local.UpdateTask(f.ctx, ref(task), core.UpdateTaskInput{Title: &title, Version: task.Version}); err != nil {
		t.Fatalf("editing with the version read before the snooze: %v", err)
	}
	if !f.reload(t, task).SnoozedAt(f.clock.Now()) {
		t.Fatal("an edit cleared the snooze")
	}
}

func TestTheWakePassClearsPassedSnoozesAndAnnouncesThem(t *testing.T) {
	f := newClaimFixture(t, 0)
	soon := f.seedTask(t, "soon", "todo", core.PriorityNormal)
	late := f.seedTask(t, "late", "todo", core.PriorityNormal)
	snooze(t, f, f.ctx, soon, time.Hour)
	snooze(t, f, f.ctx, late, 5*time.Hour)

	woken, err := f.local.WakeSnoozed(f.ctx, 10)
	if err != nil || woken != 0 {
		t.Fatalf("wake before anything passed = %d, %v; want 0", woken, err)
	}
	f.clock.Advance(2 * time.Hour)
	woken, err = f.local.WakeSnoozed(f.ctx, 10)
	if err != nil || woken != 1 {
		t.Fatalf("wake = %d, %v; want 1", woken, err)
	}
	if got := f.reload(t, soon); got.SnoozedUntil != nil || got.SnoozedByActorID != "" {
		t.Fatalf("woken task still carries until %v by %q", got.SnoozedUntil, got.SnoozedByActorID)
	}
	if got := f.reload(t, late); got.SnoozedUntil == nil {
		t.Fatal("the wake pass cleared a snooze that has not passed")
	}
	events := f.events(t, core.EventTaskWoke)
	if len(events) != 1 || events[0].Payload["ref"] != soon.Ref || events[0].Payload["snoozed_by"] != f.actor.ID {
		t.Fatalf("task.woke events = %+v, want one for %s naming who snoozed it", events, soon.Ref)
	}
	if woken, _ := f.local.WakeSnoozed(f.ctx, 10); woken != 0 {
		t.Fatalf("a second pass woke %d, want 0", woken)
	}
}

func TestTheWakePassLeavesATaskSnoozedAgain(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "x", "todo", core.PriorityNormal)
	first := snooze(t, f, f.ctx, task, time.Hour)
	f.clock.Advance(2 * time.Hour)
	again := snooze(t, f, f.ctx, task, time.Hour)

	ctx := context.Background()
	if err := f.local.store.Update(ctx, f.scope, func(tx store.Tx) error {
		ok, err := tx.WakeSnooze(ctx, task.ID, *first.SnoozedUntil)
		if ok {
			t.Error("waking with the stale wake time cleared the new snooze")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if got := f.reload(t, task); got.SnoozedUntil == nil || !got.SnoozedUntil.Equal(*again.SnoozedUntil) {
		t.Fatalf("wake time = %v, want %v", got.SnoozedUntil, again.SnoozedUntil)
	}
}

func TestASnoozedTaskIsNotClaimable(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "only", "todo", core.PriorityNormal)
	snooze(t, f, f.ctx, task, time.Hour)

	if _, err := f.local.ClaimNext(f.asOther(), core.ClaimNextInput{}); !core.IsKind(err, core.KindNoTaskAvailable) {
		t.Fatalf("claim next with only a snoozed task = %v, want no_task_available", err)
	}
	_, err := f.local.ClaimTask(f.asOther(), ref(task), core.ClaimInput{})
	if !core.IsKind(err, core.KindPrecondition) || !strings.Contains(err.Error(), "snoozed until") {
		t.Fatalf("claim by reference = %v, want a precondition naming the wake time", err)
	}

	f.clock.Advance(time.Hour + time.Second)
	claim, err := f.local.ClaimNext(f.asOther(), core.ClaimNextInput{})
	if err != nil || claim.Task.ID != task.ID {
		t.Fatalf("claim next after the wake time = %v, %v; want the task", claim, err)
	}
}

func TestSnoozingAClaimedTaskKeepsTheLease(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "held", "todo", core.PriorityNormal)
	if _, err := f.local.ClaimTask(f.asOther(), ref(task), core.ClaimInput{}); err != nil {
		t.Fatal(err)
	}
	snooze(t, f, f.ctx, task, time.Hour)
	if got := f.reload(t, task); got.ClaimedByActorID != f.other.ID {
		t.Fatalf("holder after a snooze = %q, want bob", got.ClaimedByActorID)
	}
}

func TestAnExportKeepsASnoozedTask(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "parked", "todo", core.PriorityNormal)
	snooze(t, f, f.ctx, task, time.Hour)
	var buf bytes.Buffer
	if err := f.local.ExportTo(f.ctx, core.ExportInput{}, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"title":"parked"`) || !strings.Contains(buf.String(), `"snoozed_until"`) {
		t.Fatalf("export left out the snoozed task or its wake time:\n%s", buf.String())
	}
}

func TestASnoozeSurvivesARoundTripIntoAFreshTenant(t *testing.T) {
	l, clk, _, actor := newLocal(t)
	source := core.WithActor(context.Background(), actor)
	seeded := seed(t, l, source, "infra")
	until := clk.Now().Add(48 * time.Hour).Truncate(time.Second)
	if _, err := l.SnoozeTask(source, seeded.first, core.SnoozeInput{Until: until}); err != nil {
		t.Fatal(err)
	}
	target, _, _ := newTenant(t, l, "beta")
	if _, err := l.ImportFrom(target, bytes.NewReader(exportBytes(t, l, source, core.ExportInput{})),
		core.ImportInput{Mode: core.ImportMerge}); err != nil {
		t.Fatalf("import: %v", err)
	}
	page, err := l.ListTasks(target, core.TaskFilter{Snoozed: core.Yes})
	if err != nil || len(page.Tasks) != 1 || !page.Tasks[0].SnoozedUntil.Equal(until) {
		t.Fatalf("snoozed tasks in the target = %+v, %v; want one until %v", page.Tasks, err, until)
	}
}
