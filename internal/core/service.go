package core

import (
	"context"
	"time"
)

// Service is the complete tix product surface.
//
// Every access path goes through exactly this interface. Two implementations
// exist: a local one that owns every business rule, and a remote client that
// marshals to HTTP and owns none. The HTTP handlers wrap the local one, so a
// request arriving over the network executes the same method a direct-database
// command would call. That is what makes local and remote behaviour
// indistinguishable, and it is asserted by the transport-equivalence suite.
//
// Adding a method here requires a matching entry in the capability registry
// with CLI, HTTP and Web bindings, or the parity test fails the build.
type Service interface {
	// WhoAmI returns the authenticated actor and its tenant.
	WhoAmI(ctx context.Context) (*Actor, error)

	TenantService
	ProjectService
	WorkflowService
	TaskService
	ClaimService
	HistoryService
	AuthService
	WebhookService
	TransferService
	SyncService

	// Close releases resources. Closing a remote client does not affect the server.
	Close() error
}

// TenantService covers tenants, their domains, and membership.
type TenantService interface {
	CreateTenant(ctx context.Context, in CreateTenantInput) (*Tenant, error)
	GetTenant(ctx context.Context, ref string) (*Tenant, error)
	ListTenants(ctx context.Context, page Page) ([]Tenant, string, error)
	UpdateTenant(ctx context.Context, ref string, in UpdateTenantInput) (*Tenant, error)
	DeleteTenant(ctx context.Context, ref string) error

	AddDomain(ctx context.Context, in AddDomainInput) (*Domain, error)
	ListDomains(ctx context.Context) ([]Domain, error)
	RemoveDomain(ctx context.Context, hostname string) error
	// ResolveDomain maps a request hostname to its tenant. It runs before
	// authentication, so it takes no actor.
	ResolveDomain(ctx context.Context, hostname string) (*Tenant, error)

	AddMember(ctx context.Context, actorID string, role Role) (*Membership, error)
	ListMembers(ctx context.Context) ([]Membership, error)
	RemoveMember(ctx context.Context, actorID string) error
}

// ProjectService covers projects and their custom field definitions.
type ProjectService interface {
	CreateProject(ctx context.Context, in CreateProjectInput) (*Project, error)
	GetProject(ctx context.Context, ref string) (*Project, error)
	ListProjects(ctx context.Context, f ProjectFilter) ([]Project, string, error)
	UpdateProject(ctx context.Context, ref string, in UpdateProjectInput) (*Project, error)
	ArchiveProject(ctx context.Context, ref string) error
	DeleteProject(ctx context.Context, ref string) error

	PutFieldDef(ctx context.Context, projectRef string, in FieldDefInput) (*FieldDef, error)
	ListFieldDefs(ctx context.Context, projectRef string) ([]FieldDef, error)
	DeleteFieldDef(ctx context.Context, projectRef, key string) error
}

// WorkflowService covers state machine definitions.
type WorkflowService interface {
	PutWorkflow(ctx context.Context, in WorkflowInput) (*Workflow, error)
	GetWorkflow(ctx context.Context, key string) (*Workflow, error)
	ListWorkflows(ctx context.Context) ([]Workflow, error)
	DeleteWorkflow(ctx context.Context, key string) error
}

