// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"
	"io"
	"sync"

	"github.com/heliopsy/tix/internal/core"
)

// fakeService is a core.Service whose task methods are scripted by the test.
type fakeService struct {
	mu sync.Mutex

	projects  []core.Project
	workflows []core.Workflow
	tasks     []core.Task
	events    chan core.Event

	claimErr      error
	releaseErr    error
	transitionErr error
	listErr       error

	claimed     []core.TaskRef
	released    []core.TaskRef
	transitions []core.TransitionInput

	created  []core.CreateTaskInput
	updated  []core.UpdateTaskInput
	comments []string
	tagged   []string
	untagged []string
	deps     []core.TaskRef

	// what the destructive and corrective calls recorded, and what the reads
	// the forms offer from are scripted to answer with.
	deleted        []core.DeleteTaskInput
	deletedRefs    []core.TaskRef
	depsRemoved    []core.TaskRef
	commentsEdited [][2]string
	commentsGone   []string
	thread         []core.Comment
	dependencies   []core.Dependency
	tags           []core.Tag
	deleteErr      error
	tagsErr        error

	createErr error
	updateErr error

	claimedNext  bool
	renewed      int
	projectsMade []core.CreateProjectInput

	actors        map[string]*core.Actor
	actorsQueried []string

	whoAmI    *core.Actor
	whoAmIErr error

	// what the project screen reads, and what its actions recorded. The
	// workflow is looked up by whatever reference it was asked for, so a test
	// can prove the screen asked by identifier rather than by key.
	project        *core.Project
	projectErr     error
	fieldDefs      []core.FieldDef
	workflowAsked  []string
	workflowErr    error
	projectsEdited []core.UpdateProjectInput
	projectsGone   []string
	archived       []string
	fieldsPut      []core.FieldDefInput
	fieldsGone     [][2]string
	fieldErr       error
}

func newFakeService() *fakeService {
	return &fakeService{events: make(chan core.Event, 8)}
}

func (f *fakeService) ListProjects(context.Context, core.ProjectFilter) ([]core.Project, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.projects, "", nil
}

func (f *fakeService) ListWorkflows(context.Context) ([]core.Workflow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.workflows, nil
}

func (f *fakeService) ListTasks(context.Context, core.TaskFilter) (core.TaskPage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return core.TaskPage{}, f.listErr
	}
	return core.TaskPage{Tasks: f.tasks}, nil
}

func (f *fakeService) ClaimTask(_ context.Context, ref core.TaskRef, _ core.ClaimInput) (*core.Claim, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claimed = append(f.claimed, ref)
	if f.claimErr != nil {
		return nil, f.claimErr
	}
	return &core.Claim{LeaseToken: "token-" + ref.ID}, nil
}

func (f *fakeService) ReleaseLease(_ context.Context, ref core.TaskRef, _ string, _ core.ReleaseInput) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.released = append(f.released, ref)
	return f.releaseErr
}

func (f *fakeService) TransitionTask(_ context.Context, _ core.TaskRef, in core.TransitionInput) (*core.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.transitions = append(f.transitions, in)
	if f.transitionErr != nil {
		return nil, f.transitionErr
	}
	return &core.Task{}, nil
}

func (f *fakeService) Subscribe(context.Context, core.EventFilter) (<-chan core.Event, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.events, nil
}

func (f *fakeService) WhoAmI(context.Context) (*core.Actor, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.whoAmIErr != nil {
		return nil, f.whoAmIErr
	}
	if f.whoAmI != nil {
		return f.whoAmI, nil
	}
	return &core.Actor{}, nil
}
func (f *fakeService) Close() error { return nil }

func (f *fakeService) CreateTenant(context.Context, core.CreateTenantInput) (*core.Tenant, error) {
	return nil, nil
}
func (f *fakeService) GetTenant(context.Context, string) (*core.Tenant, error) { return nil, nil }
func (f *fakeService) ListTenants(context.Context, core.Page) ([]core.Tenant, string, error) {
	return nil, "", nil
}
func (f *fakeService) Stats(context.Context, core.StatsInput) (*core.Stats, error) {
	return &core.Stats{}, nil
}
func (f *fakeService) UpdateTenant(context.Context, string, core.UpdateTenantInput) (*core.Tenant, error) {
	return nil, nil
}
func (f *fakeService) DeleteTenant(context.Context, string) error { return nil }
func (f *fakeService) AddDomain(context.Context, core.AddDomainInput) (*core.Domain, error) {
	return nil, nil
}
func (f *fakeService) ListDomains(context.Context) ([]core.Domain, error)          { return nil, nil }
func (f *fakeService) RemoveDomain(context.Context, string) error                  { return nil }
func (f *fakeService) ResolveDomain(context.Context, string) (*core.Tenant, error) { return nil, nil }
func (f *fakeService) AddMember(context.Context, string, core.Role) (*core.Membership, error) {
	return nil, nil
}
func (f *fakeService) ListMembers(context.Context) ([]core.Membership, error) { return nil, nil }
func (f *fakeService) RemoveMember(context.Context, string) error             { return nil }

