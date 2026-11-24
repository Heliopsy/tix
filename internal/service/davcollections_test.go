// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

const davOrigin = "https://tix.example"

func (f davFixture) task(t *testing.T, title string, assignee string) *core.Task {
	t.Helper()
	return mustCreateTask(t, f.l, f.admin, core.CreateTaskInput{Title: title, ProjectRef: f.project.Key,
		AssigneeActorID: assignee})
}

func (f davFixture) move(t *testing.T, task *core.Task, to string) {
	t.Helper()
	if _, err := f.l.TransitionTask(f.admin, core.TaskRef{ID: task.ID}, core.TransitionInput{To: to}); err != nil {
		t.Fatalf("moving %s to %s: %v", task.Ref, to, err)
	}
}

func (f davFixture) snapshot(t *testing.T, ctx context.Context, collection, since string, limit int) *core.DAVSnapshot {
	t.Helper()
	s, err := f.l.DAVSnapshot(ctx, core.DAVSnapshotInput{Collection: collection, Since: since, Limit: limit, Origin: davOrigin})
	if err != nil {
		t.Fatalf("snapshot of %s since %q: %v", collection, since, err)
	}
	return s
}

func memberNames(ms []core.DAVMember) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.Name
	}
	return out
}

func names(tasks ...*core.Task) []string {
	out := make([]string, len(tasks))
	for i, t := range tasks {
		out[i] = strings.ToLower(t.ID)
	}
	slices.Sort(out)
	return out
}

func (f davFixture) projectCollection() string { return core.DAVProjectPrefix + strings.ToLower(f.project.ID) }

func TestACollectionHoldsOpenAndRecentlyFinishedTasks(t *testing.T) {
	f := newDAVFixture(t)
	p := f.create(t, f.member, core.CreateDAVPasswordInput{Name: "Mac", Collections: core.DAVCollectionsProjects})
	old := f.task(t, "finished sixty days ago", "")
	f.move(t, old, "done")
	f.clk.Advance(59 * 24 * time.Hour)
	yesterday := f.task(t, "finished yesterday", "")
	f.move(t, yesterday, "done")
	f.clk.Advance(24 * time.Hour)
	open := f.task(t, "open", "")
	blocked := f.task(t, "blocked", "")
	f.move(t, blocked, "blocked")
	cancelled := f.task(t, "cancelled", "")
	f.move(t, cancelled, "cancelled")
	trashed := f.task(t, "trashed", "")
	if err := f.l.DeleteTask(f.admin, core.TaskRef{ID: trashed.ID}, core.DeleteTaskInput{}); err != nil {
		t.Fatal(err)
	}
	s := f.snapshot(t, f.as(p), f.projectCollection(), "", 0)
	if got, want := memberNames(s.Members), names(open, blocked, yesterday); !slices.Equal(got, want) {
		t.Errorf("members %v, want %v", got, want)
	}
	objects, err := f.l.GetDAVObjects(f.as(p), core.DAVObjectsInput{Collection: f.projectCollection(),
		Names: []string{strings.ToLower(yesterday.ID), strings.ToLower(old.ID)}, Origin: davOrigin})
	if err != nil {
		t.Fatal(err)
	}
	if len(objects) != 1 || objects[0].Name != strings.ToLower(yesterday.ID) ||
		!strings.Contains(string(objects[0].Data), "STATUS:COMPLETED") ||
		!strings.Contains(string(objects[0].Data), "URL:"+davOrigin+"/tasks/infra-2") {
		t.Errorf("objects %+v", objects)
	}
	if objects[0].ETag != s.Members[slices.IndexFunc(s.Members, func(m core.DAVMember) bool {
		return m.Name == objects[0].Name
	})].ETag {
		t.Error("an object's entity tag differs from its member's")
	}
}

func TestACollectionPastItsBoundSaysHowManyWereLeftOut(t *testing.T) {
	f := newDAVFixture(t, WithCalDAV(true, 14*24*time.Hour, 10))
	p := f.create(t, f.member, core.CreateDAVPasswordInput{Name: "Mac", Collections: core.DAVCollectionsProjects})
	done := f.task(t, "done", "")
	f.move(t, done, "done")
	var open []*core.Task
	for i := range 11 {
		due := f.clk.Now().Add(time.Duration(i+1) * time.Hour)
		task := mustCreateTask(t, f.l, f.admin, core.CreateTaskInput{Title: "open", ProjectRef: f.project.Key, DueAt: &due})
		open = append(open, task)
	}
	s := f.snapshot(t, f.as(p), f.projectCollection(), "", 0)
	if len(s.Members) != 10 || s.LeftOut != 2 || !strings.Contains(s.Collection.Description, "2 more left out") {
		t.Fatalf("%d members, %d left out, description %q", len(s.Members), s.LeftOut, s.Collection.Description)
	}
	if got, want := memberNames(s.Members), names(open[:10]...); !slices.Equal(got, want) {
		t.Errorf("kept %v, want the ten soonest due open tasks %v", got, want)
	}
}

