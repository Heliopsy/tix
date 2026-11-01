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

func mustCopy(t *testing.T, l *Local, ctx context.Context, src *core.Task, in core.CopyTaskInput) *core.Task {
	t.Helper()
	got, err := l.CopyTask(ctx, core.TaskRef{ID: src.ID}, in)
	if err != nil {
		t.Fatalf("copying %s: %v", src.Ref, err)
	}
	return got
}

func TestCopyCarriesTheTaskAndLeavesTheSourceAlone(t *testing.T) {
	l, ctx, scope, me, _ := newTaskFixture(t)
	seedTaskProject(t, l, scope, "ops")
	seedFieldDef(t, l, scope, core.FieldDef{ProjectID: projectIDOf(t, l, scope, "infra"), Key: "env", Label: "Env", Type: core.FieldString})
	due := time.Date(2027, 1, 2, 0, 0, 0, 0, time.UTC)
	src := mustCreateTask(t, l, ctx, core.CreateTaskInput{
		ProjectRef: "infra", Title: "rotate keys", Body: "all of them", Priority: core.PriorityHigh,
		Tags: []string{"sec", "ops"}, AssigneeActorID: me.ID, DueAt: &due, Recurrence: "every week",
		CustomFields: map[string]any{"env": "prod"},
	})
	mustStar(t, l, ctx, src.Ref)

	cases := []struct {
		name, project, title, wantPrefix, wantTitle string
	}{
		{"another project", "ops", "", "ops-", "rotate keys"},
		{"the same project", "", "", "infra-", "rotate keys"},
		{"a replacement title", "ops", "  rotate again ", "ops-", "rotate again"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mustCopy(t, l, ctx, src, core.CopyTaskInput{ProjectRef: tc.project, Title: tc.title})
			if !strings.HasPrefix(got.Ref, tc.wantPrefix) || got.ID == src.ID || got.Ref == src.Ref {
				t.Fatalf("ref = %s, want a fresh %s task", got.Ref, tc.wantPrefix)
			}
			if got.Title != tc.wantTitle || got.Body != src.Body || got.Priority != src.Priority ||
				got.AssigneeActorID != me.ID || got.DueAt == nil || !got.DueAt.Equal(due) ||
				got.Recurrence != src.Recurrence {
				t.Errorf("copy = %+v, want the source's attributes", got)
			}
			if !slices.Equal(sortedTags(got.Tags), sortedTags(src.Tags)) {
				t.Errorf("tags = %v, want %v", got.Tags, src.Tags)
			}
			if got.Starred || got.ClaimedByActorID != "" || got.CopiedFromID != "" {
				t.Errorf("personal or link state travelled: starred=%v claimed=%q link=%q",
					got.Starred, got.ClaimedByActorID, got.CopiedFromID)
			}
			if tc.project == "ops" && got.CustomFields["env"] != nil {
				t.Errorf("a field ops does not define travelled: %v", got.CustomFields)
			}
			if tc.project == "" && got.CustomFields["env"] != "prod" {
				t.Errorf("custom fields = %v, want env=prod", got.CustomFields)
			}
		})
	}
	after, err := l.GetTask(ctx, core.TaskRef{ID: src.ID})
	if err != nil || after.Version != src.Version || after.Title != src.Title {
		t.Fatalf("the source changed: %+v, %v", after, err)
	}
}

func sortedTags(tags []string) []string {
	out := slices.Clone(tags)
	slices.Sort(out)
	return out
}

func projectIDOf(t *testing.T, l *Local, scope core.TenantScope, key string) string {
	t.Helper()
	var id string
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		p, err := tx.GetProject(context.Background(), key)
		if err != nil {
			return err
		}
		id = p.ID
		return nil
	}); err != nil {
		t.Fatalf("finding project %q: %v", key, err)
	}
	return id
}

