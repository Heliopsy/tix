package sshd

import (
	"bytes"
	"fmt"
	"time"

	"context"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
	"github.com/heliopsy/tix/internal/transfer"
)

// seedProjectKey is the board a new sandbox opens on.
const seedProjectKey = "demo"

// builtinWorkflowKey is the key a tenant's default state machine carries.
// Seeding it here means the sandbox's workflow is the builtin one rather than
// a second machine sitting beside it, so a project the visitor creates later
// inherits the same short lease.
const builtinWorkflowKey = "default"

// seed fills a fresh sandbox from a snapshot, through the same importer a
// hand-written snapshot goes through. Nothing here is a private path into the
// store: if the import would be rejected from the command line, it is rejected
// here too.
func (p *provisioner) seed(ctx context.Context, tenantID, actorID string) error {
	var buf bytes.Buffer
	if err := p.writeSnapshot(&buf, tenantID, actorID); err != nil {
		return err
	}
	system := core.WithSource(core.WithActor(ctx, core.SystemActor(tenantID)), core.SourceSystem)
	if _, err := p.service.ImportFrom(system, &buf, core.ImportInput{Mode: core.ImportMerge}); err != nil {
		return core.Internal("seeding sandbox %q", tenantID).Wrap(err)
	}
	return nil
}

// writeSnapshot renders the demo board as a snapshot document.
func (p *provisioner) writeSnapshot(buf *bytes.Buffer, tenantID, actorID string) error {
	now := p.clk.Now()
	enc := transfer.NewEncoder(buf)
	if err := enc.Header(tenantID, now); err != nil {
		return err
	}

	// Identifiers are minted per sandbox rather than fixed in the fixture, so
	// two sandboxes never race for the same primary key.
	workflowID := id.NewAt(now)
	projectID := id.NewAt(now)

	if err := enc.Encode(core.SnapshotRecord{Kind: core.RecordWorkflow, Workflow: &core.Workflow{
		ID:         workflowID,
		Key:        builtinWorkflowKey,
		Name:       "Default",
		Builtin:    true,
		Definition: demoWorkflow(p.leaseTTL),
		CreatedAt:  now,
		UpdatedAt:  now,
	}}); err != nil {
		return err
	}
	if err := enc.Encode(core.SnapshotRecord{Kind: core.RecordProject, Project: &core.Project{
		ID:          projectID,
		Key:         seedProjectKey,
		Name:        "Demo board",
		Description: "A sandbox of your own, seeded on your first connection.",
		WorkflowID:  workflowID,
		Color:       "blue",
		CreatedAt:   now,
		UpdatedAt:   now,
	}}); err != nil {
		return err
	}
	for _, t := range demoTasks(projectID, actorID, now, p.tenantTTL, p.leaseTTL) {
		if err := enc.Encode(core.SnapshotRecord{Kind: core.RecordTask, Task: &t}); err != nil {
			return err
		}
	}
	return enc.Err()
}

// demoWorkflow is the shipped state machine with a lease short enough that a
// visitor watches one expire rather than reading about it.
func demoWorkflow(lease time.Duration) core.WorkflowDefinition {
	return core.WorkflowDefinition{
		Initial: "todo",
		States: []core.State{
			{Key: "todo", Label: "To do", Category: core.CategoryTodo},
			{Key: "doing", Label: "Doing", Category: core.CategoryInProgress,
				RevertOnLeaseExpiry: true, RevertTo: "todo"},
			{Key: "blocked", Label: "Blocked", Category: core.CategoryTodo},
			{Key: "done", Label: "Done", Category: core.CategoryDone, Terminal: true},
			{Key: "cancelled", Label: "Cancelled", Category: core.CategoryDone, Terminal: true},
		},
		Transitions: []core.Transition{
			{From: "todo", To: "doing"},
			{From: "todo", To: "blocked"},
			{From: "todo", To: "cancelled"},
			{From: "doing", To: "todo"},
			{From: "doing", To: "blocked"},
			{From: "doing", To: "done"},
			{From: "doing", To: "cancelled"},
			{From: "blocked", To: "todo"},
			{From: "blocked", To: "doing"},
			{From: "blocked", To: "cancelled"},
			{From: "done", To: "todo"},
			{From: "cancelled", To: "todo"},
		},
		DefaultLease: core.Duration(lease),
	}
}

// demoTasks is the board a visitor lands on. The first row carries the notice
// that this is a sandbox and how long it survives, because someone doing real
// work in something that will be deleted deserves to know.
func demoTasks(projectID, actorID string, now time.Time, ttl, lease time.Duration) []core.Task {
	rows := []struct {
		title    string
		body     string
		status   string
		priority core.Priority
		tags     []string
	}{
		{
			title: fmt.Sprintf("This board is a demo sandbox, deleted after %s unvisited", short(ttl)),
			body: "Your ssh key owns this tenant and nobody else can see it. Connect with the same key " +
				"and you get this board back, with whatever you have changed still here. Leave it alone " +
				"for " + short(ttl) + " and it is deleted, so do not keep anything here you would miss.",
			status: "todo", priority: core.PriorityLow, tags: []string{"about"},
		},
		{
			title:  "Press c to claim this task, then wait",
			body:   "A claim is a lease, not an assignment. This one lasts " + short(lease) + ", after which the task returns to the board on its own and anybody, including an agent, can pick it up. That is what stops an abandoned claim blocking work forever.",
			status: "todo", priority: core.PriorityHigh, tags: []string{"leases"},
		},
		{
			title:  "Write the deployment guide",
			body:   "Cover the reverse proxy, the certificate and the backup schedule.",
			status: "doing", priority: core.PriorityNormal, tags: []string{"docs"},
		},
		{
			title:  "Ship the export command",
			body:   "A snapshot should round-trip through import without losing a field.",
			status: "todo", priority: core.PriorityHigh, tags: []string{"cli"},
		},
		{
			title:  "Decide on the retention default",
			status: "blocked", priority: core.PriorityNormal, tags: []string{"ops"},
		},
		{
			title:  "Move the audit log behind an interface",
			status: "todo", priority: core.PriorityLow, tags: []string{"refactor"},
		},
		{
			title:  "Add the keyset pagination benchmark",
			status: "done", priority: core.PriorityNormal, tags: []string{"perf"},
		},
	}

	out := make([]core.Task, 0, len(rows))
	for _, r := range rows {
		out = append(out, core.Task{
			ProjectID:      projectID,
			CreatorActorID: actorID,
			Title:          r.title,
			Body:           r.body,
			Status:         r.status,
			Priority:       r.priority,
			Tags:           r.tags,
			CreatedAt:      now,
			UpdatedAt:      now,
		})
	}
	return out
}

// short renders a duration the way a sentence wants it rather than the way Go
// prints it, so "6h0m0s" reads as "6h".
func short(d time.Duration) string {
	switch {
	case d >= time.Hour && d%time.Hour == 0:
		return fmt.Sprintf("%dh", int(d/time.Hour))
	case d >= time.Minute && d%time.Minute == 0:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	default:
		return d.String()
	}
}