func TestAPasswordShowsTheCollectionsItChose(t *testing.T) {
	f := newDAVFixture(t)
	second := seedTaskProjectWith(t, f.l, f.scope, "web", davTestWorkflow())
	for _, tc := range []struct {
		choice core.DAVCollections
		want   []string
	}{
		{"", []string{core.DAVAssignedCollection}},
		{core.DAVCollectionsProjects, []string{"p-" + strings.ToLower(f.project.ID), "p-" + strings.ToLower(second.ID)}},
		{core.DAVCollectionsAll, []string{core.DAVAssignedCollection, "p-" + strings.ToLower(f.project.ID),
			"p-" + strings.ToLower(second.ID)}},
	} {
		p := f.create(t, f.member, core.CreateDAVPasswordInput{Name: "x" + string(tc.choice), Collections: tc.choice})
		got, err := f.l.ListDAVCollections(f.as(p))
		if err != nil {
			t.Fatal(err)
		}
		var gotNames []string
		for _, c := range got {
			gotNames = append(gotNames, c.Name)
		}
		if !slices.Equal(gotNames, tc.want) {
			t.Errorf("%q lists %v, want %v", tc.choice, gotNames, tc.want)
		}
		for _, name := range []string{core.DAVAssignedCollection, "p-" + strings.ToLower(f.project.ID)} {
			_, err := f.l.DAVSnapshot(f.as(p), core.DAVSnapshotInput{Collection: name})
			if slices.Contains(tc.want, name) != (err == nil) {
				t.Errorf("%q reading %s: %v", tc.choice, name, err)
			}
		}
	}
}

func TestAssignedToMeHoldsTheOwnersTasks(t *testing.T) {
	f := newDAVFixture(t)
	p := f.create(t, f.member, core.CreateDAVPasswordInput{Name: "iPhone"})
	mine := f.task(t, "mine", f.actor.ID)
	f.task(t, "nobody's", "")
	s := f.snapshot(t, f.as(p), core.DAVAssignedCollection, "", 0)
	if got := memberNames(s.Members); !slices.Equal(got, names(mine)) {
		t.Errorf("members %v", got)
	}
}

func TestADAVReaderFollowsItsOwnersStanding(t *testing.T) {
	f := newDAVFixture(t)
	p := f.create(t, f.member, core.CreateDAVPasswordInput{Name: "iPhone"})
	f.task(t, "mine", f.actor.ID)
	if _, err := f.l.UpdateUser(f.admin, f.actor.ID, core.UpdateUserInput{Role: ptrRole(core.RoleViewer)}); err != nil {
		t.Fatal(err)
	}
	if s := f.snapshot(t, f.as(p), core.DAVAssignedCollection, "", 0); len(s.Members) != 1 {
		t.Errorf("a viewer's phone lists %d tasks", len(s.Members))
	}
	if _, err := f.l.UpdateUser(f.admin, f.actor.ID, core.UpdateUserInput{Role: ptrRole(core.RoleAdmin)}); err != nil {
		t.Fatal(err)
	}
	s, err := f.l.davSessionOf(f.as(p))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(s.reader.Scopes, davReadScopes) || s.reader.HasScope(core.ScopeTaskDelete) ||
		s.reader.HasScope(core.ScopeTaskWrite) {
		t.Errorf("an admin's phone reads with %v", s.reader.Scopes)
	}

	for name, stop := range map[string]func(f davFixture){
		"removed": func(f davFixture) {
			if err := f.l.RemoveMember(f.admin, f.actor.ID); err != nil {
				t.Fatal(err)
			}
		},
		"disabled": func(f davFixture) {
			yes := true
			if _, err := f.l.UpdateUser(f.admin, f.actor.ID, core.UpdateUserInput{Disabled: &yes}); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newDAVFixture(t)
			p := f.create(t, f.member, core.CreateDAVPasswordInput{Name: "iPhone"})
			stop(f)
			if _, err := f.l.DAVPrincipal(f.as(p)); !core.IsKind(err, core.KindUnauthenticated) {
				t.Fatalf("err = %v, want unauthenticated", err)
			}
		})
	}
}