// TaskService covers tasks and everything attached to them.
type TaskService interface {
	CreateTask(ctx context.Context, in CreateTaskInput) (*Task, error)
	GetTask(ctx context.Context, ref TaskRef) (*Task, error)
	ListTasks(ctx context.Context, f TaskFilter) (TaskPage, error)
	UpdateTask(ctx context.Context, ref TaskRef, in UpdateTaskInput) (*Task, error)
	// TransitionTask moves a task to a new status, enforcing the project's
	// workflow. A task under lease requires the current lease token.
	TransitionTask(ctx context.Context, ref TaskRef, in TransitionInput) (*Task, error)
	DeleteTask(ctx context.Context, ref TaskRef, hard bool) error
	RestoreTask(ctx context.Context, ref TaskRef) (*Task, error)
	// TaskTree returns a task and its descendants.
	TaskTree(ctx context.Context, ref TaskRef, depth int) ([]Task, error)

	AddDependency(ctx context.Context, ref, dependsOn TaskRef) error
	RemoveDependency(ctx context.Context, ref, dependsOn TaskRef) error
	ListDependencies(ctx context.Context, ref TaskRef) ([]Dependency, error)

	AddLabel(ctx context.Context, ref TaskRef, label string) error
	RemoveLabel(ctx context.Context, ref TaskRef, label string) error
	ListLabels(ctx context.Context) ([]Label, error)

	AddComment(ctx context.Context, ref TaskRef, body string) (*Comment, error)
	ListComments(ctx context.Context, ref TaskRef) ([]Comment, error)
	EditComment(ctx context.Context, id, body string) (*Comment, error)
	DeleteComment(ctx context.Context, id string) error

	PutArtifact(ctx context.Context, ref TaskRef, in ArtifactInput) (*Artifact, error)
	ListArtifacts(ctx context.Context, ref TaskRef) ([]Artifact, error)
}

// ClaimService covers the agent work queue.
type ClaimService interface {
	// ClaimTask claims one specific task. It fails with a conflict when the
	// task is held under a live lease.
	ClaimTask(ctx context.Context, ref TaskRef, in ClaimInput) (*Claim, error)
	// ClaimNext atomically claims the highest-priority unblocked task matching
	// the filter. It fails with KindNoTaskAvailable when nothing is eligible,
	// which is an empty result rather than an error condition.
	ClaimNext(ctx context.Context, in ClaimNextInput) (*Claim, error)
	// RenewLease extends a lease. The caller must hold the current token.
	RenewLease(ctx context.Context, ref TaskRef, token string, ttl Duration) (*Claim, error)
	// ReleaseLease gives up a lease, optionally setting a final status and
	// recording a result artifact.
	ReleaseLease(ctx context.Context, ref TaskRef, token string, in ReleaseInput) error
	// SweepLeases materializes expired leases. Expiry is evaluated lazily
	// everywhere else, so this only emits events and reverts status; it is
	// never required for correctness.
	SweepLeases(ctx context.Context, limit int) (int, error)
}

// HistoryService covers the audit log, the event stream, and retention.
type HistoryService interface {
	ListAudit(ctx context.Context, f AuditFilter) ([]AuditEntry, string, error)
	// Subscribe delivers events matching the filter until ctx is cancelled.
	// A filter carrying SinceSeq replays from the durable log, so a reconnect
	// is gap-free.
	Subscribe(ctx context.Context, f EventFilter) (<-chan Event, error)
	// Prune removes records past their retention window. It never removes an
	// event at or below a live subscriber's cursor.
	Prune(ctx context.Context, in PruneInput) (*PruneResult, error)
	GetRetention(ctx context.Context) (*RetentionPolicy, error)
	PutRetention(ctx context.Context, p RetentionPolicy) (*RetentionPolicy, error)
}

// AuthService covers users, sessions and API tokens.
type AuthService interface {
	CreateUser(ctx context.Context, in CreateUserInput) (*User, error)
	GetUser(ctx context.Context, id string) (*User, error)
	ListUsers(ctx context.Context, page Page) ([]User, string, error)
	UpdateUser(ctx context.Context, id string, in UpdateUserInput) (*User, error)
	DeleteUser(ctx context.Context, id string) error

	Login(ctx context.Context, email, password string) (*Session, error)
	Logout(ctx context.Context) error

	// CreateToken mints an API token. The token value is present on the result
	// exactly once and is never recoverable afterwards.
	CreateToken(ctx context.Context, in CreateTokenInput) (*IssuedToken, error)
	ListTokens(ctx context.Context, actorID string) ([]APIToken, error)
	RevokeToken(ctx context.Context, id string) error
}

