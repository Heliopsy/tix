// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func TestTenantRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
		got, err := u.GetTenantByKey(ctx, "acme")
		if err != nil {
			return err
		}
		if got.ID != f.tenant.ID {
			t.Fatalf("tenant by key = %q, want %q", got.ID, f.tenant.ID)
		}
		byID, err := u.GetTenantByID(ctx, f.tenant.ID)
		if err != nil {
			return err
		}
		byID.Name = "Acme Ltd"
		if err := u.UpdateTenant(ctx, byID); err != nil {
			return err
		}
		reread, err := u.GetTenantByID(ctx, f.tenant.ID)
		if err != nil {
			return err
		}
		if reread.Name != "Acme Ltd" {
			t.Fatalf("tenant name = %q", reread.Name)
		}
		if _, err := u.GetTenantByKey(ctx, "nobody"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("missing tenant = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("tenant round trip: %v", err)
	}
}

func TestDomainsAndMembership(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		d := core.Domain{Hostname: "acme.example"}
		if err := tx.AddDomain(ctx, &d); err != nil {
			return err
		}
		domains, err := tx.ListDomains(ctx)
		if err != nil {
			return err
		}
		if len(domains) != 1 || domains[0].Hostname != "acme.example" {
			t.Fatalf("domains = %+v", domains)
		}
		if domains[0].CertMode != core.CertNone {
			t.Fatalf("cert mode = %q", domains[0].CertMode)
		}
		tenant, err := tx.GetTenant(ctx)
		if err != nil {
			return err
		}
		if tenant.Key != "acme" {
			t.Fatalf("scoped tenant = %+v", tenant)
		}
		if err := tx.AddMember(ctx, &core.Membership{ActorID: f.actor.ID, Role: core.RoleAdmin}); err != nil {
			return err
		}
		m, err := tx.GetMember(ctx, f.actor.ID)
		if err != nil {
			return err
		}
		if m.Role != core.RoleAdmin {
			t.Fatalf("role = %q", m.Role)
		}
		members, err := tx.ListMembers(ctx)
		if err != nil {
			return err
		}
		if len(members) != 1 {
			t.Fatalf("members = %d", len(members))
		}
		if err := tx.RemoveMember(ctx, f.actor.ID); err != nil {
			return err
		}
		if err := tx.RemoveMember(ctx, f.actor.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("removing twice = %v, want not found", err)
		}
		if err := tx.RemoveDomain(ctx, "acme.example"); err != nil {
			return err
		}
		if err := tx.RemoveDomain(ctx, "acme.example"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("removing a missing domain = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("domains and membership: %v", err)
	}
}

func TestResolveDomain(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.AddDomain(ctx, &core.Domain{Hostname: "acme.example"})
	}); err != nil {
		t.Fatalf("adding domain: %v", err)
	}
	if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
		got, err := u.ResolveDomain(ctx, "acme.example")
		if err != nil {
			return err
		}
		if got.ID != f.tenant.ID {
			t.Fatalf("resolved tenant = %q", got.ID)
		}
		if _, err := u.ResolveDomain(ctx, "nowhere.example"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("unknown hostname = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("resolving domain: %v", err)
	}
}

func TestProjectAndFieldDefRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		p, err := tx.GetProject(ctx, "alpha")
		if err != nil {
			return err
		}
		p.Description = "the first"
		if err := tx.UpdateProject(ctx, p); err != nil {
			return err
		}
		byID, err := tx.GetProject(ctx, p.ID)
		if err != nil {
			return err
		}
		if byID.Description != "the first" {
			t.Fatalf("project = %+v", byID)
		}

		def := core.FieldDef{ProjectID: p.ID, Key: "severity", Label: "Severity",
			Type: core.FieldEnum, Required: true, EnumOptions: []string{"low", "high"},
			Default: "low", Indexed: true, Position: 2}
		if err := tx.PutFieldDef(ctx, &def); err != nil {
			return err
		}
		if def.ID == "" {
			t.Fatal("field definition was not given an identifier")
		}
		def.Label = "How bad"
		def.ID = ""
		if err := tx.PutFieldDef(ctx, &def); err != nil {
			return err
		}
		defs, err := tx.ListFieldDefs(ctx, p.ID)
		if err != nil {
			return err
		}
		if len(defs) != 1 {
			t.Fatalf("field definitions = %d, want 1", len(defs))
		}
		if defs[0].Label != "How bad" || !defs[0].Required || !defs[0].Indexed {
			t.Fatalf("field definition = %+v", defs[0])
		}
		if len(defs[0].EnumOptions) != 2 || defs[0].Default != "low" {
			t.Fatalf("field definition json = %+v", defs[0])
		}
		if err := tx.DeleteFieldDef(ctx, p.ID, "severity"); err != nil {
			return err
		}
		if err := tx.DeleteFieldDef(ctx, p.ID, "severity"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("deleting twice = %v, want not found", err)
		}

		archived := core.Project{Key: "beta", Name: "Beta", WorkflowID: f.workflow.ID}
		if err := tx.CreateProject(ctx, &archived); err != nil {
			return err
		}
		now := clk.Now()
		archived.ArchivedAt = &now
		if err := tx.UpdateProject(ctx, &archived); err != nil {
			return err
		}
		live, err := tx.ListProjects(ctx, core.ProjectFilter{})
		if err != nil {
			return err
		}
		if len(live) != 1 {
			t.Fatalf("live projects = %d, want 1", len(live))
		}
		all, err := tx.ListProjects(ctx, core.ProjectFilter{IncludeArchived: true, Keys: []string{"alpha", "beta"}})
		if err != nil {
			return err
		}
		if len(all) != 2 {
			t.Fatalf("all projects = %d, want 2", len(all))
		}
		if err := tx.DeleteProject(ctx, archived.ID); err != nil {
			return err
		}
		if err := tx.DeleteProject(ctx, archived.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("deleting twice = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("project round trip: %v", err)
	}
}

func TestWorkflowRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	f.newTask(t, "one", core.PriorityNormal)

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		w, err := tx.GetWorkflow(ctx, "default")
		if err != nil {
			return err
		}
		if len(w.Definition.States) != 3 || w.Builtin {
			t.Fatalf("workflow = %+v", w)
		}
		byID, err := tx.GetWorkflowByID(ctx, w.ID)
		if err != nil {
			return err
		}
		byID.Name = "Renamed"
		byID.Builtin = true
		if err := tx.PutWorkflow(ctx, byID); err != nil {
			return err
		}
		reread, err := tx.GetWorkflow(ctx, "default")
		if err != nil {
			return err
		}
		if reread.Name != "Renamed" || !reread.Builtin {
			t.Fatalf("updated workflow = %+v", reread)
		}
		counts, err := tx.CountTasksInStates(ctx, w.ID, []string{"todo", "done"})
		if err != nil {
			return err
		}
		if counts["todo"] != 1 || counts["done"] != 0 {
			t.Fatalf("counts = %+v", counts)
		}
		list, err := tx.ListWorkflows(ctx)
		if err != nil {
			return err
		}
		if len(list) != 1 {
			t.Fatalf("workflows = %d", len(list))
		}
		if _, err := tx.GetWorkflow(ctx, "nope"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("missing workflow = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("workflow round trip: %v", err)
	}
}

func TestTaskRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	due := clk.Now().Add(48 * time.Hour)
	task := core.Task{
		ProjectID: f.project.ID, Title: "ship the thing", Body: "with a description",
		Status: "todo", Priority: core.PriorityHigh, CreatorActorID: f.actor.ID,
		AssigneeActorID: f.actor.ID, DueAt: &due,
		CustomFields: map[string]any{"severity": "high"},
	}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.CreateTask(ctx, &task)
	}); err != nil {
		t.Fatalf("creating task: %v", err)
	}
	if task.Ref != "alpha-1" {
		t.Fatalf("task ref = %q, want alpha-1", task.Ref)
	}

	child := core.Task{ProjectID: f.project.ID, Title: "subtask", Status: "todo",
		CreatorActorID: f.actor.ID, ParentID: task.ID}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.CreateTask(ctx, &child)
	}); err != nil {
		t.Fatalf("creating child: %v", err)
	}

	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		byID, err := tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		if err != nil {
			return err
		}
		if byID.Title != "ship the thing" || byID.DueAt == nil || !byID.DueAt.Equal(due.UTC()) {
			t.Fatalf("task = %+v", byID)
		}
		if byID.CustomFields["severity"] != "high" {
			t.Fatalf("custom fields = %+v", byID.CustomFields)
		}
		byRef, err := tx.GetTask(ctx, core.TaskRef{ProjectKey: "alpha", Seq: 1})
		if err != nil {
			return err
		}
		if byRef.ID != task.ID {
			t.Fatalf("task by reference = %q", byRef.ID)
		}
		kids, err := tx.Children(ctx, task.ID)
		if err != nil {
			return err
		}
		if len(kids) != 1 || kids[0].ID != child.ID {
			t.Fatalf("children = %+v", kids)
		}
		if _, err := tx.GetTask(ctx, core.TaskRef{}); !core.IsKind(err, core.KindInvalid) {
			t.Fatalf("empty reference = %v, want invalid", err)
		}
		if _, err := tx.GetTask(ctx, core.TaskRef{ID: "missing"}); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("missing task = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading tasks: %v", err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		loaded, err := tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		if err != nil {
			return err
		}
		version := loaded.Version
		loaded.Title = "ship it"
		if err := tx.UpdateTask(ctx, loaded); err != nil {
			return err
		}
		if loaded.Version != version+1 {
			t.Fatalf("version = %d, want %d", loaded.Version, version+1)
		}
		if err := tx.DeleteTask(ctx, task.ID, false); err != nil {
			return err
		}
		if err := tx.DeleteTask(ctx, task.ID, false); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("second soft delete = %v, want not found", err)
		}
		deleted, err := tx.ListTasks(ctx, core.TaskFilter{IncludeDeleted: true})
		if err != nil {
			return err
		}
		if len(deleted) != 2 {
			t.Fatalf("tasks including deleted = %d, want 2", len(deleted))
		}
		live, err := tx.ListTasks(ctx, core.TaskFilter{})
		if err != nil {
			return err
		}
		if len(live) != 1 {
			t.Fatalf("live tasks = %d, want 1", len(live))
		}
		if err := tx.RestoreTask(ctx, task.ID); err != nil {
			return err
		}
		if err := tx.RestoreTask(ctx, task.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("restoring twice = %v, want not found", err)
		}
		if err := tx.DeleteTask(ctx, child.ID, true); err != nil {
			return err
		}
		if err := tx.DeleteTask(ctx, child.ID, true); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("hard deleting twice = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("updating tasks: %v", err)
	}
}

func TestTaskFiltersIncludingFullTextSearch(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	first := f.newTask(t, "migrate the database", core.PriorityHighest)
	clk.Advance(time.Second)
	second := f.newTask(t, "paint the bikeshed", core.PriorityLow)

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		loaded, err := tx.GetTask(ctx, core.TaskRef{ID: second.ID})
		if err != nil {
			return err
		}
		loaded.Body = "choose a colour"
		loaded.AssigneeActorID = f.actor.ID
		loaded.CustomFields = map[string]any{"team": "platform"}
		return tx.UpdateTask(ctx, loaded)
	}); err != nil {
		t.Fatalf("preparing tasks: %v", err)
	}

	cases := []struct {
		name   string
		filter core.TaskFilter
		want   []string
	}{
		{"search on the title", core.TaskFilter{Query: "database"}, []string{first.ID}},
		{"search on the body", core.TaskFilter{Query: "colour"}, []string{second.ID}},
		{"search matches nothing", core.TaskFilter{Query: "absent"}, nil},
		{"by priority", core.TaskFilter{Priorities: []core.Priority{core.PriorityHighest}}, []string{first.ID}},
		{"by status", core.TaskFilter{Statuses: []string{"todo"}}, []string{first.ID, second.ID}},
		{"by assignee", core.TaskFilter{AssigneeIDs: []string{f.actor.ID}}, []string{second.ID}},
		{"by creator", core.TaskFilter{CreatorIDs: []string{f.actor.ID}}, []string{first.ID, second.ID}},
		{"by project key", core.TaskFilter{ProjectKeys: []string{"alpha"}}, []string{first.ID, second.ID}},
		{"by project id", core.TaskFilter{ProjectIDs: []string{f.project.ID}}, []string{first.ID, second.ID}},
		{"by custom field", core.TaskFilter{CustomFields: map[string]any{"team": "platform"}}, []string{second.ID}},
		{"parent is null", core.TaskFilter{ParentIsNull: true}, []string{first.ID, second.ID}},
		{"unclaimed", core.TaskFilter{Claimed: core.No}, []string{first.ID, second.ID}},
		{"claimed", core.TaskFilter{Claimed: core.Yes}, nil},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var got []core.Task
			if err := s.View(ctx, f.scope, func(tx store.Tx) error {
				var err error
				got, err = tx.ListTasks(ctx, tc.filter)
				return err
			}); err != nil {
				t.Fatalf("listing: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("tasks = %d, want %d", len(got), len(tc.want))
			}
			seen := map[string]bool{}
			for _, task := range got {
				seen[task.ID] = true
			}
			for _, id := range tc.want {
				if !seen[id] {
					t.Fatalf("task %q missing from %+v", id, got)
				}
			}
		})
	}

	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		_, err := tx.ListTasks(ctx, core.TaskFilter{CustomFields: map[string]any{"bad key": "x"}})
		return err
	}); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("bad custom field key = %v, want invalid", err)
	}

	after := clk.Now().Add(-time.Hour)
	before := clk.Now().Add(72 * time.Hour)
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		loaded, err := tx.GetTask(ctx, core.TaskRef{ID: first.ID})
		if err != nil {
			return err
		}
		due := clk.Now().Add(24 * time.Hour)
		loaded.DueAt = &due
		if err := tx.UpdateTask(ctx, loaded); err != nil {
			return err
		}
		soon, err := tx.ListTasks(ctx, core.TaskFilter{DueBefore: &before, DueAfter: &after})
		if err != nil {
			return err
		}
		if len(soon) != 1 || soon[0].ID != first.ID {
			t.Fatalf("due window = %+v", soon)
		}
		return nil
	}); err != nil {
		t.Fatalf("due filters: %v", err)
	}
}