func TestCopyStatus(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	seedTaskProject(t, l, scope, "ops")
	seedTaskProjectWith(t, l, scope, "ship", shippingWorkflow())
	src := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "x", Status: "doing"})
	done := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "y", Status: "done"})

	cases := []struct {
		name      string
		src       *core.Task
		in        core.CopyTaskInput
		want      string
		completed bool
	}{
		{"initial by default", src, core.CopyTaskInput{ProjectRef: "ops"}, "todo", false},
		{"kept when asked", src, core.CopyTaskInput{ProjectRef: "ops", KeepStatus: true}, "doing", false},
		{"initial when the target lacks it", src, core.CopyTaskInput{ProjectRef: "ship", KeepStatus: true}, "backlog", false},
		{"a kept terminal state is completed", done, core.CopyTaskInput{KeepStatus: true}, "done", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mustCopy(t, l, ctx, tc.src, tc.in)
			if got.Status != tc.want || (got.CompletedAt != nil) != tc.completed {
				t.Fatalf("status = %q completed=%v, want %q completed=%v",
					got.Status, got.CompletedAt != nil, tc.want, tc.completed)
			}
		})
	}
}

func TestCopyOptions(t *testing.T) {
	l, ctx, scope, me, _ := newTaskFixture(t)
	seedTaskProject(t, l, scope, "ops")
	ana := seedTaskActor(t, l, scope, "ana", core.RoleMember, core.ScopeAll)
	blocker := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "blocker"})
	parent := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "parent"})
	root := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "root",
		ParentRef: parent.Ref, DependsOn: []string{blocker.Ref}})
	child := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "child", ParentRef: root.Ref})
	grandchild := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "grandchild",
		ParentRef: child.Ref, DependsOn: []string{child.Ref}})
	if _, err := l.AddComment(taskContext(ana), core.TaskRef{ID: root.ID}, "looks good"); err != nil {
		t.Fatal(err)
	}

	t.Run("nothing optional by default", func(t *testing.T) {
		got := mustCopy(t, l, ctx, root, core.CopyTaskInput{ProjectRef: "ops"})
		if n := len(treeOf(t, l, ctx, got)); n != 1 {
			t.Errorf("subtree = %d tasks, want the root alone", n)
		}
		if c := commentsOf(t, l, ctx, got); len(c) != 0 {
			t.Errorf("comments = %v, want none", c)
		}
		if got.ParentID != "" || len(got.DependsOn) != 0 {
			t.Errorf("parent %q deps %v, want a root task with no edges in another project", got.ParentID, got.DependsOn)
		}
	})
	t.Run("the same project keeps the parent", func(t *testing.T) {
		got := mustCopy(t, l, ctx, root, core.CopyTaskInput{})
		if got.ParentID != parent.ID {
			t.Errorf("parent = %q, want %q", got.ParentID, parent.ID)
		}
	})
	t.Run("everything", func(t *testing.T) {
		got := mustCopy(t, l, ctx, root, core.CopyTaskInput{ProjectRef: "ops",
			WithComments: true, WithSubtasks: true, WithDependencies: true, Link: true})
		tree := treeOf(t, l, ctx, got)
		if len(tree) != 3 {
			t.Fatalf("subtree = %d tasks, want 3", len(tree))
		}
		cChild, cGrand := tree[1], tree[2]
		if cChild.Title != "child" || cChild.ParentID != got.ID || cGrand.Title != "grandchild" || cGrand.ParentID != cChild.ID {
			t.Errorf("tree = %+v, want child under the copy and grandchild under the child's copy", tree)
		}
		if cChild.ProjectID == root.ProjectID || cGrand.ProjectID != cChild.ProjectID {
			t.Errorf("subtasks stayed in the source project")
		}
		if got.CopiedFromID != root.ID || cChild.CopiedFromID != child.ID || cGrand.CopiedFromID != grandchild.ID {
			t.Errorf("links = %q %q %q", got.CopiedFromID, cChild.CopiedFromID, cGrand.CopiedFromID)
		}
		if !slices.Equal(got.DependsOn, []string{blocker.ID}) {
			t.Errorf("root deps = %v, want the blocker", got.DependsOn)
		}
		if !slices.Equal(cGrand.DependsOn, []string{cChild.ID}) {
			t.Errorf("grandchild deps = %v, want the child's copy %s", cGrand.DependsOn, cChild.ID)
		}
		comments := commentsOf(t, l, ctx, got)
		if len(comments) != 1 {
			t.Fatalf("comments = %v, want one", comments)
		}
		c := comments[0]
		if c.AuthorActorID != me.ID || !strings.Contains(c.Body, "@ana") || !strings.HasSuffix(c.Body, "looks good") ||
			!strings.Contains(c.Body, " UTC") {
			t.Errorf("comment = %+v, want the copier's comment naming @ana and the time", c)
		}
	})
}

