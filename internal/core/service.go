package core

import (
	"context"
	"io"
	"time"
)

// Service is the complete tix product surface.
type Service interface {
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
	TransitionTask(ctx context.Context, ref TaskRef, in TransitionInput) (*Task, error)
	DeleteTask(ctx context.Context, ref TaskRef, hard bool) error
	RestoreTask(ctx context.Context, ref TaskRef) (*Task, error)
	TaskTree(ctx context.Context, ref TaskRef, depth int) ([]Task, error)

	AddDependency(ctx context.Context, ref, dependsOn TaskRef) error
	RemoveDependency(ctx context.Context, ref, dependsOn TaskRef) error
	ListDependencies(ctx context.Context, ref TaskRef) ([]Dependency, error)

	AddTag(ctx context.Context, ref TaskRef, tag string) error
	RemoveTag(ctx context.Context, ref TaskRef, tag string) error
	ListTags(ctx context.Context) ([]Tag, error)

	AddComment(ctx context.Context, ref TaskRef, body string) (*Comment, error)
	ListComments(ctx context.Context, ref TaskRef) ([]Comment, error)
	EditComment(ctx context.Context, id, body string) (*Comment, error)
	DeleteComment(ctx context.Context, id string) error

	PutArtifact(ctx context.Context, ref TaskRef, in ArtifactInput) (*Artifact, error)
	ListArtifacts(ctx context.Context, ref TaskRef) ([]Artifact, error)
}

// ClaimService covers the agent work queue.
type ClaimService interface {
	ClaimTask(ctx context.Context, ref TaskRef, in ClaimInput) (*Claim, error)
	ClaimNext(ctx context.Context, in ClaimNextInput) (*Claim, error)
	RenewLease(ctx context.Context, ref TaskRef, token string, ttl Duration) (*Claim, error)
	ReleaseLease(ctx context.Context, ref TaskRef, token string, in ReleaseInput) error
	SweepLeases(ctx context.Context, limit int) (int, error)
}