func (f *fakeService) CreateProject(_ context.Context, in core.CreateProjectInput) (*core.Project, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.projectsMade = append(f.projectsMade, in)
	return &core.Project{Key: in.Key, Name: in.Name}, nil
}
func (f *fakeService) GetProject(_ context.Context, ref string) (*core.Project, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.projectErr != nil {
		return nil, f.projectErr
	}
	if f.project != nil {
		return f.project, nil
	}
	for _, p := range f.projects {
		if p.Key == ref || p.ID == ref {
			found := p
			return &found, nil
		}
	}
	return nil, core.NotFound("project %q", ref)
}
func (f *fakeService) UpdateProject(_ context.Context, _ string, in core.UpdateProjectInput) (*core.Project, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.projectsEdited = append(f.projectsEdited, in)
	return &core.Project{}, nil
}
func (f *fakeService) ArchiveProject(_ context.Context, ref string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.archived = append(f.archived, ref)
	return nil
}
func (f *fakeService) DeleteProject(_ context.Context, ref string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.projectsGone = append(f.projectsGone, ref)
	return nil
}
func (f *fakeService) PutFieldDef(_ context.Context, _ string, in core.FieldDefInput) (*core.FieldDef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fieldsPut = append(f.fieldsPut, in)
	if f.fieldErr != nil {
		return nil, f.fieldErr
	}
	return &core.FieldDef{Key: in.Key}, nil
}
func (f *fakeService) ListFieldDefs(context.Context, string) ([]core.FieldDef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fieldDefs, nil
}
func (f *fakeService) DeleteFieldDef(_ context.Context, ref, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fieldsGone = append(f.fieldsGone, [2]string{ref, key})
	return f.fieldErr
}

func (f *fakeService) PutWorkflow(context.Context, core.WorkflowInput) (*core.Workflow, error) {
	return nil, nil
}
func (f *fakeService) GetWorkflow(_ context.Context, ref string) (*core.Workflow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.workflowAsked = append(f.workflowAsked, ref)
	if f.workflowErr != nil {
		return nil, f.workflowErr
	}
	for _, w := range f.workflows {
		if w.Key == ref || w.ID == ref {
			found := w
			return &found, nil
		}
	}
	return nil, core.NotFound("workflow %q", ref)
}
func (f *fakeService) DeleteWorkflow(context.Context, string) error { return nil }

func (f *fakeService) CreateTask(_ context.Context, in core.CreateTaskInput) (*core.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created = append(f.created, in)
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &core.Task{ID: "new", Ref: in.ProjectRef + "-99", Title: in.Title}, nil
}
func (f *fakeService) GetTask(_ context.Context, ref core.TaskRef) (*core.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, t := range f.tasks {
		if t.ID == ref.ID {
			task := t
			return &task, nil
		}
	}
	return nil, core.NotFound("task %q", ref.String())
}
func (f *fakeService) UpdateTask(_ context.Context, _ core.TaskRef, in core.UpdateTaskInput) (*core.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updated = append(f.updated, in)
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return &core.Task{}, nil
}
func (f *fakeService) DeleteTask(_ context.Context, ref core.TaskRef, in core.DeleteTaskInput) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletedRefs = append(f.deletedRefs, ref)
	f.deleted = append(f.deleted, in)
	return f.deleteErr
}
func (f *fakeService) RestoreTask(context.Context, core.TaskRef) (*core.Task, error) { return nil, nil }
func (f *fakeService) TaskTree(context.Context, core.TaskRef, int) ([]core.Task, error) {
	return nil, nil
}
func (f *fakeService) AddDependency(_ context.Context, _ core.TaskRef, on core.TaskRef) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deps = append(f.deps, on)
	return nil
}

func (f *fakeService) RemoveDependency(_ context.Context, _ core.TaskRef, on core.TaskRef) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.depsRemoved = append(f.depsRemoved, on)
	return nil
}
func (f *fakeService) ListDependencies(context.Context, core.TaskRef) ([]core.Dependency, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.dependencies, nil
}
func (f *fakeService) AddTag(_ context.Context, _ core.TaskRef, tag string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tagged = append(f.tagged, tag)
	return nil
}

func (f *fakeService) RemoveTag(_ context.Context, _ core.TaskRef, tag string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.untagged = append(f.untagged, tag)
	return nil
}
func (f *fakeService) ListTags(context.Context) ([]core.Tag, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tags, f.tagsErr
}
func (f *fakeService) AddComment(_ context.Context, _ core.TaskRef, body string) (*core.Comment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.comments = append(f.comments, body)
	return &core.Comment{Body: body}, nil
}
func (f *fakeService) ListComments(context.Context, core.TaskRef) ([]core.Comment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.thread, nil
}
func (f *fakeService) EditComment(_ context.Context, id, body string) (*core.Comment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commentsEdited = append(f.commentsEdited, [2]string{id, body})
	return &core.Comment{ID: id, Body: body}, nil
}
func (f *fakeService) DeleteComment(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commentsGone = append(f.commentsGone, id)
	return nil
}
func (f *fakeService) PutArtifact(context.Context, core.TaskRef, core.ArtifactInput) (*core.Artifact, error) {
	return nil, nil
}
func (f *fakeService) ListArtifacts(context.Context, core.TaskRef) ([]core.Artifact, error) {
	return nil, nil
}

