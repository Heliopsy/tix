// SPDX-License-Identifier: AGPL-3.0-or-later

// Package store defines the persistence boundary.
package store

import (
	"context"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// Store opens transactions and reports on the database.
type Store interface {
	// Begin starts a read-write transaction scoped to one tenant.
	Begin(ctx context.Context, scope core.TenantScope) (Tx, error)
	// View runs fn in a read-only transaction.
	View(ctx context.Context, scope core.TenantScope, fn func(Tx) error) error
	// Update runs fn in a read-write transaction, committing on success.
	Update(ctx context.Context, scope core.TenantScope, fn func(Tx) error) error
	// Unscoped runs fn without a tenant in a read-write transaction, for
	// administering tenants and the installation's own rows.
	Unscoped(ctx context.Context, fn func(UnscopedTx) error) error
	// ViewUnscoped runs fn without a tenant in a read-only transaction. A
	// cross-tenant read that writes nothing belongs here rather than in
	// Unscoped, which on SQLite takes the single writer connection: host
	// resolution runs on every request, and taking the write lock for it made
	// one open write stall every request on the server.
	ViewUnscoped(ctx context.Context, fn func(UnscopedTx) error) error

	Migrate(ctx context.Context) error
	SchemaVersion(ctx context.Context) (int, error)
	Health(ctx context.Context) error
	Dialect() Dialect
	Close() error
}

// Dialect names a database engine.
type Dialect string

// Dialects.
const (
	SQLite   Dialect = "sqlite"
	Postgres Dialect = "postgres"
)

// Tx is a tenant-scoped transaction. Every method is confined to the tenant the
// transaction was opened for.
type Tx interface {
	Scope() core.TenantScope

	TenantTx
	ProjectTx
	WorkflowTx
	TaskTx
	ClaimTx
	AuthTx
	EventTx
	WebhookTx
	SyncTx
	StatsTx

	Commit() error
	Rollback() error
}

// UnscopedTx reaches across tenants. It exists only for resolving a hostname to
// a tenant before the scope is known, and for administering tenants themselves.
type UnscopedTx interface {
	ResolveDomain(ctx context.Context, hostname string) (*core.Tenant, error)
	GetTenantByKey(ctx context.Context, key string) (*core.Tenant, error)
	GetTenantByID(ctx context.Context, id string) (*core.Tenant, error)
	ListTenants(ctx context.Context, page core.Page) ([]core.Tenant, error)
	CreateTenant(ctx context.Context, t *core.Tenant) error
	UpdateTenant(ctx context.Context, t *core.Tenant) error
	DeleteTenant(ctx context.Context, id string) error
	GetUserByEmail(ctx context.Context, email string) (*core.User, string, error)
	// FindSSHKeysByFingerprint returns every live enrolment of a fingerprint,
	// across tenants. An SSH client proves a key before any tenant is known, so
	// this is the one lookup that cannot be scoped; the caller narrows to a
	// single tenant, or refuses, before anything else happens.
	FindSSHKeysByFingerprint(ctx context.Context, fingerprint string) ([]core.SSHKey, error)

	// The five server methods reach the installation's own registry of running
	// processes. A server serves every tenant, so its row belongs to none of
	// them and there is no scope to build with; this is the same door
	// FindSSHKeysByFingerprint goes through, for the neighbouring reason.
	//
	// internal/presence/confinement_test.go names the files that may call
	// them and fails the build anywhere else.

	// RegisterServer records a running process, replacing any earlier row with
	// the same identifier.
	RegisterServer(ctx context.Context, s *core.Server) error
	// HeartbeatServer refreshes a server's last seen instant. A row that is no
	// longer there is not an error: the process is being told it has been
	// forgotten, and it will register again.
	HeartbeatServer(ctx context.Context, serverID string, at time.Time) error
	// DeregisterServer removes a server's row, for a graceful shutdown. A
	// process that ends any other way leaves it, which is what a reader judging
	// staleness exists for.
	DeregisterServer(ctx context.Context, serverID string) error
	// ListServers returns registered servers, keyset-paged like every other
	// listing.
	ListServers(ctx context.Context, page core.Page) ([]core.Server, error)
	// ForgetServersBefore deletes rows unseen since the cutoff, so an
	// installation cycling through identifiers does not accumulate them.
	// Nothing a reader sees depends on it having run.
	ForgetServersBefore(ctx context.Context, cutoff time.Time) (int, error)

	Commit() error
	Rollback() error
}

// TenantTx covers domains and membership within one tenant.
type TenantTx interface {
	GetTenant(ctx context.Context) (*core.Tenant, error)
	AddDomain(ctx context.Context, d *core.Domain) error
	ListDomains(ctx context.Context) ([]core.Domain, error)
	RemoveDomain(ctx context.Context, hostname string) error
	AddMember(ctx context.Context, m *core.Membership) error
	GetMember(ctx context.Context, actorID string) (*core.Membership, error)
	ListMembers(ctx context.Context) ([]core.Membership, error)
	RemoveMember(ctx context.Context, actorID string) error
}

// ProjectTx covers projects and custom field definitions.
type ProjectTx interface {
	CreateProject(ctx context.Context, p *core.Project) error
	GetProject(ctx context.Context, ref string) (*core.Project, error)
	ListProjects(ctx context.Context, f core.ProjectFilter) ([]core.Project, error)
	UpdateProject(ctx context.Context, p *core.Project) error
	DeleteProject(ctx context.Context, id string) error

	PutFieldDef(ctx context.Context, d *core.FieldDef) error
	ListFieldDefs(ctx context.Context, projectID string) ([]core.FieldDef, error)
	DeleteFieldDef(ctx context.Context, projectID, key string) error
}

// WorkflowTx covers state machine definitions.
type WorkflowTx interface {
	PutWorkflow(ctx context.Context, w *core.Workflow) error
	GetWorkflow(ctx context.Context, key string) (*core.Workflow, error)
	GetWorkflowByID(ctx context.Context, id string) (*core.Workflow, error)
	ListWorkflows(ctx context.Context) ([]core.Workflow, error)
	DeleteWorkflow(ctx context.Context, key string) error
	CountTasksInStates(ctx context.Context, workflowID string, states []string) (map[string]int, error)
}

// TaskTx covers tasks and everything attached to them.
type TaskTx interface {
	CreateTask(ctx context.Context, t *core.Task) error
	GetTask(ctx context.Context, ref core.TaskRef) (*core.Task, error)
	ListTasks(ctx context.Context, f core.TaskFilter) ([]core.Task, error)
	UpdateTask(ctx context.Context, t *core.Task) error
	DeleteTask(ctx context.Context, id string, hard bool) error
	RestoreTask(ctx context.Context, id string) error
	// NextSeq reserves the next per-project task number.
	NextSeq(ctx context.Context, projectID string) (int64, error)
	Children(ctx context.Context, parentID string) ([]core.Task, error)

	AddDependency(ctx context.Context, d *core.Dependency) error
	RemoveDependency(ctx context.Context, taskID, dependsOn string) error
	ListDependencies(ctx context.Context, taskID string) ([]core.Dependency, error)
	// DependencyPathExists reports whether following dependencies from one task
	// reaches another, which is how a cycle is detected before it is created.
	DependencyPathExists(ctx context.Context, from, to string) (bool, error)

	PutTag(ctx context.Context, l *core.Tag) error
	ListTags(ctx context.Context) ([]core.Tag, error)
	AttachTag(ctx context.Context, taskID, tagID string) error
	DetachTag(ctx context.Context, taskID, tagID string) error

	CreateComment(ctx context.Context, c *core.Comment) error
	GetComment(ctx context.Context, id string) (*core.Comment, error)
	ListComments(ctx context.Context, taskID string) ([]core.Comment, error)
	UpdateComment(ctx context.Context, c *core.Comment) error
	DeleteComment(ctx context.Context, id string) error

	PutArtifact(ctx context.Context, a *core.Artifact) error
	ListArtifacts(ctx context.Context, taskID string) ([]core.Artifact, error)
}

// ClaimTx covers the work queue.
type ClaimTx interface {
	// ClaimTask is a conditional update that succeeds only if the task is
	// unclaimed or its lease has expired. It reports whether it claimed.
	ClaimTask(ctx context.Context, in ClaimRow) (bool, error)
	// ClaimNextTask claims the highest-priority eligible unblocked task.
	ClaimNextTask(ctx context.Context, in ClaimNextRow) (string, bool, error)
	// RenewLease extends a lease only if the token still matches.
	RenewLease(ctx context.Context, taskID, token string, until time.Time) (bool, error)
	// ReleaseLease clears a claim only if the token still matches.
	ReleaseLease(ctx context.Context, taskID, token string) (bool, error)
	// ExpiredLeases returns tasks whose lease has passed, for the sweeper.
	ExpiredLeases(ctx context.Context, now time.Time, limit int) ([]core.Task, error)
	// ClearClaim drops an expired claim and records that it expired. It
	// reports false when the row no longer carries the lapsed lease it was
	// asked to clear, which is a task another worker has legitimately taken
	// since, not a missing task.
	ClearClaim(ctx context.Context, in ExpireClaimRow) (bool, error)
	// ForceReclaim ends a live lease without its token, for an administrator
	// overriding the holder. It reports false, having written nothing, when
	// the row no longer carries the live lease the caller read: the holder it
	// names has changed, or the lease has gone. Only the lease columns move,
	// so a forced reclaim can never change a task's status.
	ForceReclaim(ctx context.Context, in ForceReclaimRow) (bool, error)
}

// ForceReclaimRow is the input to an administrative reclaim. HolderID is the
// holder the caller read, and is re-asserted inside the statement: without it
// the update is keyed on the row identifier alone and two administrators
// forcing at once, or one forcing a lease that was released and taken again
// in between, would both be told they had taken the lease the caller saw.
type ForceReclaimRow struct {
	TaskID   string
	HolderID string
	At       time.Time
}

// ExpireClaimRow is the input to clearing a lapsed claim. The holder and the
// instant travel with it rather than being read from the store clock, so the
// evidence a sweep leaves carries the sweep's own time.
type ExpireClaimRow struct {
	TaskID   string
	HolderID string
	At       time.Time
}

// ClaimRow is the input to a conditional claim.
type ClaimRow struct {
	TaskID     string
	ActorID    string
	Now        time.Time
	Until      time.Time
	LeaseToken string
}

// ClaimNextRow selects which task to claim.
type ClaimNextRow struct {
	ProjectIDs []string
	Tags       []string
	Statuses   []string
	// Terminal names the finished states of every workflow a judged row may
	// belong to. A dependency may live in any project under any workflow, so
	// the query resolves each row's own workflow rather than matching a state
	// name against a union.
	Terminal   []WorkflowTerminal
	ActorID    string
	Now        time.Time
	Until      time.Time
	LeaseToken string
}

// WorkflowTerminal pairs a workflow with the states that finish work under it.
// Terminal is a property of a workflow, not of a state name: the same name may
// finish work in one workflow and be a waypoint in another.
type WorkflowTerminal struct {
	WorkflowID string
	States     []string
}

// AuthTx covers actors, users, sessions and tokens.
type AuthTx interface {
	CreateActor(ctx context.Context, a *core.Actor) error
	GetActor(ctx context.Context, id string) (*core.Actor, error)
	GetActorByHandle(ctx context.Context, handle string) (*core.Actor, error)
	ListActors(ctx context.Context, page core.Page) ([]core.Actor, error)

	CreateUser(ctx context.Context, u *core.User, passwordHash string) error
	GetUser(ctx context.Context, id string) (*core.User, error)
	ListUsers(ctx context.Context, page core.Page) ([]core.User, error)
	UpdateUser(ctx context.Context, u *core.User, passwordHash string) error
	DeleteUser(ctx context.Context, id string) error

	CreateSession(ctx context.Context, actorID, tokenHash string, expiresAt time.Time) error
	// DeleteActorSessions ends every session an actor holds, so removing or
	// disabling a user takes effect immediately rather than at expiry.
	DeleteActorSessions(ctx context.Context, actorID string) (int64, error)
	GetSessionByHash(ctx context.Context, tokenHash string) (actorID string, expiresAt time.Time, err error)
	DeleteSession(ctx context.Context, tokenHash string) error
	DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error)

	CreateToken(ctx context.Context, t *core.APIToken, tokenHash string) error
	GetTokenByHash(ctx context.Context, tokenHash string) (*core.APIToken, error)
	// GetToken returns one token of this tenant by identifier, so a caller
	// that is about to change it can record whose it was.
	GetToken(ctx context.Context, id string) (*core.APIToken, error)
	ListTokens(ctx context.Context, actorID string) ([]core.APIToken, error)
	// TokenNameInUse reports whether an unrevoked token of this tenant already
	// carries the given name, whichever actor holds it. Revoked names are free
	// again, which is the same window the unique index enforces.
	TokenNameInUse(ctx context.Context, name string) (bool, error)
	RevokeToken(ctx context.Context, id string, at time.Time) error
	// RevokeActorTokens revokes every token an actor holds.
	RevokeActorTokens(ctx context.Context, actorID string, at time.Time) (int64, error)
	TouchToken(ctx context.Context, id string, at time.Time) error

	CreateSSHKey(ctx context.Context, k *core.SSHKey) error
	GetSSHKey(ctx context.Context, id string) (*core.SSHKey, error)
	ListSSHKeys(ctx context.Context, actorID string) ([]core.SSHKey, error)
	RevokeSSHKey(ctx context.Context, id string, at time.Time) error
	TouchSSHKey(ctx context.Context, id string, at time.Time) error
}

