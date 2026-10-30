// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func listRefs(t *testing.T, l *Local, ctx context.Context, f core.TaskFilter) []string {
	t.Helper()
	page, err := l.ListTasks(ctx, f)
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	out := make([]string, len(page.Tasks))
	for i, task := range page.Tasks {
		out[i] = task.Ref
	}
	return out
}

func mustStar(t *testing.T, l *Local, ctx context.Context, ref string) {
	t.Helper()
	if err := l.StarTask(ctx, core.TaskRef{ProjectKey: "infra", Seq: seqOf(t, ref)}); err != nil {
		t.Fatalf("starring %s: %v", ref, err)
	}
}

func seqOf(t *testing.T, ref string) int64 {
	t.Helper()
	parsed, err := core.ParseTaskRef(ref)
	if err != nil {
		t.Fatalf("parsing %q: %v", ref, err)
	}
	return parsed.Seq
}

func TestStarredTasksLeadTheReadersListing(t *testing.T) {
	l, ctx, scope, me, _ := newTaskFixture(t)
	other := taskContext(seedTaskActor(t, l, scope, "bob", core.RoleMember, core.ScopeAll))

	high := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "high", Priority: core.PriorityHigh})
	normal := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "normal"})
	low := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "low", Priority: core.PriorityLow})
	mustStar(t, l, ctx, low.Ref)

	cases := []struct {
		name string
		ctx  context.Context
		f    core.TaskFilter
		want []string
	}{
		{"the starrer sees it first", ctx, core.TaskFilter{}, []string{low.Ref, high.Ref, normal.Ref}},
		{"another reader does not", other, core.TaskFilter{}, []string{high.Ref, normal.Ref, low.Ref}},
		{"an explicit sort keeps the group", ctx, core.TaskFilter{Page: core.Page{Sort: core.SortSeq, Direction: core.Descending}},
			[]string{low.Ref, normal.Ref, high.Ref}},
		{"is:starred", ctx, core.TaskFilter{Starred: core.Yes}, []string{low.Ref}},
		{"-is:starred", ctx, core.TaskFilter{Starred: core.No}, []string{high.Ref, normal.Ref}},
		{"is:starred for another reader", other, core.TaskFilter{Starred: core.Yes}, []string{}},
		{"a caller cannot name the starrer", other, core.TaskFilter{Starred: core.Yes, StarredBy: me.ID}, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := listRefs(t, l, tc.ctx, tc.f); !slices.Equal(got, tc.want) {
				t.Fatalf("order = %v, want %v", got, tc.want)
			}
		})
	}

	got, err := l.GetTask(ctx, core.TaskRef{ID: low.ID})
	if err != nil || !got.Starred {
		t.Fatalf("GetTask for the starrer: starred = %v, err = %v", got != nil && got.Starred, err)
	}
	got, err = l.GetTask(other, core.TaskRef{ID: low.ID})
	if err != nil || got.Starred {
		t.Fatalf("GetTask for another reader reported the star: err = %v", err)
	}
}

func TestStarIsIdempotentAndRecordedOnce(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "watch me"})
	ref := core.TaskRef{ID: task.ID}

	steps := []struct {
		name  string
		call  func(context.Context, core.TaskRef) error
		write bool
		want  core.EventType
	}{
		{"star", l.StarTask, true, core.EventTaskStarred},
		{"star again", l.StarTask, false, ""},
		{"unstar", l.UnstarTask, true, core.EventTaskUnstarred},
		{"unstar again", l.UnstarTask, false, ""},
	}
	for _, step := range steps {
		events, audits := countRows(t, l, scope)
		if err := step.call(ctx, ref); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		afterEvents, afterAudits := countRows(t, l, scope)
		wrote := afterEvents == events+1 && afterAudits == audits+1
		silent := afterEvents == events && afterAudits == audits
		if step.write && !wrote || !step.write && !silent {
			t.Fatalf("%s: events %d -> %d, audits %d -> %d, want a write = %v",
				step.name, events, afterEvents, audits, afterAudits, step.write)
		}
		if !step.write {
			continue
		}
		if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
			evs, err := tx.ReadEvents(context.Background(), 0, 1000)
			if err != nil {
				return err
			}
			last := evs[len(evs)-1]
			if last.Type != step.want || last.Payload["ref"] != task.Ref || last.ActorID == "" {
				t.Errorf("%s: event = %s %v by %q", step.name, last.Type, last.Payload, last.ActorID)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestStarNeedsOnlyRead(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "x"})
	viewer := taskContext(seedTaskActor(t, l, scope, "vee", core.RoleViewer, core.ScopeTaskRead))
	none := taskContext(&core.Actor{ID: "nobody", TenantID: scope.TenantID, Kind: core.ActorAgent,
		Scopes: []core.Scope{core.ScopeProjectRead}})
	other := seedTaskProject(t, l, scope, "ops")
	pinned := &core.Actor{ID: "pinned", TenantID: scope.TenantID, Kind: core.ActorAgent,
		ProjectID: other.ID, Scopes: []core.Scope{core.ScopeAll}}

	if err := l.StarTask(viewer, core.TaskRef{ID: task.ID}); err != nil {
		t.Fatalf("a reader could not star: %v", err)
	}
	if err := l.StarTask(none, core.TaskRef{ID: task.ID}); !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("an actor without task:read starred: %v", err)
	}
	if err := l.StarTask(taskContext(pinned), core.TaskRef{ID: task.ID}); !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("a token pinned elsewhere starred: %v", err)
	}
}

