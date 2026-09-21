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

func (f *fakeService) WhoAmI(context.Context) (*core.Actor, error) { return &core.Actor{}, nil }
func (f *fakeService) Close() error                                { return nil }

func (f *fakeService) CreateTenant(context.Context, core.CreateTenantInput) (*core.Tenant, error) {
	return nil, nil
}
func (f *fakeService) GetTenant(context.Context, string) (*core.Tenant, error) { return nil, nil }
func (f *fakeService) ListTenants(context.Context, core.Page) ([]core.Tenant, string, error) {
	return nil, "", nil
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

func (f *fakeService) CreateProject(context.Context, core.CreateProjectInput) (*core.Project, error) {
	return nil, nil
}
func (f *fakeService) GetProject(context.Context, string) (*core.Project, error) { return nil, nil }
func (f *fakeService) UpdateProject(context.Context, string, core.UpdateProjectInput) (*core.Project, error) {
	return nil, nil
}
func (f *fakeService) ArchiveProject(context.Context, string) error { return nil }
func (f *fakeService) DeleteProject(context.Context, string) error  { return nil }
func (f *fakeService) PutFieldDef(context.Context, string, core.FieldDefInput) (*core.FieldDef, error) {
	return nil, nil
}
func (f *fakeService) ListFieldDefs(context.Context, string) ([]core.FieldDef, error) {
	return nil, nil
}
func (f *fakeService) DeleteFieldDef(context.Context, string, string) error { return nil }

func (f *fakeService) PutWorkflow(context.Context, core.WorkflowInput) (*core.Workflow, error) {
	return nil, nil
}
func (f *fakeService) GetWorkflow(context.Context, string) (*core.Workflow, error) { return nil, nil }
func (f *fakeService) DeleteWorkflow(context.Context, string) error                { return nil }

func (f *fakeService) CreateTask(context.Context, core.CreateTaskInput) (*core.Task, error) {
	return nil, nil
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
func (f *fakeService) UpdateTask(context.Context, core.TaskRef, core.UpdateTaskInput) (*core.Task, error) {
	return nil, nil
}
func (f *fakeService) DeleteTask(context.Context, core.TaskRef, core.DeleteTaskInput) error {
	return nil
}
func (f *fakeService) RestoreTask(context.Context, core.TaskRef) (*core.Task, error) { return nil, nil }
func (f *fakeService) TaskTree(context.Context, core.TaskRef, int) ([]core.Task, error) {
	return nil, nil
}
func (f *fakeService) AddDependency(context.Context, core.TaskRef, core.TaskRef) error    { return nil }
func (f *fakeService) RemoveDependency(context.Context, core.TaskRef, core.TaskRef) error { return nil }
func (f *fakeService) ListDependencies(context.Context, core.TaskRef) ([]core.Dependency, error) {
	return nil, nil
}
func (f *fakeService) AddTag(context.Context, core.TaskRef, string) error    { return nil }
func (f *fakeService) RemoveTag(context.Context, core.TaskRef, string) error { return nil }
func (f *fakeService) ListTags(context.Context) ([]core.Tag, error)          { return nil, nil }
func (f *fakeService) AddComment(context.Context, core.TaskRef, string) (*core.Comment, error) {
	return nil, nil
}
func (f *fakeService) ListComments(context.Context, core.TaskRef) ([]core.Comment, error) {
	return nil, nil
}
func (f *fakeService) EditComment(context.Context, string, string) (*core.Comment, error) {
	return nil, nil
}
func (f *fakeService) DeleteComment(context.Context, string) error { return nil }
func (f *fakeService) PutArtifact(context.Context, core.TaskRef, core.ArtifactInput) (*core.Artifact, error) {
	return nil, nil
}
func (f *fakeService) ListArtifacts(context.Context, core.TaskRef) ([]core.Artifact, error) {
	return nil, nil
}

func (f *fakeService) ClaimNext(context.Context, core.ClaimNextInput) (*core.Claim, error) {
	return nil, nil
}
func (f *fakeService) RenewLease(context.Context, core.TaskRef, string, core.Duration) (*core.Claim, error) {
	return nil, nil
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
func (f *fakeService) GetActor(context.Context, string) (*core.Actor, error) { return nil, nil }
func (f *fakeService) GetUser(context.Context, string) (*core.User, error)   { return nil, nil }
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