func TestTagsCommentsAndArtifacts(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	task := f.newTask(t, "tagged", core.PriorityNormal)

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		tag := core.Tag{Name: "urgent", Color: "red"}
		if err := tx.PutTag(ctx, &tag); err != nil {
			return err
		}
		tag.Color = "crimson"
		tag.ID = ""
		if err := tx.PutTag(ctx, &tag); err != nil {
			return err
		}
		scoped := core.Tag{Name: "urgent", ProjectID: f.project.ID}
		if err := tx.PutTag(ctx, &scoped); err != nil {
			return err
		}
		tags, err := tx.ListTags(ctx)
		if err != nil {
			return err
		}
		if len(tags) != 2 {
			t.Fatalf("tags = %d, want 2", len(tags))
		}
		if err := tx.AttachTag(ctx, task.ID, tag.ID); err != nil {
			return err
		}
		if err := tx.AttachTag(ctx, task.ID, tag.ID); err != nil {
			t.Fatalf("attaching twice must be idempotent: %v", err)
		}
		tagged, err := tx.ListTasks(ctx, core.TaskFilter{Tags: []string{"urgent"}})
		if err != nil {
			return err
		}
		if len(tagged) != 1 || len(tagged[0].Tags) != 1 {
			t.Fatalf("tagged tasks = %+v", tagged)
		}
		if err := tx.DetachTag(ctx, task.ID, tag.ID); err != nil {
			return err
		}
		if err := tx.DetachTag(ctx, task.ID, tag.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("detaching twice = %v, want not found", err)
		}

		c := core.Comment{TaskID: task.ID, AuthorActorID: f.actor.ID, Body: "first"}
		if err := tx.CreateComment(ctx, &c); err != nil {
			return err
		}
		c.Body = "edited"
		if err := tx.UpdateComment(ctx, &c); err != nil {
			return err
		}
		got, err := tx.GetComment(ctx, c.ID)
		if err != nil {
			return err
		}
		if got.Body != "edited" {
			t.Fatalf("comment = %+v", got)
		}
		comments, err := tx.ListComments(ctx, task.ID)
		if err != nil {
			return err
		}
		if len(comments) != 1 {
			t.Fatalf("comments = %d", len(comments))
		}
		if err := tx.DeleteComment(ctx, c.ID); err != nil {
			return err
		}
		if err := tx.DeleteComment(ctx, c.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("deleting twice = %v, want not found", err)
		}
		after, err := tx.ListComments(ctx, task.ID)
		if err != nil {
			return err
		}
		if len(after) != 0 {
			t.Fatalf("comments after deletion = %d", len(after))
		}

		a := core.Artifact{TaskID: task.ID, ActorID: f.actor.ID, Kind: core.ArtifactResult,
			Name: "report", Payload: map[string]any{"ok": true}, Blob: []byte("raw")}
		if err := tx.PutArtifact(ctx, &a); err != nil {
			return err
		}
		a.Name = "final report"
		if err := tx.PutArtifact(ctx, &a); err != nil {
			return err
		}
		artifacts, err := tx.ListArtifacts(ctx, task.ID)
		if err != nil {
			return err
		}
		if len(artifacts) != 1 || artifacts[0].Name != "final report" {
			t.Fatalf("artifacts = %+v", artifacts)
		}
		if string(artifacts[0].Blob) != "raw" || artifacts[0].Payload["ok"] != true {
			t.Fatalf("artifact payload = %+v", artifacts[0])
		}
		if artifacts[0].ContentType != "application/json" {
			t.Fatalf("content type = %q", artifacts[0].ContentType)
		}
		return nil
	}); err != nil {
		t.Fatalf("tags, comments and artifacts: %v", err)
	}
}

