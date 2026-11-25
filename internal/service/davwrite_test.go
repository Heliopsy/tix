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

// davTodo is a VCALENDAR holding one VTODO of the given lines.
func davTodo(lines ...string) []byte {
	all := append([]string{"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//test//EN", "BEGIN:VTODO"}, lines...)
	all = append(all, "END:VTODO", "END:VCALENDAR")
	return []byte(strings.Join(all, "\r\n") + "\r\n")
}

// object reads one object of a collection, failing when it is not there.
func (f davFixture) object(t *testing.T, ctx context.Context, collection, name string) core.DAVObject {
	t.Helper()
	objects, err := f.l.GetDAVObjects(ctx, core.DAVObjectsInput{Collection: collection, Names: []string{name},
		Origin: davOrigin})
	if err != nil || len(objects) != 1 {
		t.Fatalf("reading %s/%s: %d objects, %v", collection, name, len(objects), err)
	}
	return objects[0]
}

// put sends a client's object for an existing task, with its current ETag.
func (f davFixture) put(t *testing.T, ctx context.Context, task *core.Task, lines ...string) (*core.DAVPutResult, error) {
	t.Helper()
	name := strings.ToLower(task.ID)
	o := f.object(t, ctx, core.DAVAssignedCollection, name)
	return f.l.PutDAVObject(ctx, core.DAVPutInput{Collection: core.DAVAssignedCollection, Name: name,
		IfMatch: o.ETag, Data: davTodo(append([]string{"UID:task-" + name + "@tix"}, lines...)...), Origin: davOrigin})
}

