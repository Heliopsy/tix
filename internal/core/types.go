package core

import (
	"encoding/json"
	"time"
)

// Tenant is the top-level isolation boundary. It owns projects, workflows,
// labels, field definitions, webhook endpoints and retention policy.
type Tenant struct {
	ID        string     `json:"id"`
	Key       string     `json:"key"`
	Name      string     `json:"name"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// CertMode describes how a domain obtains its TLS certificate.
type CertMode string

// Certificate modes. ACME is deliberately absent in v1; see ROADMAP.md.
const (
	// CertNone means TLS is terminated ahead of tix, by a reverse proxy.
	CertNone CertMode = "none"
	// CertFile means tix serves a supplied certificate and key.
	CertFile CertMode = "file"
)

// Domain maps a hostname to a tenant. The server resolves the request Host to a
// tenant before authentication runs, and pins it for the whole request.
type Domain struct {
	ID         string     `json:"id"`
	TenantID   string     `json:"tenant_id"`
	Hostname   string     `json:"hostname"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
	CertMode   CertMode   `json:"cert_mode"`
	CertPath   string     `json:"cert_path,omitempty"`
	KeyPath    string     `json:"key_path,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// Verified reports whether the domain has completed verification.
func (d Domain) Verified() bool { return d.VerifiedAt != nil }

// User is a credentialed human. One user row may hold memberships in several
// tenants, so credentials are never duplicated per tenant.
type User struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	// DisabledAt, when set, prevents authentication without deleting history.
	DisabledAt *time.Time `json:"disabled_at,omitempty"`
	// SSOProvider and SSOSubject are the v2 single sign-on seam. They are
	// persisted but never read in v1.
	SSOProvider string `json:"sso_provider,omitempty"`
	SSOSubject  string `json:"sso_subject,omitempty"`
}

// APIToken is a scoped bearer credential, normally held by an agent. The token
// value itself is returned once at creation and never stored, only its hash.
type APIToken struct {
	ID         string     `json:"id"`
	TenantID   string     `json:"tenant_id"`
	ActorID    string     `json:"actor_id"`
	Name       string     `json:"name"`
	Scopes     []Scope    `json:"scopes"`
	ProjectID  string     `json:"project_id,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

// Active reports whether the token may still authenticate at time now.
func (t APIToken) Active(now time.Time) bool {
	if t.RevokedAt != nil {
		return false
	}
	return t.ExpiresAt == nil || t.ExpiresAt.After(now)
}

// IssuedToken carries a newly minted token value. The Token field is populated
// exactly once, at creation, and is never recoverable afterwards.
type IssuedToken struct {
	APIToken
	Token string `json:"token"`
}

// Project groups tasks and is assigned exactly one workflow.
type Project struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenant_id"`
	Key         string     `json:"key"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	WorkflowID  string     `json:"workflow_id"`
	ArchivedAt  *time.Time `json:"archived_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Archived reports whether the project is archived.
func (p Project) Archived() bool { return p.ArchivedAt != nil }

// StateCategory groups workflow states for display and for reporting, without
// constraining what states a workflow may define.
type StateCategory string

// State categories.
const (
	CategoryTodo       StateCategory = "todo"
	CategoryInProgress StateCategory = "in_progress"
	CategoryDone       StateCategory = "done"
)

// State is one node of a workflow's state machine.
type State struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	// Terminal marks a state that satisfies a dependency. A task blocks its
	// dependents until it reaches a terminal state.
	Terminal bool          `json:"terminal"`
	Category StateCategory `json:"category,omitempty"`
	// RevertOnLeaseExpiry makes the sweeper move a task out of this state when
	// its lease expires, so abandoned work returns to the queue visibly.
	RevertOnLeaseExpiry bool `json:"revert_on_lease_expiry,omitempty"`
	// RevertTo names the state to revert to. Empty means the initial state.
	RevertTo string `json:"revert_to,omitempty"`
}

// Transition is one permitted edge of a workflow's state machine.
type Transition struct {
	From string `json:"from"`
	To   string `json:"to"`
	// RequiresScope, when set, demands an additional scope beyond task:transition.
	RequiresScope Scope `json:"requires_scope,omitempty"`
	// RequiresComment demands a comment accompany the transition.
	RequiresComment bool `json:"requires_comment,omitempty"`
}

// WorkflowDefinition is the state machine itself, stored as one JSON document
// because it is always read whole and never queried by sub-field.
type WorkflowDefinition struct {
	Initial      string       `json:"initial"`
	States       []State      `json:"states"`
	Transitions  []Transition `json:"transitions"`
	DefaultLease Duration     `json:"default_lease,omitempty"`
}

