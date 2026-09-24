// SPDX-License-Identifier: AGPL-3.0-or-later

package integration

import (
	"bytes"
	"reflect"
	"sort"
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
	// covers names the core.Service methods the row drives on both transports.
	// TestEveryOperationIsCoveredOrExempt reads it, so an operation with no
	// scenario and no exemption fails the build instead of quietly going
	// unverified. List what the row actually calls, helpers included.
	covers []string
	// exempt, when non-empty, explains why this row is not run rather than
	// silently dropping it from the table. An exempt row's covers are what the
	// exemption is attributed to.
	exempt string
	run    func(t *testing.T, tg target, h *matrixHarness) (sig, error)
}

// serviceScenarios covers task lifecycle, claiming, relations and the
// failure paths named in the task: unknown reference, illegal transition,
// claim conflict, expired lease token, permission denial and a precondition
// failure (a stale optimistic-concurrency version).
var serviceScenarios = []svcScenario{
	{
		name:   "create a task",
		covers: []string{"CreateProject", "CreateTask"},
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
		name:   "read an unknown task",
		covers: []string{"GetTask"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			_, err := tg.svc.GetTask(tg.ctx, core.TaskRef{ID: "nosuchtask00000000"})
			return nil, err
		},
	},
	{
		name:   "update a task",
		covers: []string{"CreateProject", "CreateTask", "UpdateTask"},
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
		name:   "a legal transition",
		covers: []string{"CreateProject", "CreateTask", "TransitionTask"},
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
		name:   "an illegal transition",
		covers: []string{"CreateProject", "CreateTask", "TransitionTask"},
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
		name:   "delete then read is not found",
		covers: []string{"CreateProject", "CreateTask", "DeleteTask", "GetTask"},
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
		name:   "claim next",
		covers: []string{"CreateProject", "CreateTask", "ClaimNext"},
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
		name:   "claim next with nothing eligible",
		covers: []string{"CreateProject", "ClaimNext"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			_, err := tg.svc.ClaimNext(tg.ctx, core.ClaimNextInput{ProjectRefs: []string{p.Key}})
			return nil, err
		},
	},
	{
		name:   "claim by ref",
		covers: []string{"CreateProject", "CreateTask", "ClaimTask"},
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
		name:   "a claim conflict",
		covers: []string{"CreateProject", "CreateTask", "ClaimTask"},
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
		name:   "renew a held lease",
		covers: []string{"CreateProject", "CreateTask", "ClaimTask", "RenewLease"},
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
		name:   "renew with the wrong token",
		covers: []string{"CreateProject", "CreateTask", "ClaimTask", "RenewLease"},
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
		name:   "release a held lease",
		covers: []string{"CreateProject", "CreateTask", "ClaimTask", "ReleaseLease", "GetTask"},
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
		name:   "an expired lease token cannot renew",
		covers: []string{"CreateProject", "CreateTask", "ClaimTask", "RenewLease"},
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
		name:   "comments round-trip",
		covers: []string{"CreateProject", "CreateTask", "AddComment", "ListComments"},
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
		name:   "tags round-trip",
		covers: []string{"CreateProject", "CreateTask", "AddTag", "GetTask"},
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
		name:   "dependencies round-trip",
		covers: []string{"CreateProject", "CreateTask", "AddDependency", "ListDependencies"},
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
		name:   "listing with a status filter",
		covers: []string{"CreateProject", "CreateTask", "TransitionTask", "ListTasks"},
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
		name:   "keyset pagination visits every task exactly once",
		covers: []string{"CreateProject", "CreateTask", "ListTasks"},
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
		name:   "a stale version update is a conflict",
		covers: []string{"CreateProject", "CreateTask", "UpdateTask"},
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
		name:   "a scope-restricted actor is forbidden from writing",
		covers: []string{"CreateProject", "CreateTask"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			svc, ctx := h.scoped(tg, core.ScopeTaskRead)
			_, err := svc.CreateTask(ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "should be refused"})
			return nil, err
		},
	},
	{
		name:   "event subscription over the wire",
		covers: []string{"Subscribe"},
		exempt: "already exercised end to end, with precise timing assertions, by " +
			"TestDirectDatabaseWriteReachesAConnectedWebSocketClient and its neighbours in " +
			"outbox_test.go; duplicating that here would only add a second flaky-timing surface " +
			"for the same coverage",
	},
	{
		name:   "read a project back by key",
		covers: []string{"CreateProject", "GetProject"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			got, err := tg.svc.GetProject(tg.ctx, p.Key)
			if err != nil {
				return nil, err
			}
			return sig{"key": got.Key == p.Key, "name": got.Name == p.Name}, nil
		},
	},
	{
		name:   "read an unknown project",
		covers: []string{"GetProject"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			_, err := tg.svc.GetProject(tg.ctx, "nosuchproject")
			return nil, err
		},
	},
	{
		name:   "a duplicate project key is a conflict",
		covers: []string{"CreateProject"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			_, err := tg.svc.CreateProject(tg.ctx, core.CreateProjectInput{Key: p.Key, Name: "second"})
			return nil, err
		},
	},
	{
		name:   "a malformed project key is rejected",
		covers: []string{"CreateProject"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			_, err := tg.svc.CreateProject(tg.ctx, core.CreateProjectInput{Key: "Not A Key!", Name: "x"})
			return nil, err
		},
	},
	{
		name:   "update a project",
		covers: []string{"CreateProject", "UpdateProject", "GetProject"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			name := "renamed"
			updated, err := tg.svc.UpdateProject(tg.ctx, p.Key, core.UpdateProjectInput{Name: &name})
			if err != nil {
				return nil, err
			}
			got, err := tg.svc.GetProject(tg.ctx, p.Key)
			mustf(t, tg, err, "reading the project back")
			return sig{"name": updated.Name, "persisted": got.Name}, nil
		},
	},
	{
		name:   "archiving hides a project from the default listing",
		covers: []string{"CreateProject", "ArchiveProject", "ListProjects"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			if err := tg.svc.ArchiveProject(tg.ctx, p.Key); err != nil {
				return nil, err
			}
			live, _, err := tg.svc.ListProjects(tg.ctx, core.ProjectFilter{Keys: []string{p.Key}})
			mustf(t, tg, err, "listing live projects")
			all, _, err := tg.svc.ListProjects(tg.ctx, core.ProjectFilter{Keys: []string{p.Key}, IncludeArchived: true})
			mustf(t, tg, err, "listing archived projects too")
			return sig{"live": len(live), "including_archived": len(all)}, nil
		},
	},
	{
		name:   "delete then read a project is not found",
		covers: []string{"CreateProject", "DeleteProject", "GetProject"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			mustf(t, tg, tg.svc.DeleteProject(tg.ctx, p.Key), "deleting the project")
			_, err := tg.svc.GetProject(tg.ctx, p.Key)
			return nil, err
		},
	},
	{
		name:   "field definitions round-trip",
		covers: []string{"CreateProject", "PutFieldDef", "ListFieldDefs"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			def, err := tg.svc.PutFieldDef(tg.ctx, p.Key, core.FieldDefInput{
				Key: "severity", Label: "Severity", Type: core.FieldEnum, EnumOptions: []string{"low", "high"},
			})
			if err != nil {
				return nil, err
			}
			defs, err := tg.svc.ListFieldDefs(tg.ctx, p.Key)
			mustf(t, tg, err, "listing field definitions")
			return sig{"key": def.Key, "count": len(defs), "options": defs[0].EnumOptions}, nil
		},
	},
	{
		name:   "delete a field definition",
		covers: []string{"CreateProject", "PutFieldDef", "DeleteFieldDef", "ListFieldDefs"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			_, err := tg.svc.PutFieldDef(tg.ctx, p.Key, core.FieldDefInput{Key: "owner", Label: "Owner", Type: core.FieldString})
			mustf(t, tg, err, "defining the field")
			if err := tg.svc.DeleteFieldDef(tg.ctx, p.Key, "owner"); err != nil {
				return nil, err
			}
			defs, err := tg.svc.ListFieldDefs(tg.ctx, p.Key)
			mustf(t, tg, err, "listing field definitions")
			return sig{"count": len(defs)}, nil
		},
	},
	{
		name:   "an enum field with no options is rejected",
		covers: []string{"CreateProject", "PutFieldDef"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			_, err := tg.svc.PutFieldDef(tg.ctx, p.Key, core.FieldDefInput{Key: "sev", Label: "Sev", Type: core.FieldEnum})
			return nil, err
		},
	},
	{
		name:   "workflows round-trip",
		covers: []string{"PutWorkflow", "GetWorkflow", "ListWorkflows"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			key := "w" + randSuffix()
			put, err := tg.svc.PutWorkflow(tg.ctx, matrixWorkflow(key))
			if err != nil {
				return nil, err
			}
			got, err := tg.svc.GetWorkflow(tg.ctx, key)
			mustf(t, tg, err, "reading the workflow back")
			all, err := tg.svc.ListWorkflows(tg.ctx)
			mustf(t, tg, err, "listing workflows")
			return sig{
				"named":   put.Name == "Matrix "+key,
				"initial": got.Definition.Initial,
				"states":  len(got.Definition.States),
				"listed":  len(all),
			}, nil
		},
	},
	{
		name:   "delete a workflow",
		covers: []string{"PutWorkflow", "DeleteWorkflow", "GetWorkflow"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			key := "w" + randSuffix()
			_, err := tg.svc.PutWorkflow(tg.ctx, matrixWorkflow(key))
			mustf(t, tg, err, "defining the workflow")
			if err := tg.svc.DeleteWorkflow(tg.ctx, key); err != nil {
				return nil, err
			}
			_, err = tg.svc.GetWorkflow(tg.ctx, key)
			return nil, err
		},
	},
	{
		name:   "a workflow whose initial state is undefined is rejected",
		covers: []string{"PutWorkflow"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			in := matrixWorkflow("w" + randSuffix())
			in.Definition.Initial = "nowhere"
			_, err := tg.svc.PutWorkflow(tg.ctx, in)
			return nil, err
		},
	},
	{
		name:   "restore a soft-deleted task",
		covers: []string{"CreateProject", "CreateTask", "DeleteTask", "RestoreTask", "GetTask"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "t"})
			mustf(t, tg, err, "creating the task")
			mustf(t, tg, tg.svc.DeleteTask(tg.ctx, core.TaskRef{ID: task.ID}, core.DeleteTaskInput{}), "deleting the task")
			restored, err := tg.svc.RestoreTask(tg.ctx, core.TaskRef{ID: task.ID})
			if err != nil {
				return nil, err
			}
			after, err := tg.svc.GetTask(tg.ctx, core.TaskRef{ID: task.ID})
			mustf(t, tg, err, "reading the restored task")
			return sig{"title": restored.Title, "readable_again": after.Title}, nil
		},
	},
	{
		name:   "restoring a task that was never deleted",
		covers: []string{"CreateProject", "CreateTask", "RestoreTask"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "t"})
			mustf(t, tg, err, "creating the task")
			_, err = tg.svc.RestoreTask(tg.ctx, core.TaskRef{ID: task.ID})
			return nil, err
		},
	},
	{
		name:   "a task tree reaches its children",
		covers: []string{"CreateProject", "CreateTask", "TaskTree"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			parent, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "parent"})
			mustf(t, tg, err, "creating the parent")
			_, err = tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{
				ProjectRef: p.Key, Title: "child", ParentRef: parent.ID,
			})
			mustf(t, tg, err, "creating the child")
			tree, err := tg.svc.TaskTree(tg.ctx, core.TaskRef{ID: parent.ID}, 3)
			if err != nil {
				return nil, err
			}
			titles := make([]string, len(tree))
			for i, tsk := range tree {
				titles[i] = tsk.Title
			}
			return sig{"titles": titles}, nil
		},
	},
	{
		name:   "remove a dependency",
		covers: []string{"CreateProject", "CreateTask", "AddDependency", "RemoveDependency", "ListDependencies"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			a, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "a"})
			mustf(t, tg, err, "creating task a")
			b, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "b"})
			mustf(t, tg, err, "creating task b")
			mustf(t, tg, tg.svc.AddDependency(tg.ctx, core.TaskRef{ID: a.ID}, core.TaskRef{ID: b.ID}), "adding the dependency")
			if err := tg.svc.RemoveDependency(tg.ctx, core.TaskRef{ID: a.ID}, core.TaskRef{ID: b.ID}); err != nil {
				return nil, err
			}
			deps, err := tg.svc.ListDependencies(tg.ctx, core.TaskRef{ID: a.ID})
			mustf(t, tg, err, "listing dependencies")
			return sig{"dep_count": len(deps)}, nil
		},
	},
	{
		name:   "a dependency on an unknown task",
		covers: []string{"CreateProject", "CreateTask", "AddDependency"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			a, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "a"})
			mustf(t, tg, err, "creating task a")
			err = tg.svc.AddDependency(tg.ctx, core.TaskRef{ID: a.ID}, core.TaskRef{ID: "nosuchtask00000000"})
			return nil, err
		},
	},
	{
		name:   "remove a tag",
		covers: []string{"CreateProject", "CreateTask", "AddTag", "RemoveTag", "GetTask"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "t"})
			mustf(t, tg, err, "creating the task")
			mustf(t, tg, tg.svc.AddTag(tg.ctx, core.TaskRef{ID: task.ID}, "urgent"), "adding the tag")
			if err := tg.svc.RemoveTag(tg.ctx, core.TaskRef{ID: task.ID}, "urgent"); err != nil {
				return nil, err
			}
			after, err := tg.svc.GetTask(tg.ctx, core.TaskRef{ID: task.ID})
			mustf(t, tg, err, "reading the task back")
			return sig{"tags": after.Tags}, nil
		},
	},
	{
		name:   "tags are listed for the tenant",
		covers: []string{"CreateProject", "CreateTask", "AddTag", "ListTags"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "t"})
			mustf(t, tg, err, "creating the task")
			mustf(t, tg, tg.svc.AddTag(tg.ctx, core.TaskRef{ID: task.ID}, "release"), "adding the tag")
			tags, err := tg.svc.ListTags(tg.ctx)
			if err != nil {
				return nil, err
			}
			names := make([]string, 0, len(tags))
			for _, tag := range tags {
				names = append(names, tag.Name)
			}
			sort.Strings(names)
			return sig{"names": names}, nil
		},
	},
	{
		name:   "edit a comment",
		covers: []string{"CreateProject", "CreateTask", "AddComment", "EditComment", "ListComments"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "t"})
			mustf(t, tg, err, "creating the task")
			comment, err := tg.svc.AddComment(tg.ctx, core.TaskRef{ID: task.ID}, "first")
			mustf(t, tg, err, "adding the comment")
			edited, err := tg.svc.EditComment(tg.ctx, comment.ID, "second")
			if err != nil {
				return nil, err
			}
			listed, err := tg.svc.ListComments(tg.ctx, core.TaskRef{ID: task.ID})
			mustf(t, tg, err, "listing comments")
			return sig{"body": edited.Body, "persisted": listed[0].Body}, nil
		},
	},
	{
		name:   "delete a comment",
		covers: []string{"CreateProject", "CreateTask", "AddComment", "DeleteComment", "ListComments"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "t"})
			mustf(t, tg, err, "creating the task")
			comment, err := tg.svc.AddComment(tg.ctx, core.TaskRef{ID: task.ID}, "regrettable")
			mustf(t, tg, err, "adding the comment")
			if err := tg.svc.DeleteComment(tg.ctx, comment.ID); err != nil {
				return nil, err
			}
			listed, err := tg.svc.ListComments(tg.ctx, core.TaskRef{ID: task.ID})
			mustf(t, tg, err, "listing comments")
			return sig{"count": len(listed)}, nil
		},
	},
	{
		name:   "editing an unknown comment",
		covers: []string{"EditComment"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			_, err := tg.svc.EditComment(tg.ctx, "nosuchcomment00000", "text")
			return nil, err
		},
	},
	{
		name:   "artifacts round-trip",
		covers: []string{"CreateProject", "CreateTask", "PutArtifact", "ListArtifacts"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "t"})
			mustf(t, tg, err, "creating the task")
			art, err := tg.svc.PutArtifact(tg.ctx, core.TaskRef{ID: task.ID}, core.ArtifactInput{
				Kind: core.ArtifactResult, Name: "summary", Payload: map[string]any{"exit": float64(0)},
			})
			if err != nil {
				return nil, err
			}
			listed, err := tg.svc.ListArtifacts(tg.ctx, core.TaskRef{ID: task.ID})
			mustf(t, tg, err, "listing artifacts")
			return sig{"kind": art.Kind, "count": len(listed), "payload": listed[0].Payload}, nil
		},
	},
	{
		name:   "an artifact with no kind is rejected",
		covers: []string{"CreateProject", "CreateTask", "PutArtifact"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "t"})
			mustf(t, tg, err, "creating the task")
			_, err = tg.svc.PutArtifact(tg.ctx, core.TaskRef{ID: task.ID}, core.ArtifactInput{Name: "nameless"})
			return nil, err
		},
	},
	{
		name:   "sweeping reclaims an expired lease",
		covers: []string{"CreateProject", "CreateTask", "ClaimTask", "SweepLeases", "GetTask"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "t"})
			mustf(t, tg, err, "creating the task")
			_, err = tg.svc.ClaimTask(tg.ctx, core.TaskRef{ID: task.ID}, core.ClaimInput{TTL: core.Duration(time.Minute)})
			mustf(t, tg, err, "claiming the task")
			h.clk.Advance(2 * time.Minute)
			swept, err := tg.svc.SweepLeases(tg.ctx, 10)
			if err != nil {
				return nil, err
			}
			after, err := tg.svc.GetTask(tg.ctx, core.TaskRef{ID: task.ID})
			mustf(t, tg, err, "reading the task back")
			return sig{"swept": swept, "claimed_by": after.ClaimedByActorID}, nil
		},
	},
	{
		name:   "the audit log records a task creation",
		covers: []string{"CreateProject", "CreateTask", "ListAudit"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "t"})
			mustf(t, tg, err, "creating the task")
			entries, _, err := tg.svc.ListAudit(tg.ctx, core.AuditFilter{SubjectType: "task", SubjectID: task.ID})
			if err != nil {
				return nil, err
			}
			actions := make([]string, len(entries))
			for i, e := range entries {
				actions[i] = e.Action
			}
			return sig{"actions": actions}, nil
		},
	},
	{
		name:   "the retention policy round-trips",
		covers: []string{"GetRetention", "PutRetention"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			before, err := tg.svc.GetRetention(tg.ctx)
			if err != nil {
				return nil, err
			}
			next := *before
			next.Events = core.Duration(72 * time.Hour)
			put, err := tg.svc.PutRetention(tg.ctx, next)
			if err != nil {
				return nil, err
			}
			after, err := tg.svc.GetRetention(tg.ctx)
			mustf(t, tg, err, "reading the policy back")
			return sig{
				"default_events": before.Events,
				"put_events":     put.Events,
				"read_back":      after.Events,
				"audit":          after.AuditEntries,
			}, nil
		},
	},
	{
		name:   "pruning in dry run removes nothing",
		covers: []string{"Prune"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			res, err := tg.svc.Prune(tg.ctx, core.PruneInput{DryRun: true, Limit: 10})
			if err != nil {
				return nil, err
			}
			return sig{"dry_run": res.DryRun, "events": res.Events, "audit": res.AuditEntries}, nil
		},
	},
	{
		name:   "who the credential speaks for",
		covers: []string{"WhoAmI"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			actor, err := tg.svc.WhoAmI(tg.ctx)
			if err != nil {
				return nil, err
			}
			// Identity itself is not comparable here and a divergence in it
			// would be no finding: the direct target carries a user actor in
			// its context, while the remote target presents the admin's bearer
			// token, so the server answers with the agent that token names.
			// What must agree is the tenant the credential lands in and the
			// authority it carries.
			return sig{
				"tenant": actor.TenantID == h.tenantID,
				"named":  actor.Handle != "",
				"scopes": actor.Scopes,
			}, nil
		},
	},
	{
		name:   "resolve an actor by identifier",
		covers: []string{"GetActor"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			actor, err := tg.svc.GetActor(tg.ctx, h.adminActor.ID)
			if err != nil {
				return nil, err
			}
			return sig{"handle": actor.Handle, "kind": actor.Kind}, nil
		},
	},
	{
		name:   "resolve an unknown actor",
		covers: []string{"GetActor"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			_, err := tg.svc.GetActor(tg.ctx, "nosuchactor0000000")
			return nil, err
		},
	},
	{
		name:   "users round-trip",
		covers: []string{"CreateUser", "GetUser", "ListUsers"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			created, err := tg.svc.CreateUser(tg.ctx, core.CreateUserInput{
				Email: "one@example.test", Handle: "one", DisplayName: "One", Role: core.RoleMember,
			})
			if err != nil {
				return nil, err
			}
			got, err := tg.svc.GetUser(tg.ctx, created.ID)
			mustf(t, tg, err, "reading the user back")
			users, _, err := tg.svc.ListUsers(tg.ctx, core.Page{Limit: 50})
			mustf(t, tg, err, "listing users")
			return sig{"email": got.Email, "display_name": got.DisplayName, "count": len(users)}, nil
		},
	},
	{
		name:   "update then delete a user",
		covers: []string{"CreateUser", "UpdateUser", "DeleteUser", "GetUser"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			created, err := tg.svc.CreateUser(tg.ctx, core.CreateUserInput{
				Email: "two@example.test", Handle: "two", Role: core.RoleMember,
			})
			mustf(t, tg, err, "creating the user")
			name := "Renamed"
			updated, err := tg.svc.UpdateUser(tg.ctx, created.ID, core.UpdateUserInput{DisplayName: &name})
			if err != nil {
				return nil, err
			}
			if err := tg.svc.DeleteUser(tg.ctx, created.ID); err != nil {
				return nil, err
			}
			_, err = tg.svc.GetUser(tg.ctx, created.ID)
			return sig{"display_name": updated.DisplayName, "after_delete": core.KindOf(err)}, nil
		},
	},
	{
		name:   "a duplicate email is a conflict",
		covers: []string{"CreateUser"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			_, err := tg.svc.CreateUser(tg.ctx, core.CreateUserInput{Email: "dup@example.test", Handle: "dupa"})
			mustf(t, tg, err, "creating the first user")
			_, err = tg.svc.CreateUser(tg.ctx, core.CreateUserInput{Email: "dup@example.test", Handle: "dupb"})
			return nil, err
		},
	},
	{
		name:   "a short password is rejected",
		covers: []string{"CreateUser"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			_, err := tg.svc.CreateUser(tg.ctx, core.CreateUserInput{
				Email: "short@example.test", Handle: "short", Password: "short",
			})
			return nil, err
		},
	},
	{
		name:   "login with the right password",
		covers: []string{"CreateUser", "Login"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			user, err := tg.svc.CreateUser(tg.ctx, core.CreateUserInput{
				Email: "login@example.test", Handle: "login", Password: "correct horse battery",
			})
			mustf(t, tg, err, "creating the user")
			session, err := tg.svc.Login(tg.ctx, "login@example.test", "correct horse battery")
			if err != nil {
				return nil, err
			}
			return sig{"actor_matches": session.ActorID == user.ID, "has_token": session.Token != ""}, nil
		},
	},
	{
		name:   "login with the wrong password",
		covers: []string{"CreateUser", "Login"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			_, err := tg.svc.CreateUser(tg.ctx, core.CreateUserInput{
				Email: "wrong@example.test", Handle: "wrong", Password: "correct horse battery",
			})
			mustf(t, tg, err, "creating the user")
			_, err = tg.svc.Login(tg.ctx, "wrong@example.test", "not the password")
			return nil, err
		},
	},
	{
		name:   "login for an address nobody holds",
		covers: []string{"Login"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			_, err := tg.svc.Login(tg.ctx, "absent@example.test", "correct horse battery")
			return nil, err
		},
	},
	{
		name:   "logging out without a session",
		covers: []string{"Logout"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			return nil, tg.svc.Logout(tg.ctx)
		},
	},
	{
		name:   "tokens round-trip",
		covers: []string{"CreateToken", "ListTokens"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			issued, err := tg.svc.CreateToken(tg.ctx, core.CreateTokenInput{
				Name: "worker", ActorID: h.adminActor.ID, Scopes: []core.Scope{core.ScopeTaskRead},
			})
			if err != nil {
				return nil, err
			}
			tokens, err := tg.svc.ListTokens(tg.ctx, h.adminActor.ID)
			mustf(t, tg, err, "listing tokens")
			names := make([]string, 0, len(tokens))
			for _, tok := range tokens {
				names = append(names, tok.Name)
			}
			sort.Strings(names)
			return sig{"has_secret": issued.Token != "", "scopes": issued.Scopes, "names": names}, nil
		},
	},
	{
		name:   "revoking a token stops it authenticating",
		covers: []string{"CreateToken", "RevokeToken", "ListTokens"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			issued, err := tg.svc.CreateToken(tg.ctx, core.CreateTokenInput{
				Name: "doomed", ActorID: h.adminActor.ID, Scopes: []core.Scope{core.ScopeTaskRead},
			})
			mustf(t, tg, err, "minting the token")
			if err := tg.svc.RevokeToken(tg.ctx, issued.ID); err != nil {
				return nil, err
			}
			tokens, err := tg.svc.ListTokens(tg.ctx, h.adminActor.ID)
			mustf(t, tg, err, "listing tokens")
			revoked := 0
			for _, tok := range tokens {
				if tok.RevokedAt != nil {
					revoked++
				}
			}
			return sig{"revoked": revoked}, nil
		},
	},
	{
		name:   "a token with no scopes is rejected",
		covers: []string{"CreateToken"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			_, err := tg.svc.CreateToken(tg.ctx, core.CreateTokenInput{Name: "scopeless", ActorID: h.adminActor.ID})
			return nil, err
		},
	},
	{
		name:   "revoking an unknown token",
		covers: []string{"RevokeToken"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			return nil, tg.svc.RevokeToken(tg.ctx, "nosuchtoken0000000")
		},
	},
	{
		name:   "enrol and list an ssh key",
		covers: []string{"EnrolSSHKey", "ListSSHKeys"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			key, err := tg.svc.EnrolSSHKey(tg.ctx, core.EnrolSSHKeyInput{
				ActorID: h.adminActor.ID, PublicKey: matrixPublicKey, Label: "laptop",
			})
			if err != nil {
				return nil, err
			}
			keys, err := tg.svc.ListSSHKeys(tg.ctx, h.adminActor.ID)
			mustf(t, tg, err, "listing ssh keys")
			return sig{
				"fingerprint": key.Fingerprint,
				"label":       key.Label,
				"stored":      keys[0].PublicKey,
				"active":      keys[0].Active(),
				"count":       len(keys),
			}, nil
		},
	},
	{
		name:   "revoke an ssh key",
		covers: []string{"EnrolSSHKey", "RevokeSSHKey", "ListSSHKeys"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			key, err := tg.svc.EnrolSSHKey(tg.ctx, core.EnrolSSHKeyInput{
				ActorID: h.adminActor.ID, PublicKey: matrixPublicKey,
			})
			mustf(t, tg, err, "enrolling the key")
			if err := tg.svc.RevokeSSHKey(tg.ctx, key.ID); err != nil {
				return nil, err
			}
			keys, err := tg.svc.ListSSHKeys(tg.ctx, h.adminActor.ID)
			mustf(t, tg, err, "listing ssh keys")
			// A revoked key stays listed: an operator asking why a key stopped
			// working needs to see it, not find it missing.
			return sig{"count": len(keys), "active": keys[0].Active()}, nil
		},
	},
	{
		name:   "enrolling the same key twice is a conflict",
		covers: []string{"EnrolSSHKey"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			_, err := tg.svc.EnrolSSHKey(tg.ctx, core.EnrolSSHKeyInput{
				ActorID: h.adminActor.ID, PublicKey: matrixPublicKey,
			})
			mustf(t, tg, err, "the first enrolment")
			_, err = tg.svc.EnrolSSHKey(tg.ctx, core.EnrolSSHKeyInput{
				ActorID: h.adminActor.ID, PublicKey: matrixPublicKey, Label: "same key again",
			})
			return nil, err
		},
	},
	{
		name:   "enrolling something that is not a public key",
		covers: []string{"EnrolSSHKey"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			_, err := tg.svc.EnrolSSHKey(tg.ctx, core.EnrolSSHKeyInput{
				ActorID: h.adminActor.ID, PublicKey: "-----BEGIN OPENSSH PRIVATE KEY-----",
			})
			return nil, err
		},
	},
	{
		name:   "revoking an unknown ssh key",
		covers: []string{"RevokeSSHKey"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			return nil, tg.svc.RevokeSSHKey(tg.ctx, "nosuchsshkey000000")
		},
	},
	{
		name:   "listing the live connections this server holds",
		covers: []string{"ListConnections"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			list, err := tg.svc.ListConnections(tg.ctx)
			if err != nil {
				return nil, err
			}
			// ServerID names the process that answered and is necessarily
			// different between two harnesses, so only the shape is compared.
			return sig{
				"named_server": list.ServerID != "",
				"connections":  len(list.Connections),
				"events":       list.Counts.Events,
				"ssh":          list.Counts.SSH,
				"tenant":       list.Counts.Tenant,
			}, nil
		},
	},
	{
		name:   "ending an unknown connection",
		covers: []string{"EndConnection"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			return nil, tg.svc.EndConnection(tg.ctx, "nosuchconnection00")
		},
	},
	{
		name:   "ending a connection with no identifier",
		covers: []string{"EndConnection"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			return nil, tg.svc.EndConnection(tg.ctx, "  ")
		},
	},
	{
		name:   "webhooks round-trip",
		covers: []string{"PutWebhook", "ListWebhooks", "DeleteWebhook"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			endpoint, err := tg.svc.PutWebhook(tg.ctx, core.WebhookInput{
				URL: closedEndpoint, Secret: "shhh", EventTypes: []string{"task.*"}, Active: true,
			})
			if err != nil {
				return nil, err
			}
			listed, err := tg.svc.ListWebhooks(tg.ctx)
			mustf(t, tg, err, "listing webhooks")
			if err := tg.svc.DeleteWebhook(tg.ctx, endpoint.ID); err != nil {
				return nil, err
			}
			after, err := tg.svc.ListWebhooks(tg.ctx)
			mustf(t, tg, err, "listing webhooks after the delete")
			// The secret is never returned on either transport.
			return sig{
				"url":         endpoint.URL == closedEndpoint,
				"event_types": listed[0].EventTypes,
				"active":      listed[0].Active,
				"secret":      listed[0].Secret,
				"before":      len(listed),
				"after":       len(after),
			}, nil
		},
	},
	{
		name:   "a webhook without a url is rejected",
		covers: []string{"PutWebhook"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			_, err := tg.svc.PutWebhook(tg.ctx, core.WebhookInput{Active: true})
			return nil, err
		},
	},
	{
		name:   "listing webhook deliveries",
		covers: []string{"CreateProject", "CreateTask", "PutWebhook", "ListDeliveries"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			endpoint, err := tg.svc.PutWebhook(tg.ctx, core.WebhookInput{
				URL: closedEndpoint, EventTypes: []string{"*"}, Active: true,
			})
			mustf(t, tg, err, "registering the endpoint")
			p := h.newProject(t, tg)
			_, err = tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "t"})
			mustf(t, tg, err, "creating a task to deliver")
			deliveries, _, err := tg.svc.ListDeliveries(tg.ctx, core.DeliveryFilter{EndpointID: endpoint.ID})
			if err != nil {
				return nil, err
			}
			statuses := make([]string, 0, len(deliveries))
			for _, d := range deliveries {
				statuses = append(statuses, string(d.Status))
			}
			sort.Strings(statuses)
			return sig{"count": len(deliveries), "statuses": statuses}, nil
		},
	},
	{
		name:   "redelivering an unknown delivery",
		covers: []string{"RedeliverWebhook"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			return nil, tg.svc.RedeliverWebhook(tg.ctx, "nosuchdelivery0000")
		},
	},
	{
		name:   "an export carries the same record kinds",
		covers: []string{"CreateProject", "CreateTask", "AddComment", "ExportTo"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "exported"})
			mustf(t, tg, err, "creating the task")
			_, err = tg.svc.AddComment(tg.ctx, core.TaskRef{ID: task.ID}, "worth keeping")
			mustf(t, tg, err, "commenting on the task")

			var buf bytes.Buffer
			if err := tg.svc.ExportTo(tg.ctx, core.ExportInput{
				ProjectRefs: []string{p.Key}, IncludeComments: true,
			}, &buf); err != nil {
				return nil, err
			}
			return sig{"kinds": snapshotKinds(t, tg, buf.Bytes())}, nil
		},
	},
	{
		name:   "a snapshot imports back into the same tenant",
		covers: []string{"CreateProject", "CreateTask", "ExportTo", "ImportFrom", "ListTasks"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			_, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "exported"})
			mustf(t, tg, err, "creating the task")

			var buf bytes.Buffer
			mustf(t, tg, tg.svc.ExportTo(tg.ctx, core.ExportInput{ProjectRefs: []string{p.Key}}, &buf), "exporting")

			res, err := tg.svc.ImportFrom(tg.ctx, bytes.NewReader(buf.Bytes()), core.ImportInput{Mode: core.ImportMerge})
			if err != nil {
				return nil, err
			}
			page, err := tg.svc.ListTasks(tg.ctx, core.TaskFilter{ProjectKeys: []string{p.Key}})
			mustf(t, tg, err, "listing the imported tasks")
			return sig{
				"created": res.Created,
				"updated": res.Updated,
				"tasks":   len(page.Tasks),
			}, nil
		},
	},
	{
		name:   "an import with no mode is rejected",
		covers: []string{"ImportFrom"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			_, err := tg.svc.ImportFrom(tg.ctx, bytes.NewReader(nil), core.ImportInput{})
			return nil, err
		},
	},
	{
		name:   "a bundle exports and previews an import",
		covers: []string{"PutWorkflow", "ExportBundle", "ImportBundle"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			key := "w" + randSuffix()
			_, err := tg.svc.PutWorkflow(tg.ctx, matrixWorkflow(key))
			mustf(t, tg, err, "defining the workflow")

			var buf bytes.Buffer
			if err := tg.svc.ExportBundle(tg.ctx, core.BundleExportInput{
				Name: "matrix", Kinds: []core.ComponentKind{core.ComponentWorkflow}, WorkflowKeys: []string{key},
			}, &buf); err != nil {
				return nil, err
			}
			res, err := tg.svc.ImportBundle(tg.ctx, bytes.NewReader(buf.Bytes()), core.BundleImportInput{
				OnCollision: core.CollisionSkip, Preview: true,
			})
			if err != nil {
				return nil, err
			}
			actions := make([]string, 0, len(res.Outcomes))
			for _, o := range res.Outcomes {
				actions = append(actions, string(o.Action))
			}
			sort.Strings(actions)
			return sig{
				"bundle":  res.BundleName,
				"version": res.BundleVersion,
				"preview": res.Preview,
				"actions": actions,
			}, nil
		},
	},
	{
		name:   "a bundle import with no collision policy is rejected",
		covers: []string{"ImportBundle"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			_, err := tg.svc.ImportBundle(tg.ctx, bytes.NewReader(nil), core.BundleImportInput{})
			return nil, err
		},
	},
	{
		name:   "sync sources round-trip",
		covers: []string{"PutSyncSource", "ListSyncSources", "DeleteSyncSource"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			source, err := tg.svc.PutSyncSource(tg.ctx, core.SyncSourceInput{
				System: core.SystemGeneric, Name: "nightly",
			})
			if err != nil {
				return nil, err
			}
			listed, err := tg.svc.ListSyncSources(tg.ctx)
			mustf(t, tg, err, "listing sync sources")
			if err := tg.svc.DeleteSyncSource(tg.ctx, source.ID); err != nil {
				return nil, err
			}
			after, err := tg.svc.ListSyncSources(tg.ctx)
			mustf(t, tg, err, "listing sync sources after the delete")
			return sig{
				"system": source.System,
				"name":   listed[0].Name,
				"before": len(listed),
				"after":  len(after),
			}, nil
		},
	},
	{
		name:   "an unrecognised external system is rejected",
		covers: []string{"PutSyncSource"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			_, err := tg.svc.PutSyncSource(tg.ctx, core.SyncSourceInput{System: "trello", Name: "nope"})
			return nil, err
		},
	},
	{
		name:   "running an unknown sync source",
		covers: []string{"RunSync"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			_, err := tg.svc.RunSync(tg.ctx, core.RunSyncInput{SourceID: "nosuchsource000000"})
			return nil, err
		},
	},
	{
		name:   "the caller's own tenant round-trips",
		covers: []string{"GetTenant", "UpdateTenant", "ListTenants"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			own, err := tg.svc.GetTenant(tg.ctx, h.tenantID)
			if err != nil {
				return nil, err
			}
			name := "Renamed Tenant"
			updated, err := tg.svc.UpdateTenant(tg.ctx, own.Key, core.UpdateTenantInput{Name: &name})
			if err != nil {
				return nil, err
			}
			tenants, _, err := tg.svc.ListTenants(tg.ctx, core.Page{Limit: 50})
			mustf(t, tg, err, "listing tenants")
			return sig{
				"own":     own.ID == h.tenantID,
				"key":     own.Key,
				"updated": updated.Name,
				"listed":  len(tenants),
			}, nil
		},
	},
	{
		// core.Service lets an empty reference mean the caller's own tenant.
		// service.Local has always honoured it; client.Client used to
		// interpolate it into /api/v1/tenants/{ref}, where an empty segment
		// addresses no route, so the convention worked on one transport only.
		// Three web handlers rely on it, and they worked purely because the
		// web UI is handed a *service.Local today.
		name:   "an empty reference means the caller's own tenant on both transports",
		covers: []string{"GetTenant"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			own, err := tg.svc.GetTenant(tg.ctx, "")
			if err != nil {
				return nil, err
			}
			named, err := tg.svc.GetTenant(tg.ctx, h.tenantID)
			if err != nil {
				return nil, err
			}
			if own.ID != named.ID {
				t.Fatalf("empty reference gave tenant %q, naming it gave %q", own.ID, named.ID)
			}
			// Identifiers are minted per target, so the comparable fact is
			// that both spellings reached the same tenant, which the check
			// above establishes on each transport independently.
			return sig{"key": own.Key, "same": own.ID == named.ID}, nil
		},
	},
	{
		name:   "a created tenant is not visible to its creator",
		covers: []string{"CreateTenant", "GetTenant"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			key := "t" + randSuffix()
			created, err := tg.svc.CreateTenant(tg.ctx, core.CreateTenantInput{Key: key, Name: "Matrix Tenant"})
			if err != nil {
				return nil, err
			}
			// Reads are bound to the caller's own tenant, so the tenant just
			// created is absent rather than refused.
			_, err = tg.svc.GetTenant(tg.ctx, key)
			return sig{
				"name":         created.Name,
				"key":          created.Key == key,
				"other_tenant": core.KindOf(err),
			}, nil
		},
	},
	{
		name:   "deleting the caller's own tenant",
		covers: []string{"DeleteTenant", "GetTenant"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			if err := tg.svc.DeleteTenant(tg.ctx, h.tenantID); err != nil {
				return nil, err
			}
			_, err := tg.svc.GetTenant(tg.ctx, h.tenantID)
			return sig{"after_delete": core.KindOf(err)}, nil
		},
	},
	{
		name:   "reading an unknown tenant",
		covers: []string{"GetTenant"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			_, err := tg.svc.GetTenant(tg.ctx, "nosuchtenant")
			return nil, err
		},
	},
	{
		name:   "domains round-trip",
		covers: []string{"AddDomain", "ListDomains", "ResolveDomain", "RemoveDomain"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			host := "h" + randSuffix() + ".example.test"
			added, err := tg.svc.AddDomain(tg.ctx, core.AddDomainInput{Hostname: host})
			if err != nil {
				return nil, err
			}
			listed, err := tg.svc.ListDomains(tg.ctx)
			mustf(t, tg, err, "listing domains")
			resolved, err := tg.svc.ResolveDomain(tg.ctx, host)
			if err != nil {
				return nil, err
			}
			if err := tg.svc.RemoveDomain(tg.ctx, host); err != nil {
				return nil, err
			}
			after, err := tg.svc.ListDomains(tg.ctx)
			mustf(t, tg, err, "listing domains after the removal")
			return sig{
				"hostname":     added.Hostname == host,
				"listed":       len(listed),
				"resolves_own": resolved.ID == h.tenantID,
				"after":        len(after),
			}, nil
		},
	},
	{
		name:   "resolving a hostname nobody claimed",
		covers: []string{"ResolveDomain"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			_, err := tg.svc.ResolveDomain(tg.ctx, "unclaimed.example.test")
			return nil, err
		},
	},
	{
		name:   "members round-trip",
		covers: []string{"CreateUser", "AddMember", "ListMembers", "RemoveMember"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			user, err := tg.svc.CreateUser(tg.ctx, core.CreateUserInput{
				Email: "member@example.test", Handle: "member", Role: core.RoleViewer,
			})
			mustf(t, tg, err, "creating the user")
			before, err := tg.svc.ListMembers(tg.ctx)
			mustf(t, tg, err, "listing members")
			// Creating a user of this tenant already enrols them, so adding
			// them again is the conflict, and that is the interesting outcome.
			_, addErr := tg.svc.AddMember(tg.ctx, user.ID, core.RoleMember)
			if err := tg.svc.RemoveMember(tg.ctx, user.ID); err != nil {
				return nil, err
			}
			after, err := tg.svc.ListMembers(tg.ctx)
			mustf(t, tg, err, "listing members after the removal")
			return sig{"before": len(before), "re_add": core.KindOf(addErr), "after": len(after)}, nil
		},
	},
	{
		name:   "adding an actor nobody created",
		covers: []string{"AddMember"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			_, err := tg.svc.AddMember(tg.ctx, "nosuchactor0000000", core.RoleMember)
			return nil, err
		},
	},
	{
		name:   "a viewer is forbidden from administering tenants",
		covers: []string{"ListConnections"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			svc, ctx := h.scoped(tg, core.ScopeTaskRead)
			_, err := svc.ListConnections(ctx)
			return nil, err
		},
	},
	{
		name:   "read statistics over a window",
		covers: []string{"Stats"},
		run: func(t *testing.T, tg target, h *matrixHarness) (sig, error) {
			p := h.newProject(t, tg)
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{
				ProjectRef: p.Key, Title: "counted",
			})
			if err != nil {
				return nil, err
			}
			for _, to := range []string{"doing", "done"} {
				if _, err := tg.svc.TransitionTask(tg.ctx, core.TaskRef{ID: task.ID},
					core.TransitionInput{To: to}); err != nil {
					return nil, err
				}
			}
			stats, err := tg.svc.Stats(tg.ctx, core.StatsInput{ProjectRef: p.Key})
			if err != nil {
				return nil, err
			}
			return sig{
				"completed": stats.Completed,
				"created":   stats.Created,
				"actors":    len(stats.TopActors),
				"moved":     stats.TopActors[0].Moved,
				"measure":   stats.LeaderboardMeasure,
				"open":      len(stats.Oldest),
			}, nil
		},
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