func TestActorsUsersSessionsAndTokens(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	var userID string

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		byHandle, err := tx.GetActorByHandle(ctx, "worker")
		if err != nil {
			return err
		}
		if byHandle.ID != f.actor.ID {
			t.Fatalf("actor by handle = %q", byHandle.ID)
		}
		if _, err := tx.GetActor(ctx, "missing"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("missing actor = %v, want not found", err)
		}
		actors, err := tx.ListActors(ctx, core.Page{Limit: 10})
		if err != nil {
			return err
		}
		if len(actors) != 1 {
			t.Fatalf("actors = %d", len(actors))
		}

		u := core.User{Email: "a@example.com", DisplayName: "A"}
		if err := tx.CreateUser(ctx, &u, "hash-1"); err != nil {
			return err
		}
		userID = u.ID
		u.DisplayName = "Alice"
		if err := tx.UpdateUser(ctx, &u, "hash-2"); err != nil {
			return err
		}
		reread, err := tx.GetUser(ctx, u.ID)
		if err != nil {
			return err
		}
		if reread.DisplayName != "Alice" {
			t.Fatalf("user = %+v", reread)
		}
		users, err := tx.ListUsers(ctx, core.Page{Limit: 10})
		if err != nil {
			return err
		}
		if len(users) != 1 {
			t.Fatalf("users = %d", len(users))
		}

		expiry := clk.Now().Add(time.Hour)
		if err := tx.CreateSession(ctx, f.actor.ID, "session-hash", expiry); err != nil {
			return err
		}
		actorID, at, err := tx.GetSessionByHash(ctx, "session-hash")
		if err != nil {
			return err
		}
		if actorID != f.actor.ID || !at.Equal(expiry.UTC()) {
			t.Fatalf("session = %q %v", actorID, at)
		}
		if _, _, err := tx.GetSessionByHash(ctx, "nope"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("missing session = %v, want not found", err)
		}
		if err := tx.CreateSession(ctx, f.actor.ID, "second", clk.Now().Add(-time.Hour)); err != nil {
			return err
		}
		n, err := tx.DeleteExpiredSessions(ctx, clk.Now())
		if err != nil {
			return err
		}
		if n != 1 {
			t.Fatalf("expired sessions removed = %d, want 1", n)
		}
		if _, err := tx.DeleteActorSessions(ctx, "  "); !core.IsKind(err, core.KindInvalid) {
			t.Fatalf("empty actor = %v, want invalid", err)
		}
		if n, err := tx.DeleteActorSessions(ctx, f.actor.ID); err != nil || n != 1 {
			t.Fatalf("deleting actor sessions = %d, %v", n, err)
		}
		if err := tx.DeleteSession(ctx, "gone"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("deleting a missing session = %v, want not found", err)
		}

		tk := core.APIToken{ActorID: f.actor.ID, Name: "ci", Scopes: []core.Scope{core.ScopeTaskRead}}
		if err := tx.CreateToken(ctx, &tk, "token-hash"); err != nil {
			return err
		}
		byHash, err := tx.GetTokenByHash(ctx, "token-hash")
		if err != nil {
			return err
		}
		if len(byHash.Scopes) != 1 || byHash.Scopes[0] != core.ScopeTaskRead {
			t.Fatalf("token scopes = %+v", byHash.Scopes)
		}
		if err := tx.TouchToken(ctx, tk.ID, clk.Now()); err != nil {
			return err
		}
		tokens, err := tx.ListTokens(ctx, f.actor.ID)
		if err != nil {
			return err
		}
		if len(tokens) != 1 || tokens[0].LastUsedAt == nil {
			t.Fatalf("tokens = %+v", tokens)
		}
		if err := tx.RevokeToken(ctx, tk.ID, clk.Now()); err != nil {
			return err
		}
		if err := tx.RevokeToken(ctx, "missing", clk.Now()); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("revoking a missing token = %v, want not found", err)
		}
		if err := tx.TouchToken(ctx, "missing", clk.Now()); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("touching a missing token = %v, want not found", err)
		}
		if _, err := tx.RevokeActorTokens(ctx, "", clk.Now()); !core.IsKind(err, core.KindInvalid) {
			t.Fatalf("revoking for an empty actor = %v, want invalid", err)
		}
		if n, err := tx.RevokeActorTokens(ctx, f.actor.ID, clk.Now()); err != nil || n != 0 {
			t.Fatalf("revoking already revoked tokens = %d, %v", n, err)
		}
		return nil
	}); err != nil {
		t.Fatalf("auth round trip: %v", err)
	}

	if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
		got, hash, err := u.GetUserByEmail(ctx, "a@example.com")
		if err != nil {
			return err
		}
		if hash != "hash-2" || got.ID != userID || got.DisplayName != "Alice" {
			t.Fatalf("user = %+v hash = %q", got, hash)
		}
		if _, _, err := u.GetUserByEmail(ctx, "nobody@example.com"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("missing user = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading users across tenants: %v", err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.DeleteUser(ctx, userID); err != nil {
			return err
		}
		if err := tx.DeleteUser(ctx, userID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("deleting twice = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("deleting the user: %v", err)
	}
}

func TestEventsAuditAndRetention(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	task := f.newTask(t, "audited", core.PriorityNormal)

	var firstSeq int64
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		e := core.Event{Type: core.EventTaskCreated, SubjectType: "task", SubjectID: task.ID,
			ProjectID: f.project.ID, ActorID: f.actor.ID, Payload: map[string]any{"title": "audited"}}
		if err := tx.AppendEvent(ctx, &e); err != nil {
			return err
		}
		if e.Seq == 0 {
			t.Fatal("event was not given a sequence number")
		}
		firstSeq = e.Seq
		second := core.Event{Type: core.EventTaskUpdated, SubjectType: "task", SubjectID: task.ID}
		if err := tx.AppendEvent(ctx, &second); err != nil {
			return err
		}
		if second.Seq <= firstSeq {
			t.Fatalf("sequence did not advance: %d then %d", firstSeq, second.Seq)
		}
		a := core.AuditEntry{ActorID: f.actor.ID, Action: "task.create", SubjectType: "task",
			SubjectID: task.ID, After: json.RawMessage(`{"title":"audited"}`), Source: core.SourceCLI}
		if err := tx.AppendAudit(ctx, &a); err != nil {
			return err
		}
		if a.Seq == 0 {
			t.Fatal("audit entry was not given a sequence number")
		}
		return nil
	}); err != nil {
		t.Fatalf("appending: %v", err)
	}

	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		events, err := tx.ReadEvents(ctx, 0, 0)
		if err != nil {
			return err
		}
		if len(events) != 2 {
			t.Fatalf("events = %d, want 2", len(events))
		}
		if events[0].Payload["title"] != "audited" || events[0].ProjectID != f.project.ID {
			t.Fatalf("event = %+v", events[0])
		}
		latest, err := tx.LatestEventSeq(ctx)
		if err != nil {
			return err
		}
		if latest != events[1].Seq {
			t.Fatalf("latest sequence = %d, want %d", latest, events[1].Seq)
		}
		after, err := tx.ReadEvents(ctx, events[0].Seq, 10)
		if err != nil {
			return err
		}
		if len(after) != 1 {
			t.Fatalf("events after the first = %d, want 1", len(after))
		}

		entries, err := tx.ListAudit(ctx, core.AuditFilter{SubjectType: "task", SubjectID: task.ID,
			ActorIDs: []string{f.actor.ID}, Actions: []string{"task.create"},
			Sources: []core.Source{core.SourceCLI}})
		if err != nil {
			return err
		}
		if len(entries) != 1 {
			t.Fatalf("audit entries = %d, want 1", len(entries))
		}
		var decoded map[string]any
		if err := json.Unmarshal(entries[0].After, &decoded); err != nil {
			t.Fatalf("decoding audit after state: %v", err)
		}
		if decoded["title"] != "audited" {
			t.Fatalf("audit after state = %v", decoded)
		}
		since := clk.Now().Add(-time.Hour)
		until := clk.Now().Add(time.Hour)
		windowed, err := tx.ListAudit(ctx, core.AuditFilter{Since: &since, Until: &until})
		if err != nil {
			return err
		}
		if len(windowed) != 1 {
			t.Fatalf("windowed audit entries = %d, want 1", len(windowed))
		}
		return nil
	}); err != nil {
		t.Fatalf("reading events and audit: %v", err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		p, err := tx.GetRetention(ctx)
		if err != nil {
			return err
		}
		if p.TenantID != f.tenant.ID {
			t.Fatalf("default retention = %+v", p)
		}
		p.Events = core.Duration(time.Hour)
		if err := tx.PutRetention(ctx, p); err != nil {
			return err
		}
		p.Events = core.Duration(2 * time.Hour)
		if err := tx.PutRetention(ctx, p); err != nil {
			return err
		}
		stored, err := tx.GetRetention(ctx)
		if err != nil {
			return err
		}
		if time.Duration(stored.Events) != 2*time.Hour {
			t.Fatalf("retention = %+v", stored)
		}
		return nil
	}); err != nil {
		t.Fatalf("retention: %v", err)
	}

	clk.Advance(48 * time.Hour)
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		n, err := tx.PruneEvents(ctx, clk.Now(), firstSeq, 0)
		if err != nil {
			return err
		}
		if n != 1 {
			t.Fatalf("pruned events = %d, want 1", n)
		}
		if _, err := tx.PruneAudit(ctx, clk.Now(), 0); err != nil {
			return err
		}
		if _, err := tx.PruneDeliveries(ctx, clk.Now(), 0); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("pruning: %v", err)
	}
}

