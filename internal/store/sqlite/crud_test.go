package sqlite

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/store"
)

func TestTenantAndDomainRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		got, err := tx.GetTenant(ctx)
		if err != nil {
			return err
		}
		if got.Key != "acme" {
			t.Fatalf("tenant key = %q, want %q", got.Key, "acme")
		}
		d := core.Domain{Hostname: "acme.example", CertMode: core.CertFile, CertPath: "/c", KeyPath: "/k"}
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
		if domains[0].Verified() {
			t.Fatal("new domain should not be verified")
		}
		m := core.Membership{ActorID: f.actor.ID, Role: core.RoleAdmin}
		if err := tx.AddMember(ctx, &m); err != nil {
			return err
		}
		member, err := tx.GetMember(ctx, f.actor.ID)
		if err != nil {
			return err
		}
		if member.Role != core.RoleAdmin {
			t.Fatalf("role = %q, want admin", member.Role)
		}
		members, err := tx.ListMembers(ctx)
		if err != nil {
			return err
		}
		if len(members) != 1 {
			t.Fatalf("members = %d, want 1", len(members))
		}
		if err := tx.RemoveMember(ctx, f.actor.ID); err != nil {
			return err
		}
		if err := tx.RemoveDomain(ctx, "acme.example"); err != nil {
			return err
		}
		if err := tx.RemoveDomain(ctx, "acme.example"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("removing a missing domain = %v, want not found", err)
		}
		if err := tx.RemoveMember(ctx, f.actor.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("removing a missing member = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("tenant round trip: %v", err)
	}
}

