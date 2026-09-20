package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/thereisnotime/tix/internal/clock"
	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/store"
)

// tenantData is everything one tenant owns in the isolation fixture. Both
// tenants are seeded with identical-looking values, so any leak is visible.
type tenantData struct {
	fixture
	task     core.Task
	other    core.Task
	label    core.Label
	comment  core.Comment
	artifact core.Artifact
	webhook  core.WebhookEndpoint
	token    core.APIToken
	event    core.Event
	audit    core.AuditEntry
	source   core.SyncSource
	domain   core.Domain
}

func seedIsolated(t *testing.T, s *Store, clk *clock.Fake, key, hostname string) tenantData {
	t.Helper()
	ctx := context.Background()
	d := tenantData{fixture: seed(t, s, clk, key)}
	d.task = d.newTask(t, "shared title", core.PriorityNormal)
	d.other = d.newTask(t, "shared title", core.PriorityLow)

	err := s.Update(ctx, d.scope, func(tx store.Tx) error {
		d.domain = core.Domain{Hostname: hostname}
		if err := tx.AddDomain(ctx, &d.domain); err != nil {
			return err
		}
		if err := tx.AddMember(ctx, &core.Membership{ActorID: d.actor.ID, Role: core.RoleAdmin}); err != nil {
			return err
		}
		d.label = core.Label{Name: "urgent"}
		if err := tx.PutLabel(ctx, &d.label); err != nil {
			return err
		}
		if err := tx.AttachLabel(ctx, d.task.ID, d.label.ID); err != nil {
			return err
		}
		d.comment = core.Comment{TaskID: d.task.ID, AuthorActorID: d.actor.ID, Body: "shared body"}
		if err := tx.CreateComment(ctx, &d.comment); err != nil {
			return err
		}
		d.artifact = core.Artifact{TaskID: d.task.ID, ActorID: d.actor.ID,
			Kind: core.ArtifactResult, Name: "shared"}
		if err := tx.PutArtifact(ctx, &d.artifact); err != nil {
			return err
		}
		if err := tx.AddDependency(ctx, &core.Dependency{TaskID: d.task.ID, DependsOn: d.other.ID}); err != nil {
			return err
		}
		if err := tx.PutFieldDef(ctx, &core.FieldDef{ProjectID: d.project.ID,
			Key: "severity", Label: "Severity", Type: core.FieldString}); err != nil {
			return err
		}
		d.token = core.APIToken{ActorID: d.actor.ID, Name: "ci"}
		if err := tx.CreateToken(ctx, &d.token, "hash-"+key); err != nil {
			return err
		}
		if err := tx.CreateSession(ctx, d.actor.ID, "session-"+key, clk.Now().Add(time.Hour)); err != nil {
			return err
		}
		d.webhook = core.WebhookEndpoint{URL: "https://example.test/hook", Secret: "shh", Active: true}
		if err := tx.PutWebhook(ctx, &d.webhook); err != nil {
			return err
		}
		if err := tx.EnqueueDelivery(ctx, &core.WebhookDelivery{EndpointID: d.webhook.ID, EventSeq: 1}); err != nil {
			return err
		}
		d.event = core.Event{Type: core.EventTaskCreated, ProjectID: d.project.ID,
			SubjectType: "task", SubjectID: d.task.ID, ActorID: d.actor.ID}
		if err := tx.AppendEvent(ctx, &d.event); err != nil {
			return err
		}
		d.audit = core.AuditEntry{ActorID: d.actor.ID, Action: "task.create",
			SubjectType: "task", SubjectID: d.task.ID, Source: core.SourceCLI}
		if err := tx.AppendAudit(ctx, &d.audit); err != nil {
			return err
		}
		d.source = core.SyncSource{System: "github", Name: "main", Cursor: "c1"}
		if err := tx.PutSyncSource(ctx, &d.source); err != nil {
			return err
		}
		return tx.PutExternalRef(ctx, &core.ExternalRef{EntityType: "task", EntityID: d.task.ID,
			System: "github", ExternalID: "42"})
	})
	if err != nil {
		t.Fatalf("seeding isolated tenant %q: %v", key, err)
	}
	return d
}