func TestWebhooksAndDeliveries(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	endpoint := core.WebhookEndpoint{URL: "https://example.test/hook", Secret: "s3cret", Active: true}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.PutWebhook(ctx, &endpoint); err != nil {
			return err
		}
		endpoint.URL = "https://example.test/hook2"
		if err := tx.PutWebhook(ctx, &endpoint); err != nil {
			return err
		}
		got, err := tx.GetWebhook(ctx, endpoint.ID)
		if err != nil {
			return err
		}
		if got.URL != "https://example.test/hook2" || !got.Active {
			t.Fatalf("webhook = %+v", got)
		}
		if len(got.EventTypes) != 1 || got.EventTypes[0] != "*" {
			t.Fatalf("event types = %+v", got.EventTypes)
		}
		list, err := tx.ListWebhooks(ctx)
		if err != nil {
			return err
		}
		if len(list) != 1 {
			t.Fatalf("webhooks = %d", len(list))
		}
		return nil
	}); err != nil {
		t.Fatalf("webhook round trip: %v", err)
	}

	delivery := core.WebhookDelivery{EndpointID: endpoint.ID, EventSeq: 1}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.EnqueueDelivery(ctx, &delivery); err != nil {
			return err
		}
		claimed, err := tx.ClaimDeliveries(ctx, "owner-1", clk.Now(), clk.Now().Add(time.Minute), 10)
		if err != nil {
			return err
		}
		if len(claimed) != 1 || claimed[0].ID != delivery.ID {
			t.Fatalf("claimed deliveries = %+v", claimed)
		}
		again, err := tx.ClaimDeliveries(ctx, "owner-2", clk.Now(), clk.Now().Add(time.Minute), 10)
		if err != nil {
			return err
		}
		if len(again) != 0 {
			t.Fatalf("a locked delivery was claimed twice: %+v", again)
		}
		if err := tx.MarkFailed(ctx, delivery.ID, 500, "boom", clk.Now().Add(time.Minute), false); err != nil {
			return err
		}
		failed, err := tx.GetDelivery(ctx, delivery.ID)
		if err != nil {
			return err
		}
		if failed.Attempts != 1 || failed.Status != core.DeliveryPending || failed.LastStatusCode != 500 {
			t.Fatalf("delivery = %+v", failed)
		}
		if err := tx.MarkDelivered(ctx, delivery.ID, 200, clk.Now()); err != nil {
			return err
		}
		done, err := tx.GetDelivery(ctx, delivery.ID)
		if err != nil {
			return err
		}
		if done.Status != core.DeliveryDelivered || done.Attempts != 2 {
			t.Fatalf("delivery = %+v", done)
		}
		listed, err := tx.ListDeliveries(ctx, core.DeliveryFilter{EndpointID: endpoint.ID,
			Statuses: []core.DeliveryStatus{core.DeliveryDelivered}})
		if err != nil {
			return err
		}
		if len(listed) != 1 {
			t.Fatalf("deliveries = %d", len(listed))
		}
		if err := tx.MarkDelivered(ctx, "missing", 200, clk.Now()); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("marking a missing delivery = %v, want not found", err)
		}
		if err := tx.MarkFailed(ctx, "missing", 500, "", clk.Now(), true); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("failing a missing delivery = %v, want not found", err)
		}
		if _, err := tx.GetDelivery(ctx, "missing"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("missing delivery = %v, want not found", err)
		}
		if err := tx.DeleteWebhook(ctx, endpoint.ID); err != nil {
			return err
		}
		if err := tx.DeleteWebhook(ctx, endpoint.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("deleting twice = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("delivery round trip: %v", err)
	}
}