func TestDeletingATaskRemovesItsStars(t *testing.T) {
	l, ctx, scope, actor, _ := newTaskFixture(t)
	soft := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "soft"})
	hard := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "hard"})
	plain := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "plain", Priority: core.PriorityHighest})
	mustStar(t, l, ctx, soft.Ref)
	mustStar(t, l, ctx, hard.Ref)

	if err := l.DeleteTask(ctx, core.TaskRef{ID: hard.ID}, core.DeleteTaskInput{Hard: true}); err != nil {
		t.Fatalf("hard delete: %v", err)
	}
	if err := l.DeleteTask(ctx, core.TaskRef{ID: soft.ID}, core.DeleteTaskInput{}); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if _, err := l.RestoreTask(ctx, core.TaskRef{ID: soft.ID}); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		got, err := tx.StarredTasks(context.Background(), actor.ID, []string{soft.ID, hard.ID})
		if err != nil {
			return err
		}
		if got[hard.ID] || !got[soft.ID] {
			t.Errorf("stars after deletes = %v, want only the restored task", got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got := listRefs(t, l, ctx, core.TaskFilter{}); !slices.Equal(got, []string{soft.Ref, plain.Ref}) {
		t.Fatalf("after restore = %v, want the restored star first", got)
	}
}

// walk pages through a listing two at a time, returning every ref it saw.
func walk(t *testing.T, l *Local, ctx context.Context, page core.Page, between func(n int)) []string {
	t.Helper()
	var seen []string
	page.Limit = 2
	for n := 0; n < 50; n++ {
		got, err := l.ListTasks(ctx, core.TaskFilter{Page: page})
		if err != nil {
			t.Fatalf("page %d: %v", n, err)
		}
		for _, task := range got.Tasks {
			seen = append(seen, task.Ref)
		}
		if got.NextCursor == "" {
			return seen
		}
		page.Cursor = got.NextCursor
		if between != nil {
			between(n)
		}
	}
	t.Fatal("the walk did not end")
	return nil
}

func TestStarredGroupPagesWithoutGapsOrRepeats(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	clk := l.clock.(interface{ Advance(time.Duration) })
	var starred, rest []string
	for i := 0; i < 12; i++ {
		due := time.Date(2026, 1, 1+i%5, 0, 0, 0, 0, time.UTC)
		in := core.CreateTaskInput{ProjectRef: "infra", Title: fmt.Sprintf("t%02d", i),
			Priority: core.Priority(1 + i%3)}
		if i%4 != 0 {
			in.DueAt = &due
		}
		task := mustCreateTask(t, l, ctx, in)
		clk.Advance(time.Second)
		if i%3 == 1 || i == 11 {
			mustStar(t, l, ctx, task.Ref)
			starred = append(starred, task.Ref)
			continue
		}
		rest = append(rest, task.Ref)
	}

	for _, sort := range core.TaskSortFields {
		for _, dir := range []core.SortDirection{core.Ascending, core.Descending} {
			t.Run(sort+"/"+string(dir), func(t *testing.T) {
				p := core.Page{Sort: sort, Direction: dir}
				whole := listRefs(t, l, ctx, core.TaskFilter{Page: core.Page{Sort: sort, Direction: dir, Limit: 100}})
				seen := walk(t, l, ctx, p, nil)
				if !slices.Equal(seen, whole) {
					t.Fatalf("paged %v\nwhole %v", seen, whole)
				}
				head := slices.Clone(seen[:len(starred)])
				slices.Sort(head)
				want := slices.Clone(starred)
				slices.Sort(want)
				if !slices.Equal(head, want) {
					t.Fatalf("first %d rows %v, want the starred %v", len(starred), seen[:len(starred)], starred)
				}
			})
		}
	}

	t.Run("starring mid-walk", func(t *testing.T) {
		late := rest[len(rest)-1]
		seen := walk(t, l, ctx, core.Page{Sort: core.SortSeq}, func(n int) {
			if n == 3 {
				mustStar(t, l, ctx, late)
			}
		})
		counts := map[string]int{}
		for _, ref := range seen {
			counts[ref]++
		}
		for _, ref := range append(slices.Clone(starred), rest...) {
			if ref == late {
				continue
			}
			if counts[ref] != 1 {
				t.Errorf("unchanged task %s seen %d times", ref, counts[ref])
			}
		}
	})
}

func TestAnOldCursorResumesAmongTheUnstarred(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	var all []*core.Task
	for i := 0; i < 4; i++ {
		all = append(all, mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: fmt.Sprint(i)}))
	}
	mustStar(t, l, ctx, all[3].Ref)
	old := core.Cursor{SortValue: "1", ID: all[0].ID, Sort: core.SortSeq, Direction: core.Ascending}.Encode()
	got := listRefs(t, l, ctx, core.TaskFilter{Page: core.Page{Sort: core.SortSeq, Cursor: old}})
	if !slices.Equal(got, []string{all[1].Ref, all[2].Ref}) {
		t.Fatalf("resumed at %v, want the unstarred tasks after the first", got)
	}
}