func TestTenantIsolationOnReads(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	a := seedIsolated(t, s, clk, "acme", "acme.example")
	b := seedIsolated(t, s, clk, "globex", "globex.example")

	for _, own := range []tenantData{a, b} {
		own := own
		t.Run(own.tenant.Key, func(t *testing.T) {
			err := s.View(ctx, own.scope, func(tx store.Tx) error {
				tenant, err := tx.GetTenant(ctx)
				if err != nil {
					return err
				}
				if tenant.ID != own.tenant.ID {
					t.Fatalf("GetTenant returned %q, want %q", tenant.ID, own.tenant.ID)
				}

				tasks, err := tx.ListTasks(ctx, core.TaskFilter{})
				if err != nil {
					return err
				}
				if len(tasks) != 2 {
					t.Fatalf("ListTasks returned %d rows, want 2", len(tasks))
				}
				for _, task := range tasks {
					if task.TenantID != own.tenant.ID {
						t.Fatalf("ListTasks leaked a task of tenant %q", task.TenantID)
					}
				}

				projects, err := tx.ListProjects(ctx, core.ProjectFilter{})
				if err != nil {
					return err
				}
				if len(projects) != 1 || projects[0].TenantID != own.tenant.ID {
					t.Fatalf("ListProjects leaked: %+v", projects)
				}

				workflows, err := tx.ListWorkflows(ctx)
				if err != nil {
					return err
				}
				if len(workflows) != 1 || workflows[0].TenantID != own.tenant.ID {
					t.Fatalf("ListWorkflows leaked: %+v", workflows)
				}

				domains, err := tx.ListDomains(ctx)
				if err != nil {
					return err
				}
				if len(domains) != 1 || domains[0].TenantID != own.tenant.ID {
					t.Fatalf("ListDomains leaked: %+v", domains)
				}

				members, err := tx.ListMembers(ctx)
				if err != nil {
					return err
				}
				if len(members) != 1 || members[0].TenantID != own.tenant.ID {
					t.Fatalf("ListMembers leaked: %+v", members)
				}

				actors, err := tx.ListActors(ctx, core.Page{})
				if err != nil {
					return err
				}
				if len(actors) != 1 || actors[0].TenantID != own.tenant.ID {
					t.Fatalf("ListActors leaked: %+v", actors)
				}

				labels, err := tx.ListLabels(ctx)
				if err != nil {
					return err
				}
				if len(labels) != 1 || labels[0].TenantID != own.tenant.ID {
					t.Fatalf("ListLabels leaked: %+v", labels)
				}

				comments, err := tx.ListComments(ctx, own.task.ID)
				if err != nil {
					return err
				}
				if len(comments) != 1 || comments[0].TenantID != own.tenant.ID {
					t.Fatalf("ListComments leaked: %+v", comments)
				}

				artifacts, err := tx.ListArtifacts(ctx, own.task.ID)
				if err != nil {
					return err
				}
				if len(artifacts) != 1 || artifacts[0].TenantID != own.tenant.ID {
					t.Fatalf("ListArtifacts leaked: %+v", artifacts)
				}

				deps, err := tx.ListDependencies(ctx, own.task.ID)
				if err != nil {
					return err
				}
				if len(deps) != 1 || deps[0].TenantID != own.tenant.ID {
					t.Fatalf("ListDependencies leaked: %+v", deps)
				}

				defs, err := tx.ListFieldDefs(ctx, own.project.ID)
				if err != nil {
					return err
				}
				if len(defs) != 1 || defs[0].TenantID != own.tenant.ID {
					t.Fatalf("ListFieldDefs leaked: %+v", defs)
				}

				tokens, err := tx.ListTokens(ctx, own.actor.ID)
				if err != nil {
					return err
				}
				if len(tokens) != 1 || tokens[0].TenantID != own.tenant.ID {
					t.Fatalf("ListTokens leaked: %+v", tokens)
				}

				hooks, err := tx.ListWebhooks(ctx)
				if err != nil {
					return err
				}
				if len(hooks) != 1 || hooks[0].TenantID != own.tenant.ID {
					t.Fatalf("ListWebhooks leaked: %+v", hooks)
				}

				deliveries, err := tx.ListDeliveries(ctx, core.DeliveryFilter{})
				if err != nil {
					return err
				}
				if len(deliveries) != 1 || deliveries[0].TenantID != own.tenant.ID {
					t.Fatalf("ListDeliveries leaked: %+v", deliveries)
				}

				events, err := tx.ReadEvents(ctx, 0, 100)
				if err != nil {
					return err
				}
				if len(events) != 1 || events[0].TenantID != own.tenant.ID {
					t.Fatalf("ReadEvents leaked: %+v", events)
				}

				entries, err := tx.ListAudit(ctx, core.AuditFilter{})
				if err != nil {
					return err
				}
				if len(entries) != 1 || entries[0].TenantID != own.tenant.ID {
					t.Fatalf("ListAudit leaked: %+v", entries)
				}

				sources, err := tx.ListSyncSources(ctx)
				if err != nil {
					return err
				}
				if len(sources) != 1 || sources[0].TenantID != own.tenant.ID {
					t.Fatalf("ListSyncSources leaked: %+v", sources)
				}

				refs, err := tx.ListExternalRefs(ctx, "github")
				if err != nil {
					return err
				}
				if len(refs) != 1 || refs[0].TenantID != own.tenant.ID {
					t.Fatalf("ListExternalRefs leaked: %+v", refs)
				}

				leases, err := tx.ExpiredLeases(ctx, clk.Now(), 100)
				if err != nil {
					return err
				}
				for _, l := range leases {
					if l.TenantID != own.tenant.ID {
						t.Fatalf("ExpiredLeases leaked a task of tenant %q", l.TenantID)
					}
				}
				return nil
			})
			if err != nil {
				t.Fatalf("reading as tenant %q: %v", own.tenant.Key, err)
			}
		})
	}
}

