// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/workflows"
)

func TestListWorkflowTemplatesListsTheBuiltIns(t *testing.T) {
	l, _, ctx := withDefaults(t)
	got, err := l.ListWorkflowTemplates(ctx)
	if err != nil {
		t.Fatalf("ListWorkflowTemplates: %v", err)
	}
	want, _ := workflows.Templates()
	if len(got) != len(want) || len(got) != 12 {
		t.Fatalf("listed %d templates, want the 12 built in", len(got))
	}
	for _, tpl := range got {
		if tpl.Key == "" || tpl.Name == "" || tpl.Purpose == "" || tpl.States == "" {
			t.Errorf("template %+v is missing a key, name, purpose or states summary", tpl)
		}
	}
}

func TestApplyWorkflowTemplateCreatesAWorkflowAProjectCanUse(t *testing.T) {
	l, scope, ctx := withDefaults(t)
	events, audits := countRows(t, l, scope)

	wf, err := l.ApplyWorkflowTemplate(ctx, "tix-dev-review", core.ApplyWorkflowTemplateInput{})
	if err != nil {
		t.Fatalf("ApplyWorkflowTemplate: %v", err)
	}
	if wf.Key != "tix-dev-review" || wf.Name != "Development with review" || wf.Builtin {
		t.Fatalf("workflow = %+v", wf)
	}
	if afterEvents, afterAudits := countRows(t, l, scope); afterEvents != events+1 || afterAudits != audits+1 {
		t.Error("applying a template did not write both an audit entry and an event")
	}
	p, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "web", Name: "Web", WorkflowKey: wf.Key})
	if err != nil {
		t.Fatalf("CreateProject on the template's workflow: %v", err)
	}
	task, err := l.CreateTask(ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "ship it"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if task.Status != "triage" {
		t.Fatalf("status = %q, want the template's initial state", task.Status)
	}

	renamed, err := l.ApplyWorkflowTemplate(ctx, "tix-dev-review", core.ApplyWorkflowTemplateInput{As: "Review"})
	if err != nil || renamed.Key != "review" {
		t.Fatalf("apply --as = %+v, %v", renamed, err)
	}
}

func TestApplyWorkflowTemplateOnATakenKey(t *testing.T) {
	l, _, ctx := withDefaults(t)
	if _, err := l.ApplyWorkflowTemplate(ctx, "tix-kanban", core.ApplyWorkflowTemplateInput{}); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if _, err := l.ApplyWorkflowTemplate(ctx, "tix-kanban", core.ApplyWorkflowTemplateInput{}); !core.IsKind(err, core.KindConflict) {
		t.Fatalf("second apply err = %v, want conflict", err)
	}
	skipped, err := l.ApplyWorkflowTemplate(ctx, "tix-kanban", core.ApplyWorkflowTemplateInput{OnCollision: core.CollisionSkip})
	if err != nil || skipped.Key != "tix-kanban" {
		t.Fatalf("skip = %+v, %v", skipped, err)
	}
	renamed, err := l.ApplyWorkflowTemplate(ctx, "tix-kanban", core.ApplyWorkflowTemplateInput{OnCollision: core.CollisionRename})
	if err != nil || renamed.Key != "tix-kanban-2" {
		t.Fatalf("rename = %+v, %v", renamed, err)
	}
	replaced, err := l.ApplyWorkflowTemplate(ctx, "tix-gtd",
		core.ApplyWorkflowTemplateInput{As: "tix-kanban", OnCollision: core.CollisionReplace})
	if err != nil || replaced.Key != "tix-kanban" || replaced.Definition.Initial != "inbox" {
		t.Fatalf("replace = %+v, %v", replaced, err)
	}
	if _, err := l.ApplyWorkflowTemplate(ctx, "tix-kanban",
		core.ApplyWorkflowTemplateInput{As: BuiltinWorkflowKey, OnCollision: core.CollisionReplace}); !core.IsKind(err, core.KindConflict) {
		t.Fatalf("replacing the builtin err = %v, want conflict", err)
	}
	if _, err := l.ApplyWorkflowTemplate(ctx, "tix-nope", core.ApplyWorkflowTemplateInput{}); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("unknown template err = %v, want not found", err)
	}
	if _, err := l.ApplyWorkflowTemplate(ctx, "tix-kanban", core.ApplyWorkflowTemplateInput{OnCollision: "merge"}); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("unknown policy err = %v, want invalid", err)
	}
}