func TestOnlyAnAppPasswordReadsOverDAV(t *testing.T) {
	f := newDAVFixture(t)
	if _, err := f.l.DAVPrincipal(f.member); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a session read over DAV: %v", err)
	}
	if _, err := f.l.DAVPrincipal(f.admin); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("an admin read over DAV: %v", err)
	}
	off := newDAVFixture(t, WithCalDAV(false, 0, 0))
	p := off.create(t, off.member, core.CreateDAVPasswordInput{Name: "iPhone"})
	if _, err := off.l.DAVPrincipal(off.as(p)); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("disabled CalDAV answered: %v", err)
	}
}

func TestSyncTokensAnswerTheDifference(t *testing.T) {
	f := newDAVFixture(t)
	p := f.create(t, f.member, core.CreateDAVPasswordInput{Name: "iPhone"})
	ctx := f.as(p)
	a := f.task(t, "a", f.actor.ID)
	b := f.task(t, "b", f.actor.ID)
	first := f.snapshot(t, ctx, core.DAVAssignedCollection, "", 0)
	if !strings.HasPrefix(first.Token, DAVSyncTokenPrefix) || !slices.Equal(memberNames(first.Changed), names(a, b)) {
		t.Fatalf("first sync %+v", first)
	}

	again := f.snapshot(t, ctx, core.DAVAssignedCollection, first.Token, 0)
	if again.Token != first.Token || len(again.Changed) != 0 || len(again.Removed) != 0 {
		t.Errorf("nothing changed, yet %+v", again)
	}

	f.move(t, a, "doing")
	c := f.task(t, "c", f.actor.ID)
	f.move(t, b, "cancelled")
	next := f.snapshot(t, ctx, core.DAVAssignedCollection, first.Token, 0)
	if !slices.Equal(memberNames(next.Changed), names(a, c)) || !slices.Equal(next.Removed, names(b)) {
		t.Errorf("changed %v removed %v", memberNames(next.Changed), next.Removed)
	}
	if next.Token == first.Token {
		t.Error("the token did not move")
	}

	if _, err := f.l.DAVSnapshot(ctx, core.DAVSnapshotInput{Collection: core.DAVAssignedCollection,
		Since: DAVSyncTokenPrefix + "0000"}); !core.IsDAVSyncToken(err) {
		t.Errorf("an unknown token: %v", err)
	}
	if _, err := f.l.DAVSnapshot(ctx, core.DAVSnapshotInput{Collection: core.DAVAssignedCollection,
		Since: "http://elsewhere/x"}); !core.IsDAVSyncToken(err) {
		t.Errorf("a foreign token: %v", err)
	}
	other := f.create(t, f.member, core.CreateDAVPasswordInput{Name: "Mac"})
	if _, err := f.l.DAVSnapshot(f.as(other), core.DAVSnapshotInput{Collection: core.DAVAssignedCollection,
		Since: next.Token}); !core.IsDAVSyncToken(err) {
		t.Errorf("another password's token: %v", err)
	}
}

func TestAFinishedTaskAgesOutWithNoEvent(t *testing.T) {
	f := newDAVFixture(t)
	p := f.create(t, f.member, core.CreateDAVPasswordInput{Name: "iPhone"})
	task := f.task(t, "done soon", f.actor.ID)
	f.move(t, task, "done")
	f.clk.Advance(13 * 24 * time.Hour)
	held := f.snapshot(t, f.as(p), core.DAVAssignedCollection, "", 0)
	if !slices.Equal(memberNames(held.Members), names(task)) {
		t.Fatalf("members %v", memberNames(held.Members))
	}
	f.clk.Advance(2 * 24 * time.Hour)
	later := f.snapshot(t, f.as(p), core.DAVAssignedCollection, held.Token, 0)
	if !slices.Equal(later.Removed, names(task)) || len(later.Members) != 0 {
		t.Errorf("after the window: removed %v, members %v", later.Removed, memberNames(later.Members))
	}
}