func (f davFixture) reload(t *testing.T, task *core.Task) *core.Task {
	t.Helper()
	got, err := f.l.GetTask(f.admin, core.TaskRef{ID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// writer is a member with a writable app password over Assigned to me.
func (f davFixture) writer(t *testing.T) context.Context {
	t.Helper()
	return f.as(f.create(t, f.member, core.CreateDAVPasswordInput{Name: "iPhone", Collections: core.DAVCollectionsAll}))
}

func TestAnUpdateAppliesOnlyWhatChanged(t *testing.T) {
	due := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		lines []string
		check func(t *testing.T, got *core.Task)
	}{
		{"SUMMARY changes the title", []string{"SUMMARY:Rotate the keys"}, func(t *testing.T, got *core.Task) {
			if got.Title != "Rotate the keys" {
				t.Errorf("title %q", got.Title)
			}
		}},
		{"DESCRIPTION changes the body", []string{"DESCRIPTION:first\\nsecond\\, third"}, func(t *testing.T, got *core.Task) {
			if got.Body != "first\nsecond, third" {
				t.Errorf("body %q", got.Body)
			}
		}},
		{"PRIORITY 1 is highest", []string{"PRIORITY:1"}, func(t *testing.T, got *core.Task) {
			if got.Priority != core.PriorityHighest {
				t.Errorf("priority %d", got.Priority)
			}
		}},
		{"CATEGORIES replaces the tags", []string{"CATEGORIES:ops,home"}, func(t *testing.T, got *core.Task) {
			if !davSameTags(got.Tags, []string{"ops", "home"}) {
				t.Errorf("tags %q", got.Tags)
			}
		}},
		{"an empty CATEGORIES removes them", []string{"CATEGORIES:"}, func(t *testing.T, got *core.Task) {
			if len(got.Tags) != 0 {
				t.Errorf("tags %q", got.Tags)
			}
		}},
		{"DUE sets the date", []string{"DUE;VALUE=DATE:20260301"}, func(t *testing.T, got *core.Task) {
			if got.DueAt == nil || !got.DueAt.Equal(due) {
				t.Errorf("due %v", got.DueAt)
			}
		}},
		{"an absent DUE clears it", nil, func(t *testing.T, got *core.Task) {
			if got.DueAt != nil {
				t.Errorf("due %v", got.DueAt)
			}
		}},
		{"a past DTSTART changes nothing", []string{"DTSTART:20251215T090000Z"}, func(t *testing.T, got *core.Task) {
			if got.SnoozedUntil != nil {
				t.Errorf("snoozed until %v", got.SnoozedUntil)
			}
		}},
		{"a future DTSTART snoozes", []string{"DTSTART:20260115T090000Z"}, func(t *testing.T, got *core.Task) {
			if got.SnoozedUntil == nil || !got.SnoozedUntil.Equal(time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)) {
				t.Errorf("snoozed until %v", got.SnoozedUntil)
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newDAVFixture(t)
			ctx := f.writer(t)
			task := mustCreateTask(t, f.l, f.admin, core.CreateTaskInput{Title: "Rotate keys", ProjectRef: f.project.Key,
				AssigneeActorID: f.actor.ID, Tags: []string{"ops", "security"}, Body: "as written",
				DueAt: new(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC))})
			lines := tc.lines
			if tc.name != "an absent DUE clears it" && tc.name != "DUE sets the date" {
				lines = append(lines, "DUE;VALUE=DATE:20260201")
			}
			if _, err := f.put(t, ctx, task, lines...); err != nil {
				t.Fatal(err)
			}
			got := f.reload(t, task)
			tc.check(t, got)
			if tc.name != "CATEGORIES replaces the tags" && tc.name != "an empty CATEGORIES removes them" &&
				!davSameTags(got.Tags, task.Tags) {
				t.Errorf("tags changed to %q", got.Tags)
			}
			if !strings.HasPrefix(tc.name, "DESCRIPTION") && got.Body != task.Body {
				t.Errorf("body changed to %q", got.Body)
			}
		})
	}
}

func TestAppleDroppingCategoriesKeepsTheTags(t *testing.T) {
	f := newDAVFixture(t)
	ctx := f.writer(t)
	task := mustCreateTask(t, f.l, f.admin, core.CreateTaskInput{Title: "Rotate keys", ProjectRef: f.project.Key,
		AssigneeActorID: f.actor.ID, Tags: []string{"ops", "security"}, Priority: core.PriorityHigh})
	if _, err := f.put(t, ctx, task, "SUMMARY:Rotate keys today", "STATUS:NEEDS-ACTION",
		"X-APPLE-SORT-ORDER:3"); err != nil {
		t.Fatal(err)
	}
	got := f.reload(t, task)
	if got.Title != "Rotate keys today" || !davSameTags(got.Tags, []string{"ops", "security"}) ||
		got.Priority != core.PriorityHigh || got.Version != task.Version+1 {
		t.Errorf("after Apple's rewrite: %q %q %d v%d", got.Title, got.Tags, got.Priority, got.Version)
	}
}

// Thunderbird sends the whole object back, as it read it, with its own
// extensions added; nothing but those changes.
func TestThunderbirdSendingTheWholeObjectChangesNothingElse(t *testing.T) {
	f := newDAVFixture(t)
	ctx := f.writer(t)
	due := time.Date(2026, 2, 1, 14, 30, 0, 0, time.UTC)
	task := mustCreateTask(t, f.l, f.admin, core.CreateTaskInput{Title: "Rotate keys", ProjectRef: f.project.Key,
		AssigneeActorID: f.actor.ID, Tags: []string{"ops"}, Body: "a, b; c", DueAt: &due, Priority: core.PriorityLow})
	name := strings.ToLower(task.ID)
	o := f.object(t, ctx, core.DAVAssignedCollection, name)
	data := strings.Replace(string(o.Data), "END:VTODO", "X-MOZ-GENERATION:1\r\nX-MOZ-LASTACK:20260101T000000Z\r\nEND:VTODO", 1)
	if _, err := f.l.PutDAVObject(ctx, core.DAVPutInput{Collection: core.DAVAssignedCollection, Name: name,
		IfMatch: o.ETag, Data: []byte(data), Origin: davOrigin}); err != nil {
		t.Fatal(err)
	}
	if got := f.reload(t, task); got.Version != task.Version {
		t.Errorf("the task's version moved to %d for an unchanged object", got.Version)
	}
	after := f.object(t, ctx, core.DAVAssignedCollection, name)
	if after.ETag == o.ETag || !strings.Contains(string(after.Data), "X-MOZ-GENERATION:1") {
		t.Errorf("the kept extensions are not in the object, tag %s:\n%s", after.ETag, after.Data)
	}
}

// DAVx5 writes tasks.org's tick as STATUS, COMPLETED and PERCENT-COMPLETE.
func TestDAVx5CompletingATaskFinishesIt(t *testing.T) {
	f := newDAVFixture(t)
	ctx := f.writer(t)
	task := f.task(t, "Rotate keys", f.actor.ID)
	if _, err := f.put(t, ctx, task, "SUMMARY:Rotate keys", "STATUS:COMPLETED", "COMPLETED:20260101T000000Z",
		"PERCENT-COMPLETE:100"); err != nil {
		t.Fatal(err)
	}
	if got := f.reload(t, task); got.Status != "done" {
		t.Errorf("status %q", got.Status)
	}
	o := f.object(t, ctx, core.DAVAssignedCollection, strings.ToLower(task.ID))
	if strings.Count(string(o.Data), "PERCENT-COMPLETE") != 1 {
		t.Errorf("the done object:\n%s", o.Data)
	}
}

func TestStatusFromAClientMovesTheTaskThroughItsWorkflow(t *testing.T) {
	twoHop := core.WorkflowDefinition{Initial: "todo", States: []core.State{
		{Key: "todo", Category: core.CategoryTodo}, {Key: "doing", Category: core.CategoryInProgress},
		{Key: "done", Terminal: true, Category: core.CategoryDone},
	}, Transitions: []core.Transition{{From: "todo", To: "doing"}, {From: "doing", To: "done"}, {From: "done", To: "todo"}}}
	guarded := core.WorkflowDefinition{Initial: "todo", States: twoHop.States,
		Transitions: []core.Transition{{From: "todo", To: "done", RequiresComment: true}}}
	ambiguous := core.WorkflowDefinition{Initial: "todo", States: []core.State{
		{Key: "todo", Category: core.CategoryTodo}, {Key: "a", Category: core.CategoryInProgress},
		{Key: "b", Category: core.CategoryInProgress}, {Key: "done", Terminal: true, Category: core.CategoryDone},
	}, Transitions: []core.Transition{{From: "todo", To: "a"}, {From: "todo", To: "b"}, {From: "a", To: "done"},
		{From: "b", To: "done"}}}
	tests := []struct {
		name    string
		def     core.WorkflowDefinition
		start   string
		status  string
		want    string
		refused bool
		hops    int
	}{
		{"a two-hop done", twoHop, "", "STATUS:COMPLETED", "done", false, 2},
		{"a guarded edge", guarded, "", "STATUS:COMPLETED", "todo", true, 0},
		{"an ambiguous route", ambiguous, "", "STATUS:COMPLETED", "todo", true, 0},
		{"needs-action from done", twoHop, "done", "STATUS:NEEDS-ACTION", "todo", false, 1},
		{"no status from done reopens", twoHop, "done", "", "todo", false, 1},
		{"in-process", twoHop, "", "STATUS:IN-PROCESS", "doing", false, 1},
		{"COMPLETED without a status", twoHop, "", "COMPLETED:20260101T000000Z", "done", false, 2},
		{"PERCENT-COMPLETE:100 without a status", twoHop, "", "PERCENT-COMPLETE:100", "done", false, 2},
		{"no status in progress stays", twoHop, "doing", "", "doing", false, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newDAVFixture(t)
			project := seedTaskProjectWith(t, f.l, f.scope, "flow", tc.def)
			ctx := f.writer(t)
			task := mustCreateTask(t, f.l, f.admin, core.CreateTaskInput{Title: "x", ProjectRef: project.Key,
				AssigneeActorID: f.actor.ID})
			if tc.start != "" {
				if _, err := f.l.TransitionRoute(f.admin, core.TaskRef{ID: task.ID}, core.RouteInput{To: tc.start}); err != nil {
					t.Fatal(err)
				}
				task = f.reload(t, task)
			}
			seq := f.eventSeq(t)
			lines := []string{"SUMMARY:x, renamed on the phone"}
			if tc.status != "" {
				lines = append(lines, tc.status)
			}
			_, err := f.put(t, ctx, task, lines...)
			if tc.refused != (core.DAVReason(err) == core.ReasonDAVRefused) {
				t.Fatalf("answered %v", err)
			}
			got := f.reload(t, task)
			if got.Status != tc.want || tc.refused && (got.Version != task.Version || got.Title != task.Title) {
				t.Errorf("status %q version %d title %q, want %q", got.Status, got.Version, got.Title, tc.want)
			}
			if n := len(f.eventsSince(t, seq, core.EventTaskTransitioned)); n != tc.hops {
				t.Errorf("%d transitions, want %d", n, tc.hops)
			}
		})
	}
}