// State returns the named state.
func (d WorkflowDefinition) State(key string) (State, bool) {
	for _, s := range d.States {
		if s.Key == key {
			return s, true
		}
	}
	return State{}, false
}

// HasState reports whether the definition contains the named state.
func (d WorkflowDefinition) HasState(key string) bool {
	_, ok := d.State(key)
	return ok
}

// IsTerminal reports whether the named state satisfies a dependency. An unknown
// state is not terminal, so a task in a state removed from the workflow keeps
// blocking its dependents rather than silently unblocking them.
func (d WorkflowDefinition) IsTerminal(key string) bool {
	s, ok := d.State(key)
	return ok && s.Terminal
}

// TerminalStates returns the keys of every terminal state.
func (d WorkflowDefinition) TerminalStates() []string {
	var out []string
	for _, s := range d.States {
		if s.Terminal {
			out = append(out, s.Key)
		}
	}
	return out
}

// CanTransition reports whether moving from one state to another is permitted,
// and returns the transition so its requirements can be checked.
func (d WorkflowDefinition) CanTransition(from, to string) (Transition, bool) {
	for _, t := range d.Transitions {
		if t.From == from && t.To == to {
			return t, true
		}
	}
	return Transition{}, false
}

// Workflow is a named state machine owned by a tenant and assigned to projects.
type Workflow struct {
	ID         string             `json:"id"`
	TenantID   string             `json:"tenant_id"`
	Key        string             `json:"key"`
	Name       string             `json:"name"`
	Definition WorkflowDefinition `json:"definition"`
	// Builtin marks the workflow shipped by tix. It may be copied but not deleted.
	Builtin   bool      `json:"builtin"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FieldType is the type of a custom field's value.
type FieldType string

// Field types.
const (
	FieldString   FieldType = "string"
	FieldText     FieldType = "text"
	FieldInt      FieldType = "int"
	FieldFloat    FieldType = "float"
	FieldBool     FieldType = "bool"
	FieldDate     FieldType = "date"
	FieldDateTime FieldType = "datetime"
	FieldEnum     FieldType = "enum"
	FieldActor    FieldType = "actor"
	FieldJSON     FieldType = "json"
)

// Valid reports whether t is a known field type.
func (t FieldType) Valid() bool {
	switch t {
	case FieldString, FieldText, FieldInt, FieldFloat, FieldBool,
		FieldDate, FieldDateTime, FieldEnum, FieldActor, FieldJSON:
		return true
	default:
		return false
	}
}

// FieldDef defines one custom field on a project's tasks.
type FieldDef struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	ProjectID   string    `json:"project_id"`
	Key         string    `json:"key"`
	Label       string    `json:"label"`
	Type        FieldType `json:"type"`
	Required    bool      `json:"required"`
	EnumOptions []string  `json:"enum_options,omitempty"`
	Default     any       `json:"default,omitempty"`
	// Indexed generates a database index so the field can be filtered
	// efficiently. Filtering a non-indexed field is a scan, which is documented
	// behaviour rather than an error.
	Indexed  bool `json:"indexed"`
	Position int  `json:"position"`
}

// Priority orders tasks in a queue. Lower is more urgent, so the default sort
// is ascending and PriorityHighest sorts first.
type Priority int

// Priorities.
const (
	PriorityHighest Priority = 1
	PriorityHigh    Priority = 2
	PriorityNormal  Priority = 3
	PriorityLow     Priority = 4
	PriorityLowest  Priority = 5
)

// Valid reports whether p is within range.
func (p Priority) Valid() bool { return p >= PriorityHighest && p <= PriorityLowest }

// Task is the central record. A task belongs to exactly one project, may have a
// parent (containment) and dependencies (ordering), and may be claimed by an
// actor holding a lease.
type Task struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	ProjectID string `json:"project_id"`
	// Seq is the per-project number behind the human reference "infra-42".
	Seq int64 `json:"seq"`
	// Ref is the rendered human reference, populated on read.
	Ref string `json:"ref"`

	ParentID string   `json:"parent_id,omitempty"`
	Title    string   `json:"title"`
	Body     string   `json:"body,omitempty"`
	Status   string   `json:"status"`
	Priority Priority `json:"priority"`
	Labels   []string `json:"labels,omitempty"`

	AssigneeActorID string `json:"assignee_actor_id,omitempty"`
	CreatorActorID  string `json:"creator_actor_id"`

	DueAt       *time.Time `json:"due_at,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Claim fields. A lease whose expiry has passed is treated as unclaimed by
	// every read and every claim, so correctness never depends on the sweeper.
	ClaimedByActorID string     `json:"claimed_by_actor_id,omitempty"`
	ClaimedAt        *time.Time `json:"claimed_at,omitempty"`
	LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
	ClaimCount       int        `json:"claim_count"`

	CustomFields map[string]any `json:"custom_fields,omitempty"`

	// Version supports optimistic concurrency. An update carrying a stale
	// version is rejected rather than silently overwriting.
	Version   int        `json:"version"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`

	// Blocked is computed on read: true when any dependency is not terminal.
	Blocked bool `json:"blocked"`
	// DependsOn lists the tasks this one waits for, populated on detailed reads.
	DependsOn []string `json:"depends_on,omitempty"`
}