func TestSyncSourcesAndExternalRefs(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		src := core.SyncSource{System: "github", Name: "main", Cursor: "1"}
		if err := tx.PutSyncSource(ctx, &src); err != nil {
			return err
		}
		src.Cursor = "2"
		src.ID = ""
		if err := tx.PutSyncSource(ctx, &src); err != nil {
			return err
		}
		got, err := tx.GetSyncSource(ctx, src.ID)
		if err != nil {
			return err
		}
		if got.Cursor != "2" {
			t.Fatalf("sync source = %+v", got)
		}
		sources, err := tx.ListSyncSources(ctx)
		if err != nil {
			return err
		}
		if len(sources) != 1 {
			t.Fatalf("sources = %d", len(sources))
		}

		ref := core.ExternalRef{EntityType: "task", EntityID: "t1", System: "github", ExternalID: "42"}
		if err := tx.PutExternalRef(ctx, &ref); err != nil {
			return err
		}
		ref.ExternalURL = "https://github.test/42"
		if err := tx.PutExternalRef(ctx, &ref); err != nil {
			return err
		}
		stored, err := tx.GetExternalRef(ctx, "github", "42", "task")
		if err != nil {
			return err
		}
		if stored.ExternalURL != "https://github.test/42" {
			t.Fatalf("external reference = %+v", stored)
		}
		refs, err := tx.ListExternalRefs(ctx, "github")
		if err != nil {
			return err
		}
		if len(refs) != 1 {
			t.Fatalf("external references = %d", len(refs))
		}
		if _, err := tx.GetExternalRef(ctx, "github", "99", "task"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("missing reference = %v, want not found", err)
		}
		if err := tx.DeleteSyncSource(ctx, src.ID); err != nil {
			return err
		}
		if err := tx.DeleteSyncSource(ctx, src.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("deleting twice = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("sync round trip: %v", err)
	}
}

func TestDriverErrorsMapToCoreKinds(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		dup := core.Project{Key: "alpha", Name: "Duplicate", WorkflowID: f.workflow.ID}
		if err := tx.CreateProject(ctx, &dup); !core.IsKind(err, core.KindConflict) {
			t.Fatalf("duplicate project key = %v, want conflict", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("conflict case: %v", err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		orphan := core.Task{ProjectID: "no-such-project", Title: "orphan", Status: "todo",
			CreatorActorID: f.actor.ID, Seq: 1}
		if err := tx.CreateTask(ctx, &orphan); !core.IsKind(err, core.KindPrecondition) {
			t.Fatalf("missing project = %v, want precondition", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("precondition case: %v", err)
	}

	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		if _, err := tx.GetProject(ctx, "gone"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("missing project = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("not found case: %v", err)
	}
}

func TestRollbackDiscardsWrites(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	txn, err := s.Begin(ctx, f.scope)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	task := core.Task{ProjectID: f.project.ID, Title: "doomed", Status: "todo", CreatorActorID: f.actor.ID}
	if err := txn.CreateTask(ctx, &task); err != nil {
		t.Fatalf("creating task: %v", err)
	}
	if txn.Scope().TenantID != f.tenant.ID {
		t.Fatalf("scope = %+v", txn.Scope())
	}
	if err := txn.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if err := txn.Rollback(); err != nil {
		t.Fatalf("second rollback: %v", err)
	}
	if err := txn.Commit(); err == nil {
		t.Fatal("committing a finished transaction must fail")
	}

	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		tasks, err := tx.ListTasks(ctx, core.TaskFilter{})
		if err != nil {
			return err
		}
		if len(tasks) != 0 {
			t.Fatalf("rolled back task is visible: %+v", tasks)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading after rollback: %v", err)
	}
}