// EventTx covers the outbox, the audit log and retention.
type EventTx interface {
	// AppendEvent writes to the outbox. It runs in the same transaction as the
	// rows it describes, which is what makes events durable without a server.
	AppendEvent(ctx context.Context, e *core.Event) error
	ReadEvents(ctx context.Context, sinceSeq int64, limit int) ([]core.Event, error)
	LatestEventSeq(ctx context.Context) (int64, error)

	AppendAudit(ctx context.Context, e *core.AuditEntry) error
	ListAudit(ctx context.Context, f core.AuditFilter) ([]core.AuditEntry, error)

	GetRetention(ctx context.Context) (*core.RetentionPolicy, error)
	PutRetention(ctx context.Context, p *core.RetentionPolicy) error
	PruneEvents(ctx context.Context, before time.Time, floorSeq int64, limit int) (int64, error)
	PruneAudit(ctx context.Context, before time.Time, limit int) (int64, error)
	PruneDeliveries(ctx context.Context, before time.Time, limit int) (int64, error)
}

// WebhookTx covers delivery endpoints and their queue.
type WebhookTx interface {
	PutWebhook(ctx context.Context, e *core.WebhookEndpoint) error
	GetWebhook(ctx context.Context, id string) (*core.WebhookEndpoint, error)
	ListWebhooks(ctx context.Context) ([]core.WebhookEndpoint, error)
	DeleteWebhook(ctx context.Context, id string) error

	EnqueueDelivery(ctx context.Context, d *core.WebhookDelivery) error
	// ClaimDeliveries locks pending deliveries so concurrent processes do not
	// double-deliver.
	ClaimDeliveries(ctx context.Context, owner string, now, until time.Time, limit int) ([]core.WebhookDelivery, error)
	MarkDelivered(ctx context.Context, id string, statusCode int, at time.Time) error
	MarkFailed(ctx context.Context, id string, statusCode int, errMsg string, nextAttempt time.Time, terminal bool) error
	GetDelivery(ctx context.Context, id string) (*core.WebhookDelivery, error)
	ListDeliveries(ctx context.Context, f core.DeliveryFilter) ([]core.WebhookDelivery, error)
}