// HistoryService covers the audit log, the event stream, and retention.
type HistoryService interface {
	ListAudit(ctx context.Context, f AuditFilter) ([]AuditEntry, string, error)
	Subscribe(ctx context.Context, f EventFilter) (<-chan Event, error)
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
//
// Both directions stream. A tenant at the scale tix targets does not fit
// comfortably in memory, and a snapshot assembled before the first byte is
// written would bound export by RAM rather than by disk.
type TransferService interface {
	// ExportTo writes a snapshot as it walks the data.
	ExportTo(ctx context.Context, in ExportInput, w io.Writer) error
	// ImportFrom reads a snapshot record by record.
	ImportFrom(ctx context.Context, r io.Reader, in ImportInput) (*ImportResult, error)
}

// SyncService covers one-way import from external systems.
type SyncService interface {
	PutSyncSource(ctx context.Context, in SyncSourceInput) (*SyncSource, error)
	ListSyncSources(ctx context.Context) ([]SyncSource, error)
	DeleteSyncSource(ctx context.Context, id string) error
	RunSync(ctx context.Context, in RunSyncInput) (*SyncResult, error)
}

// Session is a browser or terminal login.
type Session struct {
	Token     string    `json:"token" yaml:"token"`
	ActorID   string    `json:"actor_id" yaml:"actor_id"`
	TenantID  string    `json:"tenant_id" yaml:"tenant_id"`
	ExpiresAt time.Time `json:"expires_at" yaml:"expires_at"`
}

// SnapshotHeader is the first record of a streamed snapshot.
type SnapshotHeader struct {
	Version    int       `json:"version" yaml:"version"`
	TenantKey  string    `json:"tenant_key" yaml:"tenant_key"`
	ExportedAt time.Time `json:"exported_at" yaml:"exported_at"`
}

// RecordKind names what a streamed snapshot record carries.
type RecordKind string

// Record kinds, in the order a snapshot writes them so an importer can rely on
// referenced rows arriving before the rows that reference them.
const (
	RecordHeader     RecordKind = "header"
	RecordWorkflow   RecordKind = "workflow"
	RecordProject    RecordKind = "project"
	RecordFieldDef   RecordKind = "field_def"
	RecordLabel      RecordKind = "tag"
	RecordTask       RecordKind = "task"
	RecordDependency RecordKind = "dependency"
	RecordComment    RecordKind = "comment"
	RecordArtifact   RecordKind = "artifact"
)

// SnapshotRecord is one line of a streamed snapshot. Exactly one payload field
// is set, selected by Kind.
type SnapshotRecord struct {
	Kind RecordKind `json:"kind" yaml:"kind"`

	Header     *SnapshotHeader `json:"header,omitempty" yaml:"header,omitempty"`
	Workflow   *Workflow       `json:"workflow,omitempty" yaml:"workflow,omitempty"`
	Project    *Project        `json:"project,omitempty" yaml:"project,omitempty"`
	FieldDef   *FieldDef       `json:"field_def,omitempty" yaml:"field_def,omitempty"`
	Tag        *Tag            `json:"tag,omitempty" yaml:"tag,omitempty"`
	Task       *Task           `json:"task,omitempty" yaml:"task,omitempty"`
	Dependency *Dependency     `json:"dependency,omitempty" yaml:"dependency,omitempty"`
	Comment    *Comment        `json:"comment,omitempty" yaml:"comment,omitempty"`
	Artifact   *Artifact       `json:"artifact,omitempty" yaml:"artifact,omitempty"`
}

// Snapshot is a portable dump of a tenant's data.
type Snapshot struct {
	Version    int       `json:"version" yaml:"version"`
	TenantKey  string    `json:"tenant_key" yaml:"tenant_key"`
	ExportedAt time.Time `json:"exported_at" yaml:"exported_at"`

	Projects  []Project    `json:"projects,omitempty" yaml:"projects,omitempty"`
	Workflows []Workflow   `json:"workflows,omitempty" yaml:"workflows,omitempty"`
	FieldDefs []FieldDef   `json:"field_defs,omitempty" yaml:"field_defs,omitempty"`
	Tags      []Tag        `json:"tags,omitempty" yaml:"tags,omitempty"`
	Tasks     []Task       `json:"tasks,omitempty" yaml:"tasks,omitempty"`
	Deps      []Dependency `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	Comments  []Comment    `json:"comments,omitempty" yaml:"comments,omitempty"`
	Artifacts []Artifact   `json:"artifacts,omitempty" yaml:"artifacts,omitempty"`
}

// SnapshotVersion is the current snapshot format version.
const SnapshotVersion = 1

// ImportMode selects how an import treats existing data.
type ImportMode string

// Import modes.
const (
	ImportMerge   ImportMode = "merge"
	ImportReplace ImportMode = "replace"
)

// ImportResult reports what an import did, or would do under a dry run.
type ImportResult struct {
	Created  map[string]int `json:"created" yaml:"created"`
	Updated  map[string]int `json:"updated" yaml:"updated"`
	Skipped  map[string]int `json:"skipped" yaml:"skipped"`
	Deleted  map[string]int `json:"deleted,omitempty" yaml:"deleted,omitempty"`
	Warnings []string       `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	DryRun   bool           `json:"dry_run" yaml:"dry_run"`
}

// SyncResult reports what an external import did, or would do under a dry run.
type SyncResult struct {
	ImportResult
	System string `json:"system" yaml:"system"`
	Source string `json:"source" yaml:"source"`
	Cursor string `json:"cursor,omitempty" yaml:"cursor,omitempty"`
}

// PruneResult reports what pruning removed, or would remove under a dry run.
type PruneResult struct {
	Events                 int64 `json:"events" yaml:"events"`
	AuditEntries           int64 `json:"audit_entries" yaml:"audit_entries"`
	WebhookDeliveries      int64 `json:"webhook_deliveries" yaml:"webhook_deliveries"`
	RetainedForSubscribers int64 `json:"retained_for_subscribers" yaml:"retained_for_subscribers"`
	DryRun                 bool  `json:"dry_run" yaml:"dry_run"`
}