func TestAWorkflowEditChangesWhatACollectionHolds(t *testing.T) {
	f := newDAVFixture(t)
	p := f.create(t, f.member, core.CreateDAVPasswordInput{Name: "iPhone"})
	task := f.task(t, "blocked", f.actor.ID)
	f.move(t, task, "blocked")
	held := f.snapshot(t, f.as(p), core.DAVAssignedCollection, "", 0)
	def := davTestWorkflow()
	def.States[2].Category = core.CategoryCancelled
	var wf *core.Workflow
	if err := f.l.store.View(context.Background(), f.scope, func(tx store.Tx) error {
		var err error
		wf, err = tx.GetWorkflowByID(context.Background(), f.project.WorkflowID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.l.PutWorkflow(f.admin, core.WorkflowInput{Key: wf.Key, Name: wf.Name, Definition: def}); err != nil {
		t.Fatal(err)
	}
	later := f.snapshot(t, f.as(p), core.DAVAssignedCollection, held.Token, 0)
	if !slices.Equal(later.Removed, names(task)) {
		t.Errorf("after the edit: removed %v", later.Removed)
	}
}

func TestALimitedSyncResumes(t *testing.T) {
	f := newDAVFixture(t)
	p := f.create(t, f.member, core.CreateDAVPasswordInput{Name: "iPhone"})
	ctx := f.as(p)
	gone := f.task(t, "gone", f.actor.ID)
	start := f.snapshot(t, ctx, core.DAVAssignedCollection, "", 0)
	f.move(t, gone, "cancelled")
	var made []*core.Task
	for range 4 {
		made = append(made, f.task(t, "new", f.actor.ID))
	}
	part := f.snapshot(t, ctx, core.DAVAssignedCollection, start.Token, 2)
	if !part.Truncated || len(part.Changed)+len(part.Removed) != 2 {
		t.Fatalf("limited answer %+v", part)
	}
	rest := f.snapshot(t, ctx, core.DAVAssignedCollection, part.Token, 0)
	if rest.Truncated || len(rest.Changed)+len(rest.Removed) != 3 {
		t.Fatalf("the rest %+v", rest)
	}
	got := append(memberNames(part.Changed), memberNames(rest.Changed)...)
	slices.Sort(got)
	if !slices.Equal(got, names(made...)) || !slices.Equal(append(part.Removed, rest.Removed...), names(gone)) {
		t.Errorf("across both: changed %v, removed %v %v", got, part.Removed, rest.Removed)
	}
}

func TestAnUnchangedCollectionWritesNoSnapshot(t *testing.T) {
	f := newDAVFixture(t)
	p := f.create(t, f.member, core.CreateDAVPasswordInput{Name: "iPhone"})
	f.task(t, "a", f.actor.ID)
	first := f.snapshot(t, f.as(p), core.DAVAssignedCollection, "", 0)
	seen := func() time.Time {
		var at time.Time
		if err := f.l.store.View(context.Background(), f.scope, func(tx store.Tx) error {
			st, err := tx.GetDAVSyncState(context.Background(), p.ID, core.DAVAssignedCollection,
				strings.TrimPrefix(first.Token, DAVSyncTokenPrefix))
			if err == nil {
				at = st.SeenAt
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return at
	}
	before := seen()
	f.clk.Advance(time.Minute)
	f.snapshot(t, f.as(p), core.DAVAssignedCollection, first.Token, 0)
	f.snapshot(t, f.as(p), core.DAVAssignedCollection, "", 0)
	if after := seen(); !after.Equal(before) {
		t.Errorf("polling an unchanged collection wrote its snapshot again (%v then %v)", before, after)
	}
}

func TestOldSyncTokensArePruned(t *testing.T) {
	f := newDAVFixture(t)
	p := f.create(t, f.member, core.CreateDAVPasswordInput{Name: "iPhone"})
	var tokens []string
	for range 5 {
		f.task(t, "x", f.actor.ID)
		tokens = append(tokens, f.snapshot(t, f.as(p), core.DAVAssignedCollection, "", 0).Token)
		f.clk.Advance(25 * time.Hour)
	}
	if _, err := f.l.DAVSnapshot(f.as(p), core.DAVSnapshotInput{Collection: core.DAVAssignedCollection,
		Since: tokens[0]}); !core.IsDAVSyncToken(err) {
		t.Errorf("the oldest token still answers: %v", err)
	}
	if _, err := f.l.DAVSnapshot(f.as(p), core.DAVSnapshotInput{Collection: core.DAVAssignedCollection,
		Since: tokens[4]}); err != nil {
		t.Errorf("the newest token: %v", err)
	}
}