// SyncTx covers external import bookkeeping.
type SyncTx interface {
	PutSyncSource(ctx context.Context, s *core.SyncSource) error
	GetSyncSource(ctx context.Context, id string) (*core.SyncSource, error)
	ListSyncSources(ctx context.Context) ([]core.SyncSource, error)
	DeleteSyncSource(ctx context.Context, id string) error

	PutExternalRef(ctx context.Context, r *core.ExternalRef) error
	GetExternalRef(ctx context.Context, system, externalID, entityType string) (*core.ExternalRef, error)
	ListExternalRefs(ctx context.Context, system string) ([]core.ExternalRef, error)
}

// StatsTx covers the aggregates behind a statistics read.
type StatsTx interface {
	// TaskStats answers one statistics read with the smallest set of rows the
	// figures can be derived from.
	TaskStats(ctx context.Context, q StatsQuery) (*StatsRows, error)
	// WorkCounts totals what this tenant holds, as counts rather than rows. A
	// status report wants the size of the thing, and listing a tenant's tasks
	// to learn how many there are would make the cheapest question in the
	// product the most expensive.
	//
	// now decides which claims are still claims: a lease past its expiry reads
	// as unclaimed whether or not the sweeper has been, which is the same
	// judgement Task.ClaimedAtTime makes on a single row.
	WorkCounts(ctx context.Context, now time.Time) (WorkCounts, error)
}

