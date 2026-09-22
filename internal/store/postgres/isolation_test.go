package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// tenantData is everything one tenant owns in the isolation fixture. Both
// tenants are seeded with identical-looking values, so any leak is visible.
type tenantData struct {
	fixture
	task     core.Task
	tag      core.Tag
	comment  core.Comment
	artifact core.Artifact
	webhook  core.WebhookEndpoint
	token    core.APIToken
	sshKey   core.SSHKey
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

	err := s.Update(ctx, d.scope, func(tx store.Tx) error {
		d.domain = core.Domain{Hostname: hostname}
		if err := tx.AddDomain(ctx, &d.domain); err != nil {
			return err
		}
		if err := tx.AddMember(ctx, &core.Membership{ActorID: d.actor.ID, Role: core.RoleAdmin}); err != nil {
			return err
		}
		d.tag = core.Tag{Name: "urgent"}
		if err := tx.PutTag(ctx, &d.tag); err != nil {
			return err
		}
		if err := tx.AttachTag(ctx, d.task.ID, d.tag.ID); err != nil {
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
		d.webhook = core.WebhookEndpoint{URL: "https://shared.test/hook", Secret: "s", Active: true}
		if err := tx.PutWebhook(ctx, &d.webhook); err != nil {
			return err
		}
		d.token = core.APIToken{ActorID: d.actor.ID, Name: "shared"}
		if err := tx.CreateToken(ctx, &d.token, "hash-"+key); err != nil {
			return err
		}
		d.sshKey = core.SSHKey{ActorID: d.actor.ID, Fingerprint: "SHA256:shared",
			PublicKey: "ssh-ed25519 AAAAshared", Label: "shared"}
		if err := tx.CreateSSHKey(ctx, &d.sshKey); err != nil {
			return err
		}
		d.event = core.Event{Type: core.EventTaskCreated, SubjectType: "task", SubjectID: d.task.ID}
		if err := tx.AppendEvent(ctx, &d.event); err != nil {
			return err
		}
		d.audit = core.AuditEntry{Action: "task.create", SubjectType: "task", SubjectID: d.task.ID}
		if err := tx.AppendAudit(ctx, &d.audit); err != nil {
			return err
		}
		d.source = core.SyncSource{System: "github", Name: "main"}
		if err := tx.PutSyncSource(ctx, &d.source); err != nil {
			return err
		}
		return tx.PutExternalRef(ctx, &core.ExternalRef{EntityType: "task",
			EntityID: d.task.ID, System: "github", ExternalID: "42"})
	})
	if err != nil {
		t.Fatalf("seeding isolated tenant %q: %v", key, err)
	}
	return d
}

func TestTenantIsolationOnEveryRead(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	one := seedIsolated(t, s, clk, "one", "one.example")
	two := seedIsolated(t, s, clk, "two", "two.example")

	if err := s.View(ctx, one.scope, func(tx store.Tx) error {
		tasks, err := tx.ListTasks(ctx, core.TaskFilter{Page: core.Page{Limit: core.MaxPageLimit}})
		if err != nil {
			return err
		}
		if len(tasks) != 1 || tasks[0].ID != one.task.ID {
			t.Fatalf("tasks = %+v", tasks)
		}
		if _, err := tx.GetTask(ctx, core.TaskRef{ID: two.task.ID}); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("reading another tenant's task = %v, want not found", err)
		}
		if _, err := tx.GetComment(ctx, two.comment.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("reading another tenant's comment = %v, want not found", err)
		}
		if _, err := tx.GetWebhook(ctx, two.webhook.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("reading another tenant's webhook = %v, want not found", err)
		}
		if _, err := tx.GetActor(ctx, two.actor.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("reading another tenant's actor = %v, want not found", err)
		}
		if _, err := tx.GetProject(ctx, two.project.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("reading another tenant's project = %v, want not found", err)
		}
		if _, err := tx.GetWorkflowByID(ctx, two.workflow.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("reading another tenant's workflow = %v, want not found", err)
		}
		if _, err := tx.GetMember(ctx, two.actor.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("reading another tenant's membership = %v, want not found", err)
		}
		if _, err := tx.GetTokenByHash(ctx, "hash-two"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("reading another tenant's token = %v, want not found", err)
		}
		if _, err := tx.GetSyncSource(ctx, two.source.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("reading another tenant's sync source = %v, want not found", err)
		}

		single := []struct {
			name string
			run  func() (int, error)
		}{
			{"tags", func() (int, error) { v, err := tx.ListTags(ctx); return len(v), err }},
			{"workflows", func() (int, error) { v, err := tx.ListWorkflows(ctx); return len(v), err }},
			{"members", func() (int, error) { v, err := tx.ListMembers(ctx); return len(v), err }},
			{"domains", func() (int, error) { v, err := tx.ListDomains(ctx); return len(v), err }},
			{"webhooks", func() (int, error) { v, err := tx.ListWebhooks(ctx); return len(v), err }},
			{"comments", func() (int, error) { v, err := tx.ListComments(ctx, one.task.ID); return len(v), err }},
			{"artifacts", func() (int, error) { v, err := tx.ListArtifacts(ctx, one.task.ID); return len(v), err }},
			{"sync sources", func() (int, error) { v, err := tx.ListSyncSources(ctx); return len(v), err }},
			{"external refs", func() (int, error) { v, err := tx.ListExternalRefs(ctx, "github"); return len(v), err }},
			{"tokens", func() (int, error) { v, err := tx.ListTokens(ctx, one.actor.ID); return len(v), err }},
			{"ssh keys", func() (int, error) { v, err := tx.ListSSHKeys(ctx, one.actor.ID); return len(v), err }},
			{"events", func() (int, error) { v, err := tx.ReadEvents(ctx, 0, 100); return len(v), err }},
			{"audit", func() (int, error) { v, err := tx.ListAudit(ctx, core.AuditFilter{}); return len(v), err }},
			{"projects", func() (int, error) {
				v, err := tx.ListProjects(ctx, core.ProjectFilter{})
				return len(v), err
			}},
			{"actors", func() (int, error) {
				v, err := tx.ListActors(ctx, core.Page{Limit: 100})
				return len(v), err
			}},
		}
		for _, tc := range single {
			n, err := tc.run()
			if err != nil {
				return err
			}
			if n != 1 {
				t.Fatalf("%s listing returned %d rows, want only this tenant's 1", tc.name, n)
			}
		}

		latest, err := tx.LatestEventSeq(ctx)
		if err != nil {
			return err
		}
		if latest != one.event.Seq {
			t.Fatalf("latest event sequence = %d, want this tenant's %d", latest, one.event.Seq)
		}
		return nil
	}); err != nil {
		t.Fatalf("isolated reads: %v", err)
	}
}