func TestUnscopedTenantOperations(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
		byKey, err := u.GetTenantByKey(ctx, "acme")
		if err != nil {
			return err
		}
		byID, err := u.GetTenantByID(ctx, f.tenant.ID)
		if err != nil {
			return err
		}
		if byKey.ID != byID.ID {
			t.Fatalf("lookup mismatch: %q vs %q", byKey.ID, byID.ID)
		}
		tenants, err := u.ListTenants(ctx, core.Page{})
		if err != nil {
			return err
		}
		if len(tenants) != 1 {
			t.Fatalf("tenants = %d, want 1", len(tenants))
		}
		byKey.Name = "Acme Renamed"
		if err := u.UpdateTenant(ctx, byKey); err != nil {
			return err
		}
		if _, err := u.GetTenantByKey(ctx, "nope"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("missing tenant = %v, want not found", err)
		}
		dup := core.Tenant{Key: "acme", Name: "Duplicate"}
		if err := u.CreateTenant(ctx, &dup); !core.IsKind(err, core.KindConflict) {
			t.Fatalf("duplicate tenant key = %v, want conflict", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("unscoped operations: %v", err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		d := core.Domain{Hostname: "acme.example"}
		return tx.AddDomain(ctx, &d)
	}); err != nil {
		t.Fatalf("adding domain: %v", err)
	}

	if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
		got, err := u.ResolveDomain(ctx, "acme.example")
		if err != nil {
			return err
		}
		if got.ID != f.tenant.ID {
			t.Fatalf("resolved tenant = %q, want %q", got.ID, f.tenant.ID)
		}
		if _, err := u.ResolveDomain(ctx, "unknown.example"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("resolving an unknown host = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("resolving domain: %v", err)
	}

	if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
		if err := u.DeleteTenant(ctx, f.tenant.ID); err != nil {
			return err
		}
		if err := u.DeleteTenant(ctx, f.tenant.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("deleting twice = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("deleting tenant: %v", err)
	}
}

func TestProjectAndFieldDefRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		byKey, err := tx.GetProject(ctx, "alpha")
		if err != nil {
			return err
		}
		byID, err := tx.GetProject(ctx, f.project.ID)
		if err != nil {
			return err
		}
		if byKey.ID != byID.ID {
			t.Fatalf("project lookup mismatch")
		}
		byKey.Description = "the first project"
		if err := tx.UpdateProject(ctx, byKey); err != nil {
			return err
		}
		reloaded, err := tx.GetProject(ctx, "alpha")
		if err != nil {
			return err
		}
		if reloaded.Description != "the first project" {
			t.Fatalf("description = %q", reloaded.Description)
		}

		dup := core.Project{Key: "alpha", Name: "Dup", WorkflowID: f.workflow.ID}
		if err := tx.CreateProject(ctx, &dup); !core.IsKind(err, core.KindConflict) {
			t.Fatalf("duplicate project key = %v, want conflict", err)
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
		all, err := tx.ListProjects(ctx, core.ProjectFilter{IncludeArchived: true})
		if err != nil {
			return err
		}
		if len(all) != 2 {
			t.Fatalf("all projects = %d, want 2", len(all))
		}
		filtered, err := tx.ListProjects(ctx, core.ProjectFilter{Keys: []string{"beta"}, IncludeArchived: true})
		if err != nil {
			return err
		}
		if len(filtered) != 1 || filtered[0].Key != "beta" {
			t.Fatalf("filtered projects = %+v", filtered)
		}

		def := core.FieldDef{
			ProjectID: f.project.ID, Key: "severity", Label: "Severity", Type: core.FieldEnum,
			Required: true, EnumOptions: []string{"low", "high"}, Default: "low", Indexed: true, Position: 2,
		}
		if err := tx.PutFieldDef(ctx, &def); err != nil {
			return err
		}
		if def.ID == "" {
			t.Fatal("field definition did not get an id")
		}
		def.Label = "Sev"
		def.ID = ""
		if err := tx.PutFieldDef(ctx, &def); err != nil {
			return err
		}
		defs, err := tx.ListFieldDefs(ctx, f.project.ID)
		if err != nil {
			return err
		}
		if len(defs) != 1 {
			t.Fatalf("field definitions = %d, want 1", len(defs))
		}
		if defs[0].Label != "Sev" || len(defs[0].EnumOptions) != 2 || !defs[0].Required || !defs[0].Indexed {
			t.Fatalf("field definition round trip = %+v", defs[0])
		}
		if defs[0].Default != "low" {
			t.Fatalf("field default = %v, want %q", defs[0].Default, "low")
		}
		if err := tx.DeleteFieldDef(ctx, f.project.ID, "severity"); err != nil {
			return err
		}
		if err := tx.DeleteFieldDef(ctx, f.project.ID, "severity"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("deleting a missing field = %v, want not found", err)
		}
		if err := tx.DeleteProject(ctx, archived.ID); err != nil {
			return err
		}
		if err := tx.DeleteProject(ctx, archived.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("deleting a missing project = %v, want not found", err)
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
		if w.Definition.Initial != "todo" || len(w.Definition.States) != 3 {
			t.Fatalf("definition round trip = %+v", w.Definition)
		}
		byID, err := tx.GetWorkflowByID(ctx, w.ID)
		if err != nil {
			return err
		}
		if byID.Key != "default" {
			t.Fatalf("workflow by id = %+v", byID)
		}
		w.Name = "Renamed"
		if err := tx.PutWorkflow(ctx, w); err != nil {
			return err
		}
		list, err := tx.ListWorkflows(ctx)
		if err != nil {
			return err
		}
		if len(list) != 1 || list[0].Name != "Renamed" {
			t.Fatalf("workflows = %+v", list)
		}
		counts, err := tx.CountTasksInStates(ctx, w.ID, []string{"todo", "done"})
		if err != nil {
			return err
		}
		if counts["todo"] != 1 || counts["done"] != 0 {
			t.Fatalf("counts = %+v", counts)
		}
		if _, err := tx.GetWorkflow(ctx, "missing"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("missing workflow = %v, want not found", err)
		}
		if err := tx.DeleteWorkflow(ctx, "missing"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("deleting a missing workflow = %v, want not found", err)
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
		ProjectID:      f.project.ID,
		Title:          "write the store",
		Body:           "with tests",
		Status:         "todo",
		Priority:       core.PriorityHigh,
		CreatorActorID: f.actor.ID,
		DueAt:          &due,
		CustomFields:   map[string]any{"severity": "high"},
	}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.CreateTask(ctx, &task)
	}); err != nil {
		t.Fatalf("creating task: %v", err)
	}
	if task.Seq != 1 || task.Ref != "alpha-1" {
		t.Fatalf("seq = %d, ref = %q", task.Seq, task.Ref)
	}

	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		byID, err := tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		if err != nil {
			return err
		}
		byRef, err := tx.GetTask(ctx, core.MustParseTaskRef("alpha-1"))
		if err != nil {
			return err
		}
		if byID.ID != byRef.ID {
			t.Fatal("task lookup by id and by ref disagree")
		}
		if byID.CustomFields["severity"] != "high" {
			t.Fatalf("custom fields = %+v", byID.CustomFields)
		}
		if byID.DueAt == nil || !byID.DueAt.Equal(due) {
			t.Fatalf("due = %v, want %v", byID.DueAt, due)
		}
		if byID.Blocked {
			t.Fatal("a task with no dependencies must not be blocked")
		}
		if _, err := tx.GetTask(ctx, core.TaskRef{}); !core.IsKind(err, core.KindInvalid) {
			t.Fatalf("empty ref = %v, want invalid", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading task: %v", err)
	}

	child := core.Task{ProjectID: f.project.ID, Title: "child", Status: "todo",
		Priority: core.PriorityNormal, CreatorActorID: f.actor.ID, ParentID: task.ID}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.CreateTask(ctx, &child); err != nil {
			return err
		}
		task.Title = "write the sqlite store"
		task.Status = "doing"
		if err := tx.UpdateTask(ctx, &task); err != nil {
			return err
		}
		if task.Version != 2 {
			t.Fatalf("version = %d, want 2", task.Version)
		}
		return nil
	}); err != nil {
		t.Fatalf("updating task: %v", err)
	}

	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		kids, err := tx.Children(ctx, task.ID)
		if err != nil {
			return err
		}
		if len(kids) != 1 || kids[0].ID != child.ID {
			t.Fatalf("children = %+v", kids)
		}
		all, err := tx.ListTasks(ctx, core.TaskFilter{})
		if err != nil {
			return err
		}
		if len(all) != 2 {
			t.Fatalf("tasks = %d, want 2", len(all))
		}
		doing, err := tx.ListTasks(ctx, core.TaskFilter{Statuses: []string{"doing"}})
		if err != nil {
			return err
		}
		if len(doing) != 1 || doing[0].ID != task.ID {
			t.Fatalf("filtered by status = %+v", doing)
		}
		roots, err := tx.ListTasks(ctx, core.TaskFilter{ParentIsNull: true})
		if err != nil {
			return err
		}
		if len(roots) != 1 || roots[0].ID != task.ID {
			t.Fatalf("roots = %+v", roots)
		}
		kidsOnly, err := tx.ListTasks(ctx, core.TaskFilter{ParentID: task.ID})
		if err != nil {
			return err
		}
		if len(kidsOnly) != 1 {
			t.Fatalf("children filter = %+v", kidsOnly)
		}
		found, err := tx.ListTasks(ctx, core.TaskFilter{Query: "sqlite"})
		if err != nil {
			return err
		}
		if len(found) != 1 || found[0].ID != task.ID {
			t.Fatalf("substring search = %+v", found)
		}
		byField, err := tx.ListTasks(ctx, core.TaskFilter{CustomFields: map[string]any{"severity": "high"}})
		if err != nil {
			return err
		}
		if len(byField) != 1 {
			t.Fatalf("custom field filter = %+v", byField)
		}
		if _, err := tx.ListTasks(ctx, core.TaskFilter{CustomFields: map[string]any{"bad key": 1}}); !core.IsKind(err, core.KindInvalid) {
			t.Fatal("an unsafe custom field key must be rejected")
		}
		byPriority, err := tx.ListTasks(ctx, core.TaskFilter{Priorities: []core.Priority{core.PriorityHigh}})
		if err != nil {
			return err
		}
		if len(byPriority) != 1 || byPriority[0].ID != task.ID {
			t.Fatalf("priority filter = %+v", byPriority)
		}
		byCreator, err := tx.ListTasks(ctx, core.TaskFilter{CreatorIDs: []string{f.actor.ID}})
		if err != nil {
			return err
		}
		if len(byCreator) != 2 {
			t.Fatalf("creator filter = %d, want 2", len(byCreator))
		}
		soon := due.Add(time.Hour)
		byDue, err := tx.ListTasks(ctx, core.TaskFilter{DueBefore: &soon})
		if err != nil {
			return err
		}
		if len(byDue) != 1 {
			t.Fatalf("due filter = %+v", byDue)
		}
		return nil
	}); err != nil {
		t.Fatalf("listing tasks: %v", err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.DeleteTask(ctx, child.ID, false); err != nil {
			return err
		}
		live, err := tx.ListTasks(ctx, core.TaskFilter{})
		if err != nil {
			return err
		}
		if len(live) != 1 {
			t.Fatalf("live tasks after soft delete = %d, want 1", len(live))
		}
		withDeleted, err := tx.ListTasks(ctx, core.TaskFilter{IncludeDeleted: true})
		if err != nil {
			return err
		}
		if len(withDeleted) != 2 {
			t.Fatalf("tasks including deleted = %d, want 2", len(withDeleted))
		}
		if err := tx.RestoreTask(ctx, child.ID); err != nil {
			return err
		}
		if err := tx.RestoreTask(ctx, child.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("restoring a live task = %v, want not found", err)
		}
		if err := tx.DeleteTask(ctx, child.ID, true); err != nil {
			return err
		}
		if err := tx.DeleteTask(ctx, child.ID, true); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("hard deleting twice = %v, want not found", err)
		}
		missing := core.Task{ID: "MISSINGMISSINGMISSING12345", ProjectID: f.project.ID,
			Title: "x", Status: "todo", Priority: core.PriorityNormal, CreatorActorID: f.actor.ID}
		if err := tx.UpdateTask(ctx, &missing); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("updating a missing task = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("deleting tasks: %v", err)
	}
}

func TestLabelCommentArtifactRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	task := f.newTask(t, "labelled", core.PriorityNormal)

	label := core.Label{Name: "urgent", Color: "red"}
	comment := core.Comment{TaskID: task.ID, AuthorActorID: f.actor.ID, Body: "first"}
	artifact := core.Artifact{TaskID: task.ID, ActorID: f.actor.ID, Kind: core.ArtifactResult,
		Name: "result", Payload: map[string]any{"ok": true}, Blob: []byte("bytes")}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.PutLabel(ctx, &label); err != nil {
			return err
		}
		label.Color = "crimson"
		label.ID = ""
		if err := tx.PutLabel(ctx, &label); err != nil {
			return err
		}
		labels, err := tx.ListLabels(ctx)
		if err != nil {
			return err
		}
		if len(labels) != 1 || labels[0].Color != "crimson" {
			t.Fatalf("labels = %+v", labels)
		}
		if err := tx.AttachLabel(ctx, task.ID, label.ID); err != nil {
			return err
		}
		if err := tx.AttachLabel(ctx, task.ID, label.ID); err != nil {
			t.Fatalf("attaching twice should be idempotent: %v", err)
		}

		if err := tx.CreateComment(ctx, &comment); err != nil {
			return err
		}
		comment.Body = "edited"
		if err := tx.UpdateComment(ctx, &comment); err != nil {
			return err
		}
		if err := tx.PutArtifact(ctx, &artifact); err != nil {
			return err
		}
		artifact.Name = "result-2"
		if err := tx.PutArtifact(ctx, &artifact); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("attaching to task: %v", err)
	}

	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		got, err := tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		if err != nil {
			return err
		}
		if len(got.Labels) != 1 || got.Labels[0] != "urgent" {
			t.Fatalf("task labels = %+v", got.Labels)
		}
		byLabel, err := tx.ListTasks(ctx, core.TaskFilter{Labels: []string{"urgent"}})
		if err != nil {
			return err
		}
		if len(byLabel) != 1 {
			t.Fatalf("label filter = %+v", byLabel)
		}
		comments, err := tx.ListComments(ctx, task.ID)
		if err != nil {
			return err
		}
		if len(comments) != 1 || comments[0].Body != "edited" {
			t.Fatalf("comments = %+v", comments)
		}
		one, err := tx.GetComment(ctx, comment.ID)
		if err != nil {
			return err
		}
		if one.ID != comment.ID {
			t.Fatal("comment lookup mismatch")
		}
		artifacts, err := tx.ListArtifacts(ctx, task.ID)
		if err != nil {
			return err
		}
		if len(artifacts) != 1 || artifacts[0].Name != "result-2" {
			t.Fatalf("artifacts = %+v", artifacts)
		}
		if artifacts[0].Payload["ok"] != true || string(artifacts[0].Blob) != "bytes" {
			t.Fatalf("artifact payload = %+v", artifacts[0])
		}
		return nil
	}); err != nil {
		t.Fatalf("reading attachments: %v", err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.DetachLabel(ctx, task.ID, label.ID); err != nil {
			return err
		}
		if err := tx.DetachLabel(ctx, task.ID, label.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("detaching twice = %v, want not found", err)
		}
		if err := tx.DeleteComment(ctx, comment.ID); err != nil {
			return err
		}
		if err := tx.DeleteComment(ctx, comment.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("deleting a comment twice = %v, want not found", err)
		}
		if _, err := tx.GetComment(ctx, "MISSINGMISSINGMISSING12345"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("missing comment = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("detaching: %v", err)
	}
}

func TestAuthRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	user := core.User{Email: "person@example.com", DisplayName: "Person"}
	token := core.APIToken{ActorID: f.actor.ID, Name: "ci", Scopes: []core.Scope{core.ScopeTaskRead}}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.CreateUser(ctx, &user, "hash-1"); err != nil {
			return err
		}
		byID, err := tx.GetUser(ctx, user.ID)
		if err != nil {
			return err
		}
		if byID.Email != user.Email {
			t.Fatal("user lookup mismatch")
		}
		user.DisplayName = "Renamed"
		if err := tx.UpdateUser(ctx, &user, "hash-2"); err != nil {
			return err
		}
		users, err := tx.ListUsers(ctx, core.Page{})
		if err != nil {
			return err
		}
		if len(users) != 1 {
			t.Fatalf("users = %d, want 1", len(users))
		}
		actors, err := tx.ListActors(ctx, core.Page{})
		if err != nil {
			return err
		}
		if len(actors) != 1 {
			t.Fatalf("actors = %d, want 1", len(actors))
		}
		byHandle, err := tx.GetActorByHandle(ctx, "worker")
		if err != nil {
			return err
		}
		if byHandle.ID != f.actor.ID {
			t.Fatal("actor lookup mismatch")
		}
		if _, err := tx.GetActor(ctx, "MISSINGMISSINGMISSING12345"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("missing actor = %v, want not found", err)
		}
		dup := core.Actor{Kind: core.ActorAgent, Handle: "worker"}
		if err := tx.CreateActor(ctx, &dup); !core.IsKind(err, core.KindConflict) {
			t.Fatalf("duplicate handle = %v, want conflict", err)
		}

		if err := tx.CreateSession(ctx, f.actor.ID, "session-hash", clk.Now().Add(time.Hour)); err != nil {
			return err
		}
		actorID, expires, err := tx.GetSessionByHash(ctx, "session-hash")
		if err != nil {
			return err
		}
		if actorID != f.actor.ID || !expires.After(clk.Now()) {
			t.Fatalf("session = %q %v", actorID, expires)
		}
		if _, _, err := tx.GetSessionByHash(ctx, "nope"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("missing session = %v, want not found", err)
		}

		if err := tx.CreateToken(ctx, &token, "token-hash"); err != nil {
			return err
		}
		fetched, err := tx.GetTokenByHash(ctx, "token-hash")
		if err != nil {
			return err
		}
		if len(fetched.Scopes) != 1 || fetched.Scopes[0] != core.ScopeTaskRead {
			t.Fatalf("token scopes = %+v", fetched.Scopes)
		}
		if !fetched.Active(clk.Now()) {
			t.Fatal("a fresh token should be active")
		}
		if err := tx.TouchToken(ctx, token.ID, clk.Now()); err != nil {
			return err
		}
		if err := tx.RevokeToken(ctx, token.ID, clk.Now()); err != nil {
			return err
		}
		tokens, err := tx.ListTokens(ctx, f.actor.ID)
		if err != nil {
			return err
		}
		if len(tokens) != 1 || tokens[0].RevokedAt == nil || tokens[0].LastUsedAt == nil {
			t.Fatalf("tokens = %+v", tokens)
		}
		if tokens[0].Active(clk.Now()) {
			t.Fatal("a revoked token must not be active")
		}
		if err := tx.RevokeToken(ctx, "MISSINGMISSINGMISSING12345", clk.Now()); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("revoking a missing token = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("auth round trip: %v", err)
	}

	if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
		got, hash, err := u.GetUserByEmail(ctx, "person@example.com")
		if err != nil {
			return err
		}
		if got.ID != user.ID || hash != "hash-2" {
			t.Fatalf("user by email = %+v hash %q", got, hash)
		}
		if got.DisplayName != "Renamed" {
			t.Fatalf("display name = %q, want Renamed", got.DisplayName)
		}
		if _, _, err := u.GetUserByEmail(ctx, "nobody@example.com"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("missing user = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("user by email: %v", err)
	}

	clk.Advance(2 * time.Hour)
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		n, err := tx.DeleteExpiredSessions(ctx, clk.Now())
		if err != nil {
			return err
		}
		if n != 1 {
			t.Fatalf("expired sessions removed = %d, want 1", n)
		}
		if err := tx.DeleteSession(ctx, "session-hash"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("deleting a gone session = %v, want not found", err)
		}
		if err := tx.DeleteUser(ctx, user.ID); err != nil {
			return err
		}
		if err := tx.DeleteUser(ctx, user.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("deleting a gone user = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("session cleanup: %v", err)
	}
}

func TestEventAuditAndRetentionRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	task := f.newTask(t, "eventful", core.PriorityNormal)

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		seq, err := tx.LatestEventSeq(ctx)
		if err != nil {
			return err
		}
		if seq != 0 {
			t.Fatalf("latest seq on an empty outbox = %d, want 0", seq)
		}
		for i := 0; i < 3; i++ {
			e := core.Event{
				Type: core.EventTaskCreated, ProjectID: f.project.ID,
				SubjectType: "task", SubjectID: task.ID, ActorID: f.actor.ID,
				Payload: map[string]any{"n": float64(i)},
			}
			if err := tx.AppendEvent(ctx, &e); err != nil {
				return err
			}
			if e.Seq == 0 {
				t.Fatal("event did not receive a sequence number")
			}
		}
		events, err := tx.ReadEvents(ctx, 0, 10)
		if err != nil {
			return err
		}
		if len(events) != 3 {
			t.Fatalf("events = %d, want 3", len(events))
		}
		if events[0].Payload["n"] != float64(0) || events[0].ActorID != f.actor.ID {
			t.Fatalf("event round trip = %+v", events[0])
		}
		after, err := tx.ReadEvents(ctx, events[0].Seq, 10)
		if err != nil {
			return err
		}
		if len(after) != 2 {
			t.Fatalf("events after the first = %d, want 2", len(after))
		}
		latest, err := tx.LatestEventSeq(ctx)
		if err != nil {
			return err
		}
		if latest != events[2].Seq {
			t.Fatalf("latest seq = %d, want %d", latest, events[2].Seq)
		}

		entry := core.AuditEntry{
			ActorID: f.actor.ID, Action: "task.create", SubjectType: "task", SubjectID: task.ID,
			After: json.RawMessage(`{"title":"eventful"}`), Source: core.SourceCLI,
		}
		if err := tx.AppendAudit(ctx, &entry); err != nil {
			return err
		}
		entries, err := tx.ListAudit(ctx, core.AuditFilter{SubjectType: "task", SubjectID: task.ID})
		if err != nil {
			return err
		}
		if len(entries) != 1 || string(entries[0].After) != `{"title":"eventful"}` {
			t.Fatalf("audit entries = %+v", entries)
		}
		byActor, err := tx.ListAudit(ctx, core.AuditFilter{
			ActorIDs: []string{f.actor.ID},
			Actions:  []string{"task.create"},
			Sources:  []core.Source{core.SourceCLI},
		})
		if err != nil {
			return err
		}
		if len(byActor) != 1 {
			t.Fatalf("audit by actor = %+v", byActor)
		}
		since := clk.Now().Add(-time.Hour)
		until := clk.Now().Add(time.Hour)
		window, err := tx.ListAudit(ctx, core.AuditFilter{Since: &since, Until: &until})
		if err != nil {
			return err
		}
		if len(window) != 1 {
			t.Fatalf("audit window = %+v", window)
		}

		policy, err := tx.GetRetention(ctx)
		if err != nil {
			return err
		}
		if policy.Events != core.DefaultRetention(f.tenant.ID).Events {
			t.Fatalf("default retention = %+v", policy)
		}
		policy.Events = core.Duration(time.Hour)
		if err := tx.PutRetention(ctx, policy); err != nil {
			return err
		}
		if err := tx.PutRetention(ctx, policy); err != nil {
			return err
		}
		stored, err := tx.GetRetention(ctx)
		if err != nil {
			return err
		}
		if stored.Events != core.Duration(time.Hour) {
			t.Fatalf("stored retention = %+v", stored)
		}
		return nil
	}); err != nil {
		t.Fatalf("event round trip: %v", err)
	}

	clk.Advance(72 * time.Hour)
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		latest, err := tx.LatestEventSeq(ctx)
		if err != nil {
			return err
		}
		n, err := tx.PruneEvents(ctx, clk.Now(), latest-1, 0)
		if err != nil {
			return err
		}
		if n != 2 {
			t.Fatalf("pruned events = %d, want 2", n)
		}
		left, err := tx.ReadEvents(ctx, 0, 10)
		if err != nil {
			return err
		}
		if len(left) != 1 {
			t.Fatalf("events left = %d, want 1", len(left))
		}
		audited, err := tx.PruneAudit(ctx, clk.Now(), 0)
		if err != nil {
			return err
		}
		if audited != 1 {
			t.Fatalf("pruned audit entries = %d, want 1", audited)
		}
		return nil
	}); err != nil {
		t.Fatalf("pruning: %v", err)
	}
}

func TestWebhookRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	endpoint := core.WebhookEndpoint{URL: "https://example.test/hook", Secret: "shh", Active: true}
	var delivery core.WebhookDelivery

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.PutWebhook(ctx, &endpoint); err != nil {
			return err
		}
		endpoint.URL = "https://example.test/hook2"
		if err := tx.PutWebhook(ctx, &endpoint); err != nil {
			return err
		}
		list, err := tx.ListWebhooks(ctx)
		if err != nil {
			return err
		}
		if len(list) != 1 || list[0].URL != "https://example.test/hook2" {
			t.Fatalf("webhooks = %+v", list)
		}
		if len(list[0].EventTypes) != 1 || list[0].EventTypes[0] != "*" {
			t.Fatalf("event types = %+v", list[0].EventTypes)
		}
		got, err := tx.GetWebhook(ctx, endpoint.ID)
		if err != nil {
			return err
		}
		if !got.Active {
			t.Fatal("endpoint should be active")
		}
		delivery = core.WebhookDelivery{EndpointID: endpoint.ID, EventSeq: 1}
		return tx.EnqueueDelivery(ctx, &delivery)
	}); err != nil {
		t.Fatalf("webhook setup: %v", err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		claimed, err := tx.ClaimDeliveries(ctx, "worker-1", clk.Now(), clk.Now().Add(time.Minute), 10)
		if err != nil {
			return err
		}
		if len(claimed) != 1 || claimed[0].ID != delivery.ID {
			t.Fatalf("claimed deliveries = %+v", claimed)
		}
		again, err := tx.ClaimDeliveries(ctx, "worker-2", clk.Now(), clk.Now().Add(time.Minute), 10)
		if err != nil {
			return err
		}
		if len(again) != 0 {
			t.Fatalf("a locked delivery was claimed twice: %+v", again)
		}
		if err := tx.MarkFailed(ctx, delivery.ID, 500, "boom", clk.Now(), false); err != nil {
			return err
		}
		got, err := tx.GetDelivery(ctx, delivery.ID)
		if err != nil {
			return err
		}
		if got.Attempts != 1 || got.Status != core.DeliveryPending || got.LastError != "boom" {
			t.Fatalf("delivery after failure = %+v", got)
		}
		if err := tx.MarkDelivered(ctx, delivery.ID, 200, clk.Now()); err != nil {
			return err
		}
		done, err := tx.ListDeliveries(ctx, core.DeliveryFilter{
			EndpointID: endpoint.ID,
			Statuses:   []core.DeliveryStatus{core.DeliveryDelivered},
		})
		if err != nil {
			return err
		}
		if len(done) != 1 || done[0].LastStatusCode != 200 {
			t.Fatalf("delivered = %+v", done)
		}
		return nil
	}); err != nil {
		t.Fatalf("delivery lifecycle: %v", err)
	}

	clk.Advance(72 * time.Hour)
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		n, err := tx.PruneDeliveries(ctx, clk.Now(), 0)
		if err != nil {
			return err
		}
		if n != 1 {
			t.Fatalf("pruned deliveries = %d, want 1", n)
		}
		if err := tx.DeleteWebhook(ctx, endpoint.ID); err != nil {
			return err
		}
		if err := tx.DeleteWebhook(ctx, endpoint.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("deleting twice = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("webhook cleanup: %v", err)
	}
}

func TestSyncRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	task := f.newTask(t, "imported", core.PriorityNormal)

	src := core.SyncSource{System: "github", Name: "main", Cursor: "c1", LastStatus: "ok"}
	ref := core.ExternalRef{EntityType: "task", EntityID: task.ID, System: "github",
		ExternalID: "42", ExternalURL: "https://example.test/42", ExternalVersion: "v1"}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.PutSyncSource(ctx, &src); err != nil {
			return err
		}
		src.Cursor = "c2"
		src.ID = ""
		if err := tx.PutSyncSource(ctx, &src); err != nil {
			return err
		}
		got, err := tx.GetSyncSource(ctx, src.ID)
		if err != nil {
			return err
		}
		if got.Cursor != "c2" {
			t.Fatalf("cursor = %q, want c2", got.Cursor)
		}
		sources, err := tx.ListSyncSources(ctx)
		if err != nil {
			return err
		}
		if len(sources) != 1 {
			t.Fatalf("sources = %d, want 1", len(sources))
		}

		if err := tx.PutExternalRef(ctx, &ref); err != nil {
			return err
		}
		ref.ExternalVersion = "v2"
		if err := tx.PutExternalRef(ctx, &ref); err != nil {
			return err
		}
		fetched, err := tx.GetExternalRef(ctx, "github", "42", "task")
		if err != nil {
			return err
		}
		if fetched.ExternalVersion != "v2" {
			t.Fatalf("external ref = %+v", fetched)
		}
		refs, err := tx.ListExternalRefs(ctx, "github")
		if err != nil {
			return err
		}
		if len(refs) != 1 {
			t.Fatalf("external refs = %d, want 1", len(refs))
		}
		if _, err := tx.GetExternalRef(ctx, "github", "99", "task"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("missing external ref = %v, want not found", err)
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