func TestTheTemplatePrefixIsReserved(t *testing.T) {
	l, _, ctx := withDefaults(t)
	_, err := l.PutWorkflow(ctx, core.WorkflowInput{Key: "tix-mine", Definition: twoStateDefinition()})
	if !core.IsKind(err, core.KindInvalid) || core.FieldOf(err) != "key" {
		t.Fatalf("putting a tix- workflow by hand err = %v, want invalid on key", err)
	}
	if _, err := l.ApplyWorkflowTemplate(ctx, "tix-bugs", core.ApplyWorkflowTemplateInput{}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, err := l.PutWorkflow(ctx, core.WorkflowInput{Key: "tix-bugs", Name: "Bugs", Definition: func() core.WorkflowDefinition {
		tpl, _, _ := workflows.Lookup("tix-bugs")
		return tpl.Definition
	}()}); err != nil {
		t.Fatalf("redefining a workflow a template created: %v", err)
	}
}

func TestCreateProjectFromATemplateGetsItsWorkflowAndFields(t *testing.T) {
	l, _, ctx := withDefaults(t)
	p, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "deals", Name: "Deals", WorkflowTemplate: "tix-sales"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	wf, err := l.GetWorkflow(ctx, "tix-sales")
	if err != nil || wf.ID != p.WorkflowID {
		t.Fatalf("project runs on %q, want the tix-sales workflow (%+v, %v)", p.WorkflowID, wf, err)
	}
	fields, err := l.ListFieldDefs(ctx, "deals")
	if err != nil {
		t.Fatalf("ListFieldDefs: %v", err)
	}
	var keys []string
	for _, f := range fields {
		keys = append(keys, f.Key)
	}
	if len(keys) != 3 || keys[0] != "company" || keys[1] != "deal_value" || keys[2] != "expected_close" {
		t.Fatalf("fields = %v, want the template's three", keys)
	}

	second, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "deals2", Name: "More deals", WorkflowTemplate: "tix-sales"})
	if err != nil || second.WorkflowID != wf.ID {
		t.Fatalf("a second project from the template = %+v, %v; want it to reuse the workflow", second, err)
	}
	if _, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "x", Name: "X", WorkflowKey: "default",
		WorkflowTemplate: "tix-sales"}); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("naming both err = %v, want invalid", err)
	}
	if _, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "y", Name: "Y", WorkflowTemplate: "tix-nope"}); core.FieldOf(err) != "workflow_template" {
		t.Fatalf("unknown template err = %v, want it on workflow_template", err)
	}
}

// templateTask creates a task in a fresh project on a template and walks it
// to the state named.
func templateTask(t *testing.T, l *Local, ctx context.Context, template, project string, path ...string) core.TaskRef {
	t.Helper()
	if _, err := l.CreateProject(ctx, core.CreateProjectInput{Key: project, Name: project, WorkflowTemplate: template}); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	task, err := l.CreateTask(ctx, core.CreateTaskInput{ProjectRef: project, Title: "work"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	ref := core.TaskRef{ID: task.ID}
	for _, to := range path {
		if _, err := l.TransitionTask(ctx, ref, core.TransitionInput{To: to, Comment: "on the way"}); err != nil {
			t.Fatalf("moving to %s: %v", to, err)
		}
	}
	return ref
}

func TestTemplateRulesAreEnforced(t *testing.T) {
	l, scope, ctx := withDefaults(t)
	member := taskContext(seedTaskActor(t, l, scope, "worker", core.RoleMember))

	review := templateTask(t, l, ctx, "tix-dev-review", "dev", "ready", "in_progress", "in_review")
	if _, err := l.TransitionTask(member, review, core.TransitionInput{To: "in_progress"}); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("dev-review: sending back from review without a comment err = %v, want invalid", err)
	}
	if _, err := l.TransitionTask(member, review, core.TransitionInput{To: "in_progress", Comment: "needs tests"}); err != nil {
		t.Fatalf("dev-review: with a comment: %v", err)
	}

	agent := templateTask(t, l, ctx, "tix-agent-review", "agents", "claimed", "needs_review")
	if _, err := l.TransitionTask(member, agent, core.TransitionInput{To: "approved"}); !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("agent-review: approving without task:approve err = %v, want forbidden", err)
	}
	if _, err := l.TransitionTask(ctx, agent, core.TransitionInput{To: "rejected"}); err != nil {
		t.Fatalf("agent-review: rejecting as an admin: %v", err)
	}
	if _, err := l.TransitionTask(ctx, agent, core.TransitionInput{To: "queued"}); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("agent-review: requeueing without a comment err = %v, want invalid", err)
	}

	approver := taskContext(seedTaskActor(t, l, scope, "approver", core.RoleMember, core.ScopeTaskApprove))
	signed := templateTask(t, l, ctx, "tix-agent-review", "agents2", "claimed", "needs_review")
	if _, err := l.TransitionTask(approver, signed, core.TransitionInput{To: "approved"}); err != nil {
		t.Fatalf("agent-review: approving with task:approve: %v", err)
	}

	deal := templateTask(t, l, ctx, "tix-sales", "sales", "contacted")
	if _, err := l.TransitionTask(member, deal, core.TransitionInput{To: "lost"}); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("sales: losing a deal without a comment err = %v, want invalid", err)
	}
	if _, err := l.TransitionTask(member, deal, core.TransitionInput{To: "lost", Comment: "went with a competitor"}); err != nil {
		t.Fatalf("sales: with a comment: %v", err)
	}
}