func TestTenantIsolationOnEveryWrite(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	one := seedIsolated(t, s, clk, "one", "one.example")
	two := seedIsolated(t, s, clk, "two", "two.example")

	if err := s.Update(ctx, one.scope, func(tx store.Tx) error {
		if err := tx.DeleteTask(ctx, two.task.ID, false); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("soft deleting another tenant's task = %v, want not found", err)
		}
		if err := tx.DeleteTask(ctx, two.task.ID, true); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("hard deleting another tenant's task = %v, want not found", err)
		}
		if err := tx.DeleteComment(ctx, two.comment.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("deleting another tenant's comment = %v, want not found", err)
		}
		if err := tx.DeleteWebhook(ctx, two.webhook.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("deleting another tenant's webhook = %v, want not found", err)
		}
		if err := tx.DeleteProject(ctx, two.project.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("deleting another tenant's project = %v, want not found", err)
		}
		if err := tx.DeleteSyncSource(ctx, two.source.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("deleting another tenant's sync source = %v, want not found", err)
		}
		if err := tx.RemoveMember(ctx, two.actor.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("removing another tenant's member = %v, want not found", err)
		}
		if err := tx.RemoveDomain(ctx, "two.example"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("removing another tenant's domain = %v, want not found", err)
		}
		if err := tx.RevokeToken(ctx, two.token.ID, clk.Now()); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("revoking another tenant's token = %v, want not found", err)
		}
		if err := tx.ClearClaim(ctx, two.task.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("clearing another tenant's claim = %v, want not found", err)
		}
		ok, err := tx.ClaimTask(ctx, store.ClaimRow{TaskID: two.task.ID, ActorID: one.actor.ID,
			Now: clk.Now(), Until: clk.Now().Add(time.Hour), LeaseToken: "leak"})
		if err != nil {
			return err
		}
		if ok {
			t.Fatal("claimed another tenant's task")
		}
		if err := tx.DetachTag(ctx, two.task.ID, two.tag.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("detaching another tenant's tag = %v, want not found", err)
		}
		if err := tx.RemoveDependency(ctx, two.task.ID, one.task.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("removing another tenant's dependency = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("isolated writes: %v", err)
	}

	if err := s.View(ctx, two.scope, func(tx store.Tx) error {
		got, err := tx.GetTask(ctx, core.TaskRef{ID: two.task.ID})
		if err != nil {
			return err
		}
		if got.Deleted() || got.ClaimedByActorID != "" {
			t.Fatalf("the other tenant's task was touched: %+v", got)
		}
		comments, err := tx.ListComments(ctx, two.task.ID)
		if err != nil {
			return err
		}
		if len(comments) != 1 {
			t.Fatalf("the other tenant's comment was removed")
		}
		return nil
	}); err != nil {
		t.Fatalf("verifying the untouched tenant: %v", err)
	}
}

func TestPruningStaysInsideTheTenant(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	one := seedIsolated(t, s, clk, "one", "one.example")
	two := seedIsolated(t, s, clk, "two", "two.example")

	clk.Advance(72 * time.Hour)
	if err := s.Update(ctx, one.scope, func(tx store.Tx) error {
		if _, err := tx.PruneEvents(ctx, clk.Now(), one.event.Seq+1000, 0); err != nil {
			return err
		}
		_, err := tx.PruneAudit(ctx, clk.Now(), 0)
		return err
	}); err != nil {
		t.Fatalf("pruning: %v", err)
	}

	if err := s.View(ctx, two.scope, func(tx store.Tx) error {
		events, err := tx.ReadEvents(ctx, 0, 100)
		if err != nil {
			return err
		}
		if len(events) != 1 {
			t.Fatalf("the other tenant's events were pruned: %+v", events)
		}
		entries, err := tx.ListAudit(ctx, core.AuditFilter{})
		if err != nil {
			return err
		}
		if len(entries) != 1 {
			t.Fatalf("the other tenant's audit entries were pruned: %+v", entries)
		}
		return nil
	}); err != nil {
		t.Fatalf("verifying the untouched tenant: %v", err)
	}
}