// WebhookService covers outgoing delivery.
type WebhookService interface {
	PutWebhook(ctx context.Context, in WebhookInput) (*WebhookEndpoint, error)
	ListWebhooks(ctx context.Context) ([]WebhookEndpoint, error)
	DeleteWebhook(ctx context.Context, id string) error
	ListDeliveries(ctx context.Context, f DeliveryFilter) ([]WebhookDelivery, string, error)
	RedeliverWebhook(ctx context.Context, deliveryID string) error
}

// TransferService covers snapshot export and import.
type TransferService interface {
	Export(ctx context.Context, in ExportInput) (*Snapshot, error)
	Import(ctx context.Context, snap *Snapshot, in ImportInput) (*ImportResult, error)
}

// SyncService covers one-way import from external systems. Bidirectional sync
// is a v2 addition; see ROADMAP.md.
type SyncService interface {
	PutSyncSource(ctx context.Context, in SyncSourceInput) (*SyncSource, error)
	ListSyncSources(ctx context.Context) ([]SyncSource, error)
	DeleteSyncSource(ctx context.Context, id string) error
	// RunSync imports from a configured source. With DryRun set it reports what
	// would change without writing.
	RunSync(ctx context.Context, in RunSyncInput) (*SyncResult, error)
}

// Session is a browser or terminal login.
type Session struct {
	// Token is present exactly once, at login.
	Token     string    `json:"token"`
	ActorID   string    `json:"actor_id"`
	TenantID  string    `json:"tenant_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Snapshot is a portable dump of a tenant's data. It never contains password
// hashes, token hashes or webhook secrets.
type Snapshot struct {
	Version    int       `json:"version"`
	TenantKey  string    `json:"tenant_key"`
	ExportedAt time.Time `json:"exported_at"`

	Projects  []Project    `json:"projects,omitempty"`
	Workflows []Workflow   `json:"workflows,omitempty"`
	FieldDefs []FieldDef   `json:"field_defs,omitempty"`
	Labels    []Label      `json:"labels,omitempty"`
	Tasks     []Task       `json:"tasks,omitempty"`
	Deps      []Dependency `json:"dependencies,omitempty"`
	Comments  []Comment    `json:"comments,omitempty"`
	Artifacts []Artifact   `json:"artifacts,omitempty"`
}

// SnapshotVersion is the current snapshot format version.
const SnapshotVersion = 1

// ImportMode selects how an import treats existing data.
type ImportMode string

// Import modes. There is deliberately no destructive default.
const (
	// ImportMerge adds and updates, leaving unmentioned records alone.
	ImportMerge ImportMode = "merge"
	// ImportReplace removes records absent from the snapshot.
	ImportReplace ImportMode = "replace"
)

// ImportResult reports what an import did, or would do under a dry run.
type ImportResult struct {
	Created map[string]int `json:"created"`
	Updated map[string]int `json:"updated"`
	Skipped map[string]int `json:"skipped"`
	Deleted map[string]int `json:"deleted,omitempty"`
	// Warnings records lossy mappings and anything that could not be applied.
	Warnings []string `json:"warnings,omitempty"`
	DryRun   bool     `json:"dry_run"`
}

// SyncResult reports what an external import did, or would do under a dry run.
type SyncResult struct {
	ImportResult
	System string `json:"system"`
	Source string `json:"source"`
	// Cursor is the position reached. It advances only on success, so an
	// interrupted run is safely re-runnable.
	Cursor string `json:"cursor,omitempty"`
}

// PruneResult reports what pruning removed, or would remove under a dry run.
type PruneResult struct {
	Events            int64 `json:"events"`
	AuditEntries      int64 `json:"audit_entries"`
	WebhookDeliveries int64 `json:"webhook_deliveries"`
	// RetainedForSubscribers counts events kept because a live subscriber has
	// not yet read past them.
	RetainedForSubscribers int64 `json:"retained_for_subscribers"`
	DryRun                 bool  `json:"dry_run"`
}