func treeOf(t *testing.T, l *Local, ctx context.Context, task *core.Task) []core.Task {
	t.Helper()
	tree, err := l.TaskTree(ctx, core.TaskRef{ID: task.ID}, 0)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func commentsOf(t *testing.T, l *Local, ctx context.Context, task *core.Task) []core.Comment {
	t.Helper()
	c, err := l.ListComments(ctx, core.TaskRef{ID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCopyRecordsOneAuditAndEventPerTask(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	seedTaskProject(t, l, scope, "ops")
	root := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "root"})
	mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "child", ParentRef: root.Ref})
	if _, err := l.AddComment(ctx, core.TaskRef{ID: root.ID}, "note"); err != nil {
		t.Fatal(err)
	}
	events, audits := countRows(t, l, scope)
	got := mustCopy(t, l, ctx, root, core.CopyTaskInput{ProjectRef: "ops", WithSubtasks: true, WithComments: true})
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents-events != 3 || afterAudits-audits != 3 {
		t.Fatalf("events +%d audits +%d, want +3 each (two tasks, one comment)", afterEvents-events, afterAudits-audits)
	}
	actions := auditActions(t, l, scope)
	if n := countOf(actions, "task.copy"); n != 2 {
		t.Errorf("task.copy entries = %d in %v, want 2", n, actions)
	}
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		evs, err := tx.ReadEvents(context.Background(), 0, 1000)
		if err != nil {
			return err
		}
		for _, e := range evs {
			if e.SubjectID == got.ID {
				if e.Type != core.EventTaskCreated || e.Payload["copied_from"] != root.Ref || e.Payload["ref"] != got.Ref {
					t.Errorf("event = %s %v", e.Type, e.Payload)
				}
				return nil
			}
		}
		t.Error("no event for the copy")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func countOf(list []string, want string) int {
	n := 0
	for _, v := range list {
		if v == want {
			n++
		}
	}
	return n
}

func TestRefusedCopyWritesNothing(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	ops := seedTaskProject(t, l, scope, "ops")
	seedFieldDef(t, l, scope, core.FieldDef{ProjectID: projectIDOf(t, l, scope, "infra"), Key: "team", Label: "Team", Type: core.FieldString})
	root := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "root", CustomFields: map[string]any{"team": "a"}})
	mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "child", ParentRef: root.Ref})
	seedFieldDef(t, l, scope, core.FieldDef{ProjectID: ops.ID, Key: "team", Label: "Team", Type: core.FieldString, Required: true})

	events, audits := countRows(t, l, scope)
	_, err := l.CopyTask(ctx, core.TaskRef{ID: root.ID}, core.CopyTaskInput{ProjectRef: "ops", WithSubtasks: true})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("err = %v, want invalid for the child missing a required field", err)
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != events || afterAudits != audits {
		t.Fatalf("a refused copy wrote: events %d -> %d, audits %d -> %d", events, afterEvents, audits, afterAudits)
	}
	page, err := l.ListTasks(ctx, core.TaskFilter{ProjectKeys: []string{"ops"}})
	if err != nil || len(page.Tasks) != 0 {
		t.Fatalf("ops holds %d tasks after a refused copy: %v", len(page.Tasks), err)
	}
}