// ClaimedAtTime reports whether the task is effectively claimed at time now.
// An expired lease is not a claim.
func (t Task) ClaimedAtTime(now time.Time) bool {
	if t.ClaimedByActorID == "" || t.LeaseExpiresAt == nil {
		return false
	}
	return t.LeaseExpiresAt.After(now)
}

// Deleted reports whether the task is soft deleted.
func (t Task) Deleted() bool { return t.DeletedAt != nil }

// Dependency is a directed edge: Task waits for DependsOn to reach a terminal state.
type Dependency struct {
	TenantID  string    `json:"tenant_id"`
	TaskID    string    `json:"task_id"`
	DependsOn string    `json:"depends_on"`
	CreatedAt time.Time `json:"created_at"`
}

// Label is a free-form tag. A label with no project is available tenant-wide.
type Label struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	ProjectID string `json:"project_id,omitempty"`
	Name      string `json:"name"`
	Color     string `json:"color,omitempty"`
}

// Comment is a message on a task.
type Comment struct {
	ID            string     `json:"id"`
	TenantID      string     `json:"tenant_id"`
	TaskID        string     `json:"task_id"`
	AuthorActorID string     `json:"author_actor_id"`
	Body          string     `json:"body"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
}

// ArtifactKind classifies a worker's structured output.
type ArtifactKind string

// Artifact kinds. The set is open; these are the ones tix itself writes.
const (
	ArtifactResult ArtifactKind = "result"
	ArtifactLog    ArtifactKind = "log"
	ArtifactFile   ArtifactKind = "file"
	ArtifactMetric ArtifactKind = "metric"
)

// Artifact is structured output attached to a task by a worker. This is how an
// agent reports what it did in a form another program can read.
type Artifact struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenant_id"`
	TaskID      string         `json:"task_id"`
	ActorID     string         `json:"actor_id"`
	Kind        ArtifactKind   `json:"kind"`
	Name        string         `json:"name,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
	ContentType string         `json:"content_type,omitempty"`
	// Blob carries small inline content. Large content belongs elsewhere and is
	// referenced from Payload.
	Blob      []byte    `json:"blob,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Claim is the result of successfully claiming a task. The lease token is
// returned once and must be presented to renew, transition or release.
type Claim struct {
	Task           *Task     `json:"task"`
	LeaseToken     string    `json:"lease_token"`
	LeaseExpiresAt time.Time `json:"lease_expires_at"`
}

// EventType names a kind of domain event.
type EventType string

// Event types.
const (
	EventTaskCreated      EventType = "task.created"
	EventTaskUpdated      EventType = "task.updated"
	EventTaskTransitioned EventType = "task.transitioned"
	EventTaskClaimed      EventType = "task.claimed"
	EventTaskReleased     EventType = "task.released"
	EventTaskLeaseExpired EventType = "task.lease_expired"
	EventTaskDeleted      EventType = "task.deleted"

	EventCommentAdded    EventType = "comment.added"
	EventArtifactAdded   EventType = "artifact.added"
	EventDependencyAdded EventType = "dependency.added"
	EventLabelAdded      EventType = "label.added"

	EventProjectCreated  EventType = "project.created"
	EventProjectUpdated  EventType = "project.updated"
	EventWorkflowUpdated EventType = "workflow.updated"
	EventFieldUpdated    EventType = "field.updated"

	EventWebhookDelivered EventType = "webhook.delivered"
	EventImportCompleted  EventType = "import.completed"
)

// Event is one durable record in the outbox. Events are appended in the same
// transaction as the rows they describe, which is why a command-line write with
// no server running still reaches every connected subscriber.
type Event struct {
	// Seq is the monotonic cursor consumers resume from.
	Seq         int64          `json:"seq"`
	ID          string         `json:"id"`
	TenantID    string         `json:"tenant_id"`
	Type        EventType      `json:"type"`
	ProjectID   string         `json:"project_id,omitempty"`
	SubjectType string         `json:"subject_type"`
	SubjectID   string         `json:"subject_id"`
	ActorID     string         `json:"actor_id,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
	OccurredAt  time.Time      `json:"occurred_at"`
}

// AuditEntry is one append-only record of a change. Entries cannot be edited or
// deleted through any API.
type AuditEntry struct {
	Seq         int64  `json:"seq"`
	TenantID    string `json:"tenant_id"`
	ActorID     string `json:"actor_id,omitempty"`
	Action      string `json:"action"`
	SubjectType string `json:"subject_type"`
	SubjectID   string `json:"subject_id"`
	// Before is nil for a creation, After is nil for a deletion. Neither ever
	// contains a secret or a password hash.
	Before     json.RawMessage `json:"before,omitempty"`
	After      json.RawMessage `json:"after,omitempty"`
	Source     Source          `json:"source"`
	OccurredAt time.Time       `json:"occurred_at"`
}

// WebhookEndpoint is a registered delivery target.
type WebhookEndpoint struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	URL      string `json:"url"`
	// Secret signs deliveries. It is never included in an export.
	Secret     string    `json:"-"`
	EventTypes []string  `json:"event_types"`
	Active     bool      `json:"active"`
	CreatedAt  time.Time `json:"created_at"`
}

// DeliveryStatus is the state of one webhook delivery attempt chain.
type DeliveryStatus string

// Delivery statuses.
const (
	DeliveryPending   DeliveryStatus = "pending"
	DeliveryDelivered DeliveryStatus = "delivered"
	DeliveryFailed    DeliveryStatus = "failed"
)

// WebhookDelivery tracks one event's delivery to one endpoint.
type WebhookDelivery struct {
	ID             string         `json:"id"`
	TenantID       string         `json:"tenant_id"`
	EndpointID     string         `json:"endpoint_id"`
	EventSeq       int64          `json:"event_seq"`
	Attempts       int            `json:"attempts"`
	NextAttemptAt  time.Time      `json:"next_attempt_at"`
	Status         DeliveryStatus `json:"status"`
	LastError      string         `json:"last_error,omitempty"`
	LastStatusCode int            `json:"last_status_code,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

// RetentionPolicy bounds how long a tenant keeps its append-only tables.
// Audit is kept substantially longer than events by default: audit is the
// compliance record, events are a transport buffer.
type RetentionPolicy struct {
	TenantID          string   `json:"tenant_id"`
	Events            Duration `json:"events"`
	AuditEntries      Duration `json:"audit_entries"`
	WebhookDeliveries Duration `json:"webhook_deliveries"`
}

// DefaultRetention returns the shipped policy.
func DefaultRetention(tenantID string) RetentionPolicy {
	return RetentionPolicy{
		TenantID:          tenantID,
		Events:            Duration(30 * 24 * time.Hour),
		AuditEntries:      Duration(365 * 24 * time.Hour),
		WebhookDeliveries: Duration(30 * 24 * time.Hour),
	}
}

// ExternalRef ties a tix entity to its counterpart in an external system. This
// is what makes a re-import an update rather than a duplicate, and it is the
// field bidirectional sync will build on.
type ExternalRef struct {
	TenantID    string `json:"tenant_id"`
	EntityType  string `json:"entity_type"`
	EntityID    string `json:"entity_id"`
	System      string `json:"system"`
	ExternalID  string `json:"external_id"`
	ExternalURL string `json:"external_url,omitempty"`
	// ExternalVersion records the source's version or etag. v1 reads it only to
	// detect change; v2 uses it to detect conflicts.
	ExternalVersion string    `json:"external_version,omitempty"`
	LastSyncedAt    time.Time `json:"last_synced_at"`
}

// SyncSource records an import source and its cursor, so a refresh fetches only
// what changed since the last successful run.
type SyncSource struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	System   string `json:"system"`
	Name     string `json:"name"`
	// Cursor advances only when a run succeeds, so an interrupted import is
	// safely re-runnable.
	Cursor     string     `json:"cursor,omitempty"`
	LastRunAt  *time.Time `json:"last_run_at,omitempty"`
	LastStatus string     `json:"last_status,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}