func TestTenantIsolationOnPointReads(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	a := seedIsolated(t, s, clk, "acme", "acme.example")
	b := seedIsolated(t, s, clk, "globex", "globex.example")

	err := s.View(ctx, a.scope, func(tx store.Tx) error {
		cases := []struct {
			name string
			err  error
		}{
			{"GetTask", errOf(func() error { _, e := tx.GetTask(ctx, core.TaskRef{ID: b.task.ID}); return e })},
			{"GetProject", errOf(func() error { _, e := tx.GetProject(ctx, b.project.ID); return e })},
			{"GetWorkflowByID", errOf(func() error { _, e := tx.GetWorkflowByID(ctx, b.workflow.ID); return e })},
			{"GetActor", errOf(func() error { _, e := tx.GetActor(ctx, b.actor.ID); return e })},
			{"GetComment", errOf(func() error { _, e := tx.GetComment(ctx, b.comment.ID); return e })},
			{"GetWebhook", errOf(func() error { _, e := tx.GetWebhook(ctx, b.webhook.ID); return e })},
			{"GetSyncSource", errOf(func() error { _, e := tx.GetSyncSource(ctx, b.source.ID); return e })},
			{"GetMember", errOf(func() error { _, e := tx.GetMember(ctx, b.actor.ID); return e })},
			{"GetTokenByHash", errOf(func() error { _, e := tx.GetTokenByHash(ctx, "hash-globex"); return e })},
			{"GetSessionByHash", errOf(func() error { _, _, e := tx.GetSessionByHash(ctx, "session-globex"); return e })},
		}
		for _, c := range cases {
			if !core.IsKind(c.err, core.KindNotFound) {
				t.Fatalf("%s across tenants = %v, want not found", c.name, c.err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("point reads: %v", err)
	}
}

func TestTenantIsolationOnWrites(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	a := seedIsolated(t, s, clk, "acme", "acme.example")
	b := seedIsolated(t, s, clk, "globex", "globex.example")

	err := s.Update(ctx, a.scope, func(tx store.Tx) error {
		stolen := b.task
		stolen.Title = "stolen"
		if err := tx.UpdateTask(ctx, &stolen); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("UpdateTask across tenants = %v, want not found", err)
		}
		if err := tx.DeleteTask(ctx, b.task.ID, false); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("DeleteTask across tenants = %v, want not found", err)
		}
		if err := tx.DeleteTask(ctx, b.task.ID, true); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("hard DeleteTask across tenants = %v, want not found", err)
		}
		project := b.project
		project.Name = "stolen"
		if err := tx.UpdateProject(ctx, &project); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("UpdateProject across tenants = %v, want not found", err)
		}
		if err := tx.DeleteProject(ctx, b.project.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("DeleteProject across tenants = %v, want not found", err)
		}
		if err := tx.DeleteWebhook(ctx, b.webhook.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("DeleteWebhook across tenants = %v, want not found", err)
		}
		if err := tx.DeleteComment(ctx, b.comment.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("DeleteComment across tenants = %v, want not found", err)
		}
		if err := tx.DetachLabel(ctx, b.task.ID, b.label.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("DetachLabel across tenants = %v, want not found", err)
		}
		if err := tx.RemoveDependency(ctx, b.task.ID, b.other.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("RemoveDependency across tenants = %v, want not found", err)
		}
		if err := tx.RemoveDomain(ctx, "globex.example"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("RemoveDomain across tenants = %v, want not found", err)
		}
		if err := tx.RemoveMember(ctx, b.actor.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("RemoveMember across tenants = %v, want not found", err)
		}
		if err := tx.ClearClaim(ctx, b.task.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("ClearClaim across tenants = %v, want not found", err)
		}
		claimed, err := tx.ClaimTask(ctx, store.ClaimRow{
			TaskID: b.task.ID, ActorID: a.actor.ID, Now: clk.Now(),
			Until: clk.Now().Add(time.Minute), LeaseToken: "cross-tenant",
		})
		if err != nil {
			return err
		}
		if claimed {
			t.Fatal("ClaimTask claimed another tenant's task")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("cross-tenant writes: %v", err)
	}

	if err := s.View(ctx, b.scope, func(tx store.Tx) error {
		task, err := tx.GetTask(ctx, core.TaskRef{ID: b.task.ID})
		if err != nil {
			return err
		}
		if task.Title != "shared title" || task.Deleted() || task.ClaimedByActorID != "" {
			t.Fatalf("the other tenant's task was modified: %+v", task)
		}
		if _, err := tx.GetProject(ctx, b.project.ID); err != nil {
			t.Fatalf("the other tenant's project was modified: %v", err)
		}
		comments, err := tx.ListComments(ctx, b.task.ID)
		if err != nil {
			return err
		}
		if len(comments) != 1 {
			t.Fatalf("the other tenant's comments were modified: %+v", comments)
		}
		return nil
	}); err != nil {
		t.Fatalf("verifying the other tenant: %v", err)
	}
}

func TestClaimNextTaskStaysInsideTheTenant(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	a := seedIsolated(t, s, clk, "acme", "acme.example")
	b := seedIsolated(t, s, clk, "globex", "globex.example")

	var claimed string
	err := s.Update(ctx, a.scope, func(tx store.Tx) error {
		id, ok, err := tx.ClaimNextTask(ctx, store.ClaimNextRow{
			ProjectIDs: []string{b.project.ID}, ActorID: a.actor.ID,
			Now: clk.Now(), Until: clk.Now().Add(time.Minute), LeaseToken: "t1",
			TerminalStates: []string{"done"},
		})
		if err != nil {
			return err
		}
		if ok {
			t.Fatalf("claimed %q from another tenant's project", id)
		}
		id, ok, err = tx.ClaimNextTask(ctx, store.ClaimNextRow{
			ActorID: a.actor.ID, Now: clk.Now(), Until: clk.Now().Add(time.Minute),
			LeaseToken: "t2", TerminalStates: []string{"done"},
		})
		if err != nil {
			return err
		}
		if !ok {
			t.Fatal("expected a claim inside the caller's own tenant")
		}
		claimed = id
		return nil
	})
	if err != nil {
		t.Fatalf("claiming next: %v", err)
	}
	if claimed != a.other.ID {
		t.Fatalf("claimed %q, want the unblocked task %q", claimed, a.other.ID)
	}
}

func errOf(fn func() error) error { return fn() }