func TestCopyAuthority(t *testing.T) {
	l, ctx, scope, _, infra := newTaskFixture(t)
	seedTaskProject(t, l, scope, "ops")
	src := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "x"})
	if _, err := l.AddComment(ctx, core.TaskRef{ID: src.ID}, "note"); err != nil {
		t.Fatal(err)
	}
	confined := seedTaskActor(t, l, scope, "pinned", core.RoleMember, core.ScopeAll)
	confined.ProjectID = infra.ID
	viewer := seedTaskActor(t, l, scope, "vee", core.RoleViewer, core.ScopeTaskRead)
	noComment := seedTaskActor(t, l, scope, "quiet", "", core.ScopeTaskRead, core.ScopeTaskWrite)

	cases := []struct {
		name  string
		actor *core.Actor
		in    core.CopyTaskInput
		ok    bool
	}{
		{"confined within its project", confined, core.CopyTaskInput{}, true},
		{"confined into another project", confined, core.CopyTaskInput{ProjectRef: "ops"}, false},
		{"read only", viewer, core.CopyTaskInput{}, false},
		{"write without comment", noComment, core.CopyTaskInput{}, true},
		{"comments need comment write", noComment, core.CopyTaskInput{WithComments: true}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := l.CopyTask(taskContext(tc.actor), core.TaskRef{ID: src.ID}, tc.in)
			if tc.ok && err != nil || !tc.ok && !core.IsKind(err, core.KindForbidden) {
				t.Fatalf("err = %v, want ok=%v", err, tc.ok)
			}
		})
	}
}

func TestCopyOfAMissingProjectOrTask(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	src := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "x"})
	if _, err := l.CopyTask(ctx, core.TaskRef{ID: src.ID}, core.CopyTaskInput{ProjectRef: "nope"}); !core.IsKind(err, core.KindNotFound) || core.FieldOf(err) != "project_ref" {
		t.Errorf("unknown project: err = %v", err)
	}
	if _, err := l.CopyTask(ctx, core.TaskRef{ProjectKey: "infra", Seq: 99}, core.CopyTaskInput{}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("unknown task: err = %v", err)
	}
	long := strings.Repeat("x", core.MaxTitleLength+1)
	if _, err := l.CopyTask(ctx, core.TaskRef{ID: src.ID}, core.CopyTaskInput{Title: long}); !core.IsKind(err, core.KindInvalid) || core.FieldOf(err) != "title" {
		t.Errorf("long title: err = %v", err)
	}
}

func TestCopiedCommentBodyStaysWithinTheLimit(t *testing.T) {
	at := time.Date(2026, 10, 31, 14, 2, 0, 0, time.UTC)
	body := copiedCommentBody("@ana", at, strings.Repeat("é", core.MaxCommentLength))
	if len(body) > core.MaxCommentLength {
		t.Fatalf("len = %d, over %d", len(body), core.MaxCommentLength)
	}
	if !strings.HasPrefix(body, "Copied from a comment by @ana on 2026-10-31 14:02 UTC:\n\n") {
		t.Fatalf("head = %q", body[:60])
	}
	if !strings.HasSuffix(copiedCommentBody("@ana", at, "short"), "\n\nshort") {
		t.Fatal("a short body was altered")
	}
}

func TestRoundTripKeepsTheCopiedFromLink(t *testing.T) {
	l, _, _, actor := newLocal(t)
	source := core.WithActor(context.Background(), actor)
	seed(t, l, source, "infra")
	page, err := l.ListTasks(source, core.TaskFilter{})
	if err != nil || len(page.Tasks) == 0 {
		t.Fatalf("listing the seed: %v", err)
	}
	orig := page.Tasks[0]
	if _, err := l.CopyTask(source, core.TaskRef{ID: orig.ID}, core.CopyTaskInput{Title: "the copy", Link: true}); err != nil {
		t.Fatalf("copying: %v", err)
	}
	snapshot := exportBytes(t, l, source, core.ExportInput{})

	target, _, _ := newTenant(t, l, "beta")
	if _, err := l.ImportFrom(target, bytes.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge}); err != nil {
		t.Fatalf("import: %v", err)
	}
	imported, err := l.ListTasks(target, core.TaskFilter{})
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	byTitle := map[string]core.Task{}
	for _, task := range imported.Tasks {
		byTitle[task.Title] = task
	}
	cp, src := byTitle["the copy"], byTitle[orig.Title]
	if cp.ID == "" || src.ID == "" || cp.CopiedFromID != src.ID {
		t.Fatalf("imported copy links %q, want the imported source %q", cp.CopiedFromID, src.ID)
	}
}