func (f davFixture) eventSeq(t *testing.T) int64 {
	t.Helper()
	var seq int64
	if err := f.l.store.View(context.Background(), f.scope, func(tx store.Tx) error {
		var err error
		seq, err = tx.LatestEventSeq(context.Background())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return seq
}

func (f davFixture) eventsSince(t *testing.T, seq int64, kind core.EventType) []core.Event {
	t.Helper()
	var out []core.Event
	if err := f.l.store.View(context.Background(), f.scope, func(tx store.Tx) error {
		events, err := tx.ReadEvents(context.Background(), seq, 1000)
		for _, e := range events {
			if e.Type == kind {
				out = append(out, e)
			}
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestAnUpdateNeedsTheCurrentETag(t *testing.T) {
	f := newDAVFixture(t)
	ctx := f.writer(t)
	task := f.task(t, "Rotate keys", f.actor.ID)
	name := strings.ToLower(task.ID)
	o := f.object(t, ctx, core.DAVAssignedCollection, name)
	body := davTodo("UID:task-"+name+"@tix", "SUMMARY:from the phone")
	put := func(ifMatch string, none bool) error {
		_, err := f.l.PutDAVObject(ctx, core.DAVPutInput{Collection: core.DAVAssignedCollection, Name: name,
			IfMatch: ifMatch, IfNoneMatch: none, Data: body, Origin: davOrigin})
		return err
	}
	if err := put("", false); core.DAVReason(err) != core.ReasonDAVIfMatchRequired {
		t.Errorf("no If-Match: %v", err)
	}
	if err := put(o.ETag, true); core.DAVReason(err) != core.ReasonDAVStale {
		t.Errorf("If-None-Match on an existing object: %v", err)
	}
	title := "from the browser"
	if _, err := f.l.UpdateTask(f.admin, core.TaskRef{ID: task.ID}, core.UpdateTaskInput{Title: &title}); err != nil {
		t.Fatal(err)
	}
	if err := put(o.ETag, false); core.DAVReason(err) != core.ReasonDAVStale {
		t.Errorf("a stale If-Match: %v", err)
	}
	if got := f.reload(t, task); got.Title != "from the browser" {
		t.Errorf("the browser's edit was overwritten: %q", got.Title)
	}
	if err := put(`"other", `+f.object(t, ctx, core.DAVAssignedCollection, name).ETag, false); err != nil {
		t.Errorf("a current If-Match in a list: %v", err)
	}
	if err := put("*", false); err != nil {
		t.Errorf("If-Match *: %v", err)
	}
}

// The version an ETag stands for is what the update is made with, so a
// browser edit landing between the check and the write is a conflict.
func TestABrowserEditRacingAPhoneEditIsAConflict(t *testing.T) {
	f := newDAVFixture(t)
	ctx := f.writer(t)
	task := f.task(t, "Rotate keys", f.actor.ID)
	e := davEntry{task: *task, category: core.CategoryTodo, def: davTestWorkflow()}
	w, err := f.l.davWriterOf(ctx, core.DAVAssignedCollection, davTodo("UID:x", "SUMMARY:from the phone"))
	if err != nil {
		t.Fatal(err)
	}
	title := "from the browser"
	if _, err := f.l.UpdateTask(f.admin, core.TaskRef{ID: task.ID}, core.UpdateTaskInput{Title: &title}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.apply(&e); core.DAVReason(err) != core.ReasonDAVStale {
		t.Fatalf("the racing write answered %v", err)
	}
	if got := f.reload(t, task); got.Title != "from the browser" {
		t.Errorf("title %q", got.Title)
	}
}

func TestAClientCreatesATask(t *testing.T) {
	f := newDAVFixture(t)
	ctx := f.writer(t)
	res, err := f.l.PutDAVObject(ctx, core.DAVPutInput{Collection: core.DAVAssignedCollection, Name: "ABC-1",
		IfNoneMatch: true, Data: davTodo("UID:ABC-1-UID", "SUMMARY:Call the bank", "CATEGORIES:home",
			"BEGIN:VALARM", "ACTION:DISPLAY", "TRIGGER:-PT15M", "END:VALARM"), Origin: davOrigin})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Created || res.Task.Title != "Call the bank" || res.Task.ProjectID != f.project.ID ||
		res.Task.AssigneeActorID != f.actor.ID || !slices.Equal(res.Task.Tags, []string{"home"}) {
		t.Fatalf("created %+v", res.Task)
	}
	o := f.object(t, ctx, core.DAVAssignedCollection, "ABC-1")
	if !strings.Contains(string(o.Data), "UID:ABC-1-UID\r\n") || !strings.Contains(string(o.Data), "TRIGGER:-PT15M") {
		t.Errorf("the created object:\n%s", o.Data)
	}
	if objects, _ := f.l.GetDAVObjects(ctx, core.DAVObjectsInput{Collection: core.DAVAssignedCollection,
		Names: []string{"abc-1"}}); len(objects) != 0 {
		t.Error("a client's name was found in another case")
	}
	project := f.object(t, ctx, f.projectCollection(), "ABC-1")
	if project.ETag != o.ETag {
		t.Error("the same object has two tags in two collections")
	}

	evolution, err := f.l.PutDAVObject(ctx, core.DAVPutInput{Collection: f.projectCollection(), Name: "evo",
		Data: davTodo("UID:evo-uid", "SUMMARY:From Evolution"), Origin: davOrigin})
	if err != nil || !evolution.Created || evolution.Task.AssigneeActorID != "" {
		t.Fatalf("Evolution's create without If-None-Match: %+v %v", evolution, err)
	}
	for name, in := range map[string]core.DAVPutInput{
		"the same UID":       {Name: "other", Data: davTodo("UID:ABC-1-UID", "SUMMARY:again")},
		"a tix task's UID":   {Name: "other", Data: davTodo("UID:task-"+strings.ToLower(res.Task.ID)+"@tix", "SUMMARY:again")},
		"a tix task's name":  {Name: strings.ToLower(f.task(t, "trashed", "").ID), Data: davTodo("UID:new", "SUMMARY:again")},
		"another collection": {Name: "evo", Data: davTodo("UID:new2", "SUMMARY:again")},
	} {
		in.Collection, in.IfNoneMatch = core.DAVAssignedCollection, true
		if _, err := f.l.PutDAVObject(ctx, in); core.DAVReason(err) != core.ReasonDAVUIDConflict {
			t.Errorf("%s: %v", name, err)
		}
	}
	for name, in := range map[string]core.DAVPutInput{
		"an If-Match on a new name": {Name: "new3", IfMatch: `"x"`, Data: davTodo("UID:new3", "SUMMARY:x")},
		"no summary":                {Name: "new4", Data: davTodo("UID:new4")},
		"an event":                  {Name: "new5", Data: []byte("BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:e\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")},
	} {
		in.Collection = core.DAVAssignedCollection
		if _, err := f.l.PutDAVObject(ctx, in); core.DAVReason(err) == "" {
			t.Errorf("%s was accepted", name)
		}
	}
}

func TestDeleteCancelsAndNeverTrashes(t *testing.T) {
	f := newDAVFixture(t)
	ctx := f.writer(t)
	open := f.task(t, "open", f.actor.ID)
	done := f.task(t, "done", f.actor.ID)
	f.move(t, done, "done")
	done = f.reload(t, done)
	del := func(task *core.Task) error {
		return f.l.DeleteDAVObject(ctx, core.DAVDeleteInput{Collection: core.DAVAssignedCollection,
			Name: strings.ToLower(task.ID), Origin: davOrigin})
	}
	if err := del(open); err != nil {
		t.Fatal(err)
	}
	if got := f.reload(t, open); got.Status != "cancelled" || got.DeletedAt != nil {
		t.Errorf("deleted open task is %q, trashed %v", got.Status, got.DeletedAt)
	}
	if err := del(done); err != nil {
		t.Fatal(err)
	}
	if got := f.reload(t, done); got.Status != "done" || got.Version != done.Version || got.DeletedAt != nil {
		t.Errorf("a deleted done task changed: %q v%d", got.Status, got.Version)
	}
	if err := del(open); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("deleting a cancelled task again: %v", err)
	}
	if err := f.l.DeleteDAVObject(ctx, core.DAVDeleteInput{Collection: core.DAVAssignedCollection,
		Name: strings.ToLower(done.ID), IfMatch: `"stale"`}); core.DAVReason(err) != core.ReasonDAVStale {
		t.Errorf("a stale If-Match on delete: %v", err)
	}

	noCancel := core.WorkflowDefinition{Initial: "todo", States: []core.State{
		{Key: "todo", Category: core.CategoryTodo}, {Key: "done", Terminal: true, Category: core.CategoryDone},
	}, Transitions: []core.Transition{{From: "todo", To: "done"}}}
	project := seedTaskProjectWith(t, f.l, f.scope, "nocancel", noCancel)
	stuck := mustCreateTask(t, f.l, f.admin, core.CreateTaskInput{Title: "stuck", ProjectRef: project.Key,
		AssigneeActorID: f.actor.ID})
	if err := del(stuck); core.DAVReason(err) != core.ReasonDAVRefused {
		t.Errorf("no reachable cancelled state: %v", err)
	}
	if got := f.reload(t, stuck); got.Status != "todo" || got.Version != stuck.Version {
		t.Errorf("the task changed: %q", got.Status)
	}
}

func TestKeptPropertiesStayWithTheirOwner(t *testing.T) {
	f := newDAVFixture(t)
	bobCtx := f.writer(t)
	ana, anaActor := f.person(t, "ana@example.com", "ana")
	anaCtx := f.as(f.create(t, ana, core.CreateDAVPasswordInput{Name: "Mac", Collections: core.DAVCollectionsProjects}))
	task := f.task(t, "shared", f.actor.ID)
	_ = anaActor
	name := strings.ToLower(task.ID)
	before := f.snapshot(t, bobCtx, core.DAVAssignedCollection, "", 0)
	alarm := []string{"BEGIN:VALARM", "ACTION:DISPLAY", "TRIGGER:-PT15M", "END:VALARM"}
	if _, err := f.put(t, bobCtx, task, append([]string{"SUMMARY:shared", "RRULE:FREQ=WEEKLY",
		"LOCATION:office"}, alarm...)...); err != nil {
		t.Fatal(err)
	}
	mine := string(f.object(t, bobCtx, core.DAVAssignedCollection, name).Data)
	if !strings.Contains(mine, strings.Join(alarm, "\r\n")) || !strings.Contains(mine, "LOCATION:office") ||
		strings.Contains(mine, "RRULE") {
		t.Errorf("bob's object:\n%s", mine)
	}
	theirs := string(f.object(t, anaCtx, f.projectCollection(), name).Data)
	if strings.Contains(theirs, "VALARM") || strings.Contains(theirs, "office") {
		t.Errorf("ana sees bob's alarm:\n%s", theirs)
	}
	after := f.snapshot(t, bobCtx, core.DAVAssignedCollection, before.Token, 0)
	if len(after.Changed) != 1 || after.Changed[0].Name != name {
		t.Errorf("the sync after a kept-only change reports %+v", after.Changed)
	}

	big := []string{"SUMMARY:shared", "X-HUGE:" + strings.Repeat("a", MaxDAVKeptBytes)}
	res, err := f.put(t, bobCtx, task, big...)
	if err != nil || !res.KeptDropped {
		t.Fatalf("an oversized extra: %+v %v", res, err)
	}
	if data := string(f.object(t, bobCtx, core.DAVAssignedCollection, name).Data); strings.Contains(data, "X-HUGE") ||
		strings.Contains(data, "VALARM") {
		t.Errorf("over the bound, something was kept:\n%s", data)
	}
}

func TestAReadOnlyPasswordOrAViewerCannotWrite(t *testing.T) {
	f := newDAVFixture(t)
	task := f.task(t, "Rotate keys", f.actor.ID)
	ro := f.as(f.create(t, f.member, core.CreateDAVPasswordInput{Name: "Mac", ReadOnly: true}))
	data := davTodo("UID:task-"+strings.ToLower(task.ID)+"@tix", "SUMMARY:changed")
	put := func(ctx context.Context) error {
		_, err := f.l.PutDAVObject(ctx, core.DAVPutInput{Collection: core.DAVAssignedCollection,
			Name: strings.ToLower(task.ID), IfMatch: "*", Data: data})
		return err
	}
	if err := put(ro); core.DAVReason(err) != core.ReasonDAVReadOnly {
		t.Errorf("a read-only password: %v", err)
	}
	if p, _ := f.l.DAVPrincipal(ro); p.Write {
		t.Error("a read-only password's principal writes")
	}
	rw := f.writer(t)
	if p, _ := f.l.DAVPrincipal(rw); !p.Write {
		t.Error("a member's principal does not write")
	}
	if _, err := f.l.UpdateUser(f.admin, f.actor.ID, core.UpdateUserInput{Role: ptrRole(core.RoleViewer)}); err != nil {
		t.Fatal(err)
	}
	if err := put(rw); core.DAVReason(err) != core.ReasonDAVReadOnly {
		t.Errorf("after a demotion to viewer: %v", err)
	}
	if err := f.l.DeleteDAVObject(rw, core.DAVDeleteInput{Collection: core.DAVAssignedCollection,
		Name: strings.ToLower(task.ID)}); core.DAVReason(err) != core.ReasonDAVReadOnly {
		t.Errorf("a viewer's delete: %v", err)
	}
	if got := f.reload(t, task); got.Title != task.Title || got.Status != task.Status {
		t.Errorf("the task changed: %+v", got)
	}
}

func TestADAVWriteIsAnOrdinaryEventOnTheCalDAVChannel(t *testing.T) {
	f := newDAVFixture(t)
	p := f.create(t, f.member, core.CreateDAVPasswordInput{Name: "iPhone"})
	ctx := core.WithVia(f.as(p), core.Via{Kind: core.ViaCalDAV, TokenID: p.ID, TokenName: p.TokenName})
	task := f.task(t, "Rotate keys", f.actor.ID)
	seq := f.eventSeq(t)
	if _, err := f.put(t, ctx, task, "SUMMARY:Rotate keys now"); err != nil {
		t.Fatal(err)
	}
	events := f.eventsSince(t, seq, core.EventTaskUpdated)
	if len(events) != 1 || events[0].Via == nil || events[0].Via.Kind != core.ViaCalDAV ||
		events[0].Via.TokenName != p.TokenName || events[0].ActorID != f.actor.ID {
		t.Fatalf("events %+v", events)
	}
	if got := f.reload(t, task); got.Version != task.Version+1 {
		t.Errorf("version %d, want %d", got.Version, task.Version+1)
	}
	entries, _, err := f.l.ListAudit(f.admin, core.AuditFilter{Vias: []core.ViaKind{core.ViaCalDAV}})
	if err != nil || len(entries) != 1 || entries[0].Action != "task.update" {
		t.Errorf("audit %+v %v", entries, err)
	}
}