// WorkCounts totals one tenant's work.
type WorkCounts struct {
	Projects int
	Tasks    int
	Claimed  int
	// LeasesExpiredUnswept are claims past their expiry that the sweeper has
	// not cleared yet. They already read as unclaimed; the figure says how far
	// behind the sweeper is.
	LeasesExpiredUnswept int
	WebhooksPending      int
	WebhooksFailed       int
}

// StatsQuery bounds one statistics read. Statuses is supplied by the caller
// because which statuses exist is a workflow question, and the store does not
// answer workflow questions.
type StatsQuery struct {
	ProjectID string
	Since     time.Time
	Until     time.Time
	Statuses  []string
	Oldest    int
}

// CompletedTask is one task that reached a terminal state inside the window.
// ActorID is empty when the transition that terminated it is no longer in the
// audit trail, because the tasks table records when a task was completed and
// never by whom.
type CompletedTask struct {
	TaskID      string
	ProjectID   string
	ActorID     string
	CreatedAt   time.Time
	CompletedAt time.Time
}

// StatsRows are the raw aggregates a statistics read is derived from. Soft
// deleted tasks are absent from every field.
type StatsRows struct {
	Completed    []CompletedTask
	Created      int
	StatusCounts map[string]int
	Oldest       []core.Task
}