func (f *fakeService) ClaimNext(context.Context, core.ClaimNextInput) (*core.Claim, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claimedNext = true
	return &core.Claim{Task: &core.Task{ID: "next"}, LeaseToken: "token-next"}, nil
}

func (f *fakeService) RenewLease(context.Context, core.TaskRef, string, core.Duration) (*core.Claim, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.renewed++
	return &core.Claim{}, nil
}

func (f *fakeService) SweepLeases(context.Context, int) (int, error) { return 0, nil }

func (f *fakeService) ListAudit(context.Context, core.AuditFilter) ([]core.AuditEntry, string, error) {
	return nil, "", nil
}
func (f *fakeService) Prune(context.Context, core.PruneInput) (*core.PruneResult, error) {
	return nil, nil
}
func (f *fakeService) GetRetention(context.Context) (*core.RetentionPolicy, error) { return nil, nil }
func (f *fakeService) PutRetention(context.Context, core.RetentionPolicy) (*core.RetentionPolicy, error) {
	return nil, nil
}

func (f *fakeService) CreateUser(context.Context, core.CreateUserInput) (*core.User, error) {
	return nil, nil
}
func (f *fakeService) GetActor(_ context.Context, id string) (*core.Actor, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.actorsQueried = append(f.actorsQueried, id)
	if a, ok := f.actors[id]; ok {
		return a, nil
	}
	return nil, core.NotFound("actor %q", id)
}
func (f *fakeService) ListActors(context.Context, core.Page) ([]core.Actor, string, error) {
	return nil, "", nil
}
func (f *fakeService) GetUser(context.Context, string) (*core.User, error) { return nil, nil }
func (f *fakeService) ListUsers(context.Context, core.Page) ([]core.User, string, error) {
	return nil, "", nil
}
func (f *fakeService) UpdateUser(context.Context, string, core.UpdateUserInput) (*core.User, error) {
	return nil, nil
}
func (f *fakeService) DeleteUser(context.Context, string) error { return nil }
func (f *fakeService) Login(context.Context, string, string) (*core.Session, error) {
	return nil, nil
}
func (f *fakeService) Logout(context.Context) error { return nil }
func (f *fakeService) CreateToken(context.Context, core.CreateTokenInput) (*core.IssuedToken, error) {
	return nil, nil
}
func (f *fakeService) ListTokens(context.Context, string) ([]core.APIToken, error) { return nil, nil }
func (f *fakeService) RevokeToken(context.Context, string) error                   { return nil }

func (f *fakeService) ListConnections(context.Context) (*core.ConnectionList, error) {
	return nil, nil
}
func (f *fakeService) EndConnection(context.Context, string) error { return nil }

func (f *fakeService) EnrolSSHKey(context.Context, core.EnrolSSHKeyInput) (*core.SSHKey, error) {
	return nil, nil
}
func (f *fakeService) ListSSHKeys(context.Context, string) ([]core.SSHKey, error) { return nil, nil }
func (f *fakeService) RevokeSSHKey(context.Context, string) error                 { return nil }

func (f *fakeService) PutWebhook(context.Context, core.WebhookInput) (*core.WebhookEndpoint, error) {
	return nil, nil
}
func (f *fakeService) ListWebhooks(context.Context) ([]core.WebhookEndpoint, error) { return nil, nil }
func (f *fakeService) DeleteWebhook(context.Context, string) error                  { return nil }
func (f *fakeService) ListDeliveries(context.Context, core.DeliveryFilter) ([]core.WebhookDelivery, string, error) {
	return nil, "", nil
}
func (f *fakeService) RedeliverWebhook(context.Context, string) error { return nil }

func (f *fakeService) ExportTo(context.Context, core.ExportInput, io.Writer) error { return nil }
func (f *fakeService) ImportFrom(context.Context, io.Reader, core.ImportInput) (*core.ImportResult, error) {
	return nil, nil
}

func (f *fakeService) PutSyncSource(context.Context, core.SyncSourceInput) (*core.SyncSource, error) {
	return nil, nil
}
func (f *fakeService) ListSyncSources(context.Context) ([]core.SyncSource, error) { return nil, nil }
func (f *fakeService) DeleteSyncSource(context.Context, string) error             { return nil }
func (f *fakeService) RunSync(context.Context, core.RunSyncInput) (*core.SyncResult, error) {
	return nil, nil
}

func (f *fakeService) ExportBundle(context.Context, core.BundleExportInput, io.Writer) error {
	return nil
}

func (f *fakeService) ImportBundle(context.Context, io.Reader, core.BundleImportInput) (*core.BundleResult, error) {
	return nil, nil
}

var _ core.Service = (*fakeService)(nil)
