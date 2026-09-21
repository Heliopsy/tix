package integration

import (
	"reflect"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// svcScenario is one row of the transport-equivalence table: a sequence of
// core.Service calls run once against a direct *service.Local and once
// against a *client.Client fronting an httptest server wrapping the SAME
// Local, expecting identical outcomes and, especially, identical error
// kinds. Adding a scenario is adding one entry to serviceScenarios; nothing
// else in this file changes.
type svcScenario struct {
	name string
	// exempt, when non-empty, explains why this row is not run rather than
	// silently dropping it from the table.
	exempt string
	run    func(t *testing.T, tg target, h *matrixHarness) (sig, error)
}

// serviceScenarios covers task lifecycle, claiming, relations and the
// failure paths named in the task: unknown reference, illegal transition,
// claim conflict, expired lease token, permission denial and a precondition
// failure (a stale optimistic-concurrency version).
var serviceScenarios = []svcScenario{
	{
		name: "create a task",
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{
				ProjectRef: p.Key, Title: "widget", Priority: core.PriorityHigh,
			})
			if err != nil {
				return nil, err
			}
			return sig{"title": task.Title, "status": task.Status, "priority": task.Priority}, nil
		},
	},
	{
		name: "read an unknown task",
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			_, err := tg.svc.GetTask(tg.ctx, core.TaskRef{ID: "nosuchtask00000000"})
			return nil, err
		},
	},
	{
		name: "update a task",
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "before"})
			mustf(t, tg, err, "creating the task")
			renamed := "after"
			updated, err := tg.svc.UpdateTask(tg.ctx, core.TaskRef{ID: task.ID}, core.UpdateTaskInput{Title: &renamed})
			if err != nil {
				return nil, err
			}
			return sig{"title": updated.Title, "version": updated.Version}, nil
		},
	},
	{
		name: "a legal transition",
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "t"})
			mustf(t, tg, err, "creating the task")
			out, err := tg.svc.TransitionTask(tg.ctx, core.TaskRef{ID: task.ID}, core.TransitionInput{To: "doing"})
			if err != nil {
				return nil, err
			}
			return sig{"status": out.Status}, nil
		},
	},
	{
		name: "an illegal transition",
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "t"})
			mustf(t, tg, err, "creating the task")
			// The builtin workflow has no todo -> done edge; only doing -> done.
			_, err = tg.svc.TransitionTask(tg.ctx, core.TaskRef{ID: task.ID}, core.TransitionInput{To: "done"})
			return nil, err
		},
	},
	{
		name: "delete then read is not found",
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "t"})
			mustf(t, tg, err, "creating the task")
			mustf(t, tg, tg.svc.DeleteTask(tg.ctx, core.TaskRef{ID: task.ID}, core.DeleteTaskInput{}), "deleting the task")
			_, err = tg.svc.GetTask(tg.ctx, core.TaskRef{ID: task.ID})
			return nil, err
		},
	},
	{
		name: "claim next",
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			_, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "queued"})
			mustf(t, tg, err, "creating the task")
			claim, err := tg.svc.ClaimNext(tg.ctx, core.ClaimNextInput{ProjectRefs: []string{p.Key}})
			if err != nil {
				return nil, err
			}
			return sig{"status": claim.Task.Status, "has_lease": claim.LeaseToken != ""}, nil
		},
	},
	{
		name: "claim next with nothing eligible",
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			_, err := tg.svc.ClaimNext(tg.ctx, core.ClaimNextInput{ProjectRefs: []string{p.Key}})
			return nil, err
		},
	},
	{
		name: "claim by ref",
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "t"})
			mustf(t, tg, err, "creating the task")
			claim, err := tg.svc.ClaimTask(tg.ctx, core.TaskRef{ID: task.ID}, core.ClaimInput{})
			if err != nil {
				return nil, err
			}
			return sig{"status": claim.Task.Status, "has_lease": claim.LeaseToken != ""}, nil
		},
	},
	{
		name: "a claim conflict",
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "t"})
			mustf(t, tg, err, "creating the task")
			_, err = tg.svc.ClaimTask(tg.ctx, core.TaskRef{ID: task.ID}, core.ClaimInput{ActorID: h.adminActor.ID})
			mustf(t, tg, err, "the first claim")
			_, err = tg.svc.ClaimTask(tg.ctx, core.TaskRef{ID: task.ID}, core.ClaimInput{ActorID: "someone-else-1"})
			return nil, err
		},
	},
	{
		name: "renew a held lease",
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "t"})
			mustf(t, tg, err, "creating the task")
			claim, err := tg.svc.ClaimTask(tg.ctx, core.TaskRef{ID: task.ID}, core.ClaimInput{})
			mustf(t, tg, err, "claiming the task")
			renewed, err := tg.svc.RenewLease(tg.ctx, core.TaskRef{ID: task.ID}, claim.LeaseToken, core.Duration(time.Hour))
			if err != nil {
				return nil, err
			}
			return sig{"renewed": renewed.LeaseExpiresAt.After(claim.LeaseExpiresAt)}, nil
		},
	},
	{
		name: "renew with the wrong token",
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "t"})
			mustf(t, tg, err, "creating the task")
			_, err = tg.svc.ClaimTask(tg.ctx, core.TaskRef{ID: task.ID}, core.ClaimInput{})
			mustf(t, tg, err, "claiming the task")
			_, err = tg.svc.RenewLease(tg.ctx, core.TaskRef{ID: task.ID}, "not-the-real-token", core.Duration(time.Hour))
			return nil, err
		},
	},
	{
		name: "release a held lease",
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "t"})
			mustf(t, tg, err, "creating the task")
			claim, err := tg.svc.ClaimTask(tg.ctx, core.TaskRef{ID: task.ID}, core.ClaimInput{})
			mustf(t, tg, err, "claiming the task")
			err = tg.svc.ReleaseLease(tg.ctx, core.TaskRef{ID: task.ID}, claim.LeaseToken, core.ReleaseInput{})
			if err != nil {
				return nil, err
			}
			after, err := tg.svc.GetTask(tg.ctx, core.TaskRef{ID: task.ID})
			mustf(t, tg, err, "reading the task back")
			return sig{"claimed_by": after.ClaimedByActorID}, nil
		},
	},
	{
		name: "an expired lease token cannot renew",
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "t"})
			mustf(t, tg, err, "creating the task")
			claim, err := tg.svc.ClaimTask(tg.ctx, core.TaskRef{ID: task.ID}, core.ClaimInput{TTL: core.Duration(time.Minute)})
			mustf(t, tg, err, "claiming the task")
			h.clk.Advance(2 * time.Minute)
			_, err = tg.svc.RenewLease(tg.ctx, core.TaskRef{ID: task.ID}, claim.LeaseToken, core.Duration(time.Hour))
			return nil, err
		},
	},
	{
		name: "comments round-trip",
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "t"})
			mustf(t, tg, err, "creating the task")
			_, err = tg.svc.AddComment(tg.ctx, core.TaskRef{ID: task.ID}, "note")
			if err != nil {
				return nil, err
			}
			comments, err := tg.svc.ListComments(tg.ctx, core.TaskRef{ID: task.ID})
			mustf(t, tg, err, "listing comments")
			if len(comments) != 1 {
				t.Fatalf("[%s] comments = %d, want 1", tg.name, len(comments))
			}
			return sig{"count": len(comments), "body": comments[0].Body}, nil
		},
	},
	{
		name: "tags round-trip",
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "t"})
			mustf(t, tg, err, "creating the task")
			err = tg.svc.AddTag(tg.ctx, core.TaskRef{ID: task.ID}, "urgent")
			if err != nil {
				return nil, err
			}
			after, err := tg.svc.GetTask(tg.ctx, core.TaskRef{ID: task.ID})
			mustf(t, tg, err, "reading the task back")
			return sig{"tags": after.Tags}, nil
		},
	},
	{
		name: "dependencies round-trip",
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			a, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "a"})
			mustf(t, tg, err, "creating task a")
			b, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "b"})
			mustf(t, tg, err, "creating task b")
			err = tg.svc.AddDependency(tg.ctx, core.TaskRef{ID: a.ID}, core.TaskRef{ID: b.ID})
			if err != nil {
				return nil, err
			}
			deps, err := tg.svc.ListDependencies(tg.ctx, core.TaskRef{ID: a.ID})
			mustf(t, tg, err, "listing dependencies")
			return sig{"dep_count": len(deps)}, nil
		},
	},
	{
		name: "listing with a status filter",
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			todo, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "stays todo"})
			mustf(t, tg, err, "creating a todo task")
			doing, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "moves on"})
			mustf(t, tg, err, "creating a second task")
			_, err = tg.svc.TransitionTask(tg.ctx, core.TaskRef{ID: doing.ID}, core.TransitionInput{To: "doing"})
			mustf(t, tg, err, "transitioning the second task")

			page, err := tg.svc.ListTasks(tg.ctx, core.TaskFilter{ProjectKeys: []string{p.Key}, Statuses: []string{"todo"}})
			if err != nil {
				return nil, err
			}
			titles := make([]string, len(page.Tasks))
			for i, tsk := range page.Tasks {
				titles[i] = tsk.Title
			}
			_ = todo
			return sig{"titles": titles}, nil
		},
	},
	{
		name: "keyset pagination visits every task exactly once",
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			const n = 5
			want := map[string]bool{}
			for i := 0; i < n; i++ {
				title := "task-" + string(rune('a'+i))
				_, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: title})
				mustf(t, tg, err, "creating task %d", i)
				want[title] = true
			}
			seen := map[string]bool{}
			cursor := ""
			for i := 0; i < n+1; i++ { // one extra round trip would prove a broken cursor loops forever
				page, err := tg.svc.ListTasks(tg.ctx, core.TaskFilter{
					ProjectKeys: []string{p.Key}, Page: core.Page{Limit: 2, Cursor: cursor},
				})
				if err != nil {
					return nil, err
				}
				for _, tsk := range page.Tasks {
					if seen[tsk.Title] {
						t.Fatalf("[%s] keyset pagination revisited %q", tg.name, tsk.Title)
					}
					seen[tsk.Title] = true
				}
				if page.NextCursor == "" {
					break
				}
				cursor = page.NextCursor
			}
			return sig{"seen": len(seen), "matches_created": reflect.DeepEqual(seen, want)}, nil
		},
	},
	{
		name: "a stale version update is a conflict",
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "t"})
			mustf(t, tg, err, "creating the task")
			title := "first edit"
			_, err = tg.svc.UpdateTask(tg.ctx, core.TaskRef{ID: task.ID}, core.UpdateTaskInput{Title: &title})
			mustf(t, tg, err, "the first edit")
			stale := "second edit, stale version"
			_, err = tg.svc.UpdateTask(tg.ctx, core.TaskRef{ID: task.ID},
				core.UpdateTaskInput{Title: &stale, Version: task.Version})
			return nil, err
		},
	},
	{
		name: "a scope-restricted actor is forbidden from writing",
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			svc, ctx := h.scoped(tg, core.ScopeTaskRead)
			_, err := svc.CreateTask(ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "should be refused"})
			return nil, err
		},
	},
	{
		name: "event subscription over the wire",
		exempt: "already exercised end to end, with precise timing assertions, by " +
			"TestDirectDatabaseWriteReachesAConnectedWebSocketClient and its neighbours in " +
			"outbox_test.go; duplicating that here would only add a second flaky-timing surface " +
			"for the same coverage",
	},
}

func TestTransportEquivalence(t *testing.T) {
	for _, sc := range serviceScenarios {
		sc := sc
		t.Run(sc.name, func(t *testing.T) {
			t.Parallel()
			if sc.exempt != "" {
				t.Skipf("exempt: %s", sc.exempt)
			}

			localH := newMatrixHarness(t)
			localSig, localErr := sc.run(t, localH.localTarget(), localH)

			remoteH := newMatrixHarness(t)
			remoteSig, remoteErr := sc.run(t, remoteH.remoteTarget(), remoteH)

			localKind, remoteKind := core.KindOf(localErr), core.KindOf(remoteErr)
			if localKind != remoteKind {
				t.Fatalf("error kind diverges between transports:\n"+
					"  direct service.Local: kind=%q err=%v\n"+
					"  client.Client:        kind=%q err=%v",
					localKind, localErr, remoteKind, remoteErr)
			}
			if localErr != nil {
				return
			}
			if !reflect.DeepEqual(localSig, remoteSig) {
				t.Fatalf("result diverges between transports:\n  direct service.Local: %#v\n  client.Client:        %#v",
					localSig, remoteSig)
			}
		})
	}
}
