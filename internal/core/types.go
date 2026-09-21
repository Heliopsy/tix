package core

import (
	"encoding/json"
	"time"
)

// Tenant is the top-level isolation boundary.
type Tenant struct {
	ID        string     `json:"id" yaml:"id"`
	Key       string     `json:"key" yaml:"key"`
	Name      string     `json:"name" yaml:"name"`
	CreatedAt time.Time  `json:"created_at" yaml:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" yaml:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" yaml:"deleted_at,omitempty"`
}

// CertMode describes how a domain obtains its TLS certificate.
type CertMode string

// Certificate modes.
const (
	CertNone CertMode = "none"
	CertFile CertMode = "file"
)

// Domain maps a hostname to a tenant.
type Domain struct {
	ID         string     `json:"id" yaml:"id"`
	TenantID   string     `json:"tenant_id" yaml:"tenant_id"`
	Hostname   string     `json:"hostname" yaml:"hostname"`
	VerifiedAt *time.Time `json:"verified_at,omitempty" yaml:"verified_at,omitempty"`
	CertMode   CertMode   `json:"cert_mode" yaml:"cert_mode"`
	CertPath   string     `json:"cert_path,omitempty" yaml:"cert_path,omitempty"`
	KeyPath    string     `json:"key_path,omitempty" yaml:"key_path,omitempty"`
	CreatedAt  time.Time  `json:"created_at" yaml:"created_at"`
}

// Verified reports whether the domain has completed verification.
func (d Domain) Verified() bool { return d.VerifiedAt != nil }

// User is a credentialed human.
type User struct {
	ID          string     `json:"id" yaml:"id"`
	Email       string     `json:"email" yaml:"email"`
	DisplayName string     `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	CreatedAt   time.Time  `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" yaml:"updated_at"`
	DisabledAt  *time.Time `json:"disabled_at,omitempty" yaml:"disabled_at,omitempty"`
	SSOProvider string     `json:"sso_provider,omitempty" yaml:"sso_provider,omitempty"`
	SSOSubject  string     `json:"sso_subject,omitempty" yaml:"sso_subject,omitempty"`
}

// APIToken is a scoped bearer credential, normally held by an agent.
type APIToken struct {
	ID         string     `json:"id" yaml:"id"`
	TenantID   string     `json:"tenant_id" yaml:"tenant_id"`
	ActorID    string     `json:"actor_id" yaml:"actor_id"`
	Name       string     `json:"name" yaml:"name"`
	Scopes     []Scope    `json:"scopes" yaml:"scopes"`
	ProjectID  string     `json:"project_id,omitempty" yaml:"project_id,omitempty"`
	CreatedAt  time.Time  `json:"created_at" yaml:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty" yaml:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty" yaml:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty" yaml:"revoked_at,omitempty"`
}

// Active reports whether the token may still authenticate at time now.
func (t APIToken) Active(now time.Time) bool {
	if t.RevokedAt != nil {
		return false
	}
	return t.ExpiresAt == nil || t.ExpiresAt.After(now)
}

// IssuedToken carries a newly minted token value.
type IssuedToken struct {
	APIToken
	Token string `json:"token" yaml:"token"`
}

// Project groups tasks and is assigned exactly one workflow.
type Project struct {
	ID          string `json:"id" yaml:"id"`
	TenantID    string `json:"tenant_id" yaml:"tenant_id"`
	Key         string `json:"key" yaml:"key"`
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	WorkflowID  string `json:"workflow_id" yaml:"workflow_id"`

	// Color and Icon let a reader tell one project's rows from another's.
	Color ProjectColor `json:"color,omitempty" yaml:"color,omitempty"`
	Icon  string       `json:"icon,omitempty" yaml:"icon,omitempty"`

	ArchivedAt *time.Time `json:"archived_at,omitempty" yaml:"archived_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at" yaml:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at" yaml:"updated_at"`
}

// Archived reports whether the project is archived.
func (p Project) Archived() bool { return p.ArchivedAt != nil }

// StateCategory groups workflow states for display and reporting.
type StateCategory string

// State categories.
const (
	CategoryTodo       StateCategory = "todo"
	CategoryInProgress StateCategory = "in_progress"
	CategoryDone       StateCategory = "done"
)

// State is one node of a workflow's state machine.
type State struct {
	Key                 string        `json:"key" yaml:"key"`
	Label               string        `json:"label" yaml:"label"`
	Terminal            bool          `json:"terminal" yaml:"terminal"`
	Category            StateCategory `json:"category,omitempty" yaml:"category,omitempty"`
	RevertOnLeaseExpiry bool          `json:"revert_on_lease_expiry,omitempty" yaml:"revert_on_lease_expiry,omitempty"`
	RevertTo            string        `json:"revert_to,omitempty" yaml:"revert_to,omitempty"`
}

// Transition is one permitted edge of a workflow's state machine.
type Transition struct {
	From            string `json:"from" yaml:"from"`
	To              string `json:"to" yaml:"to"`
	RequiresScope   Scope  `json:"requires_scope,omitempty" yaml:"requires_scope,omitempty"`
	RequiresComment bool   `json:"requires_comment,omitempty" yaml:"requires_comment,omitempty"`
}

// WorkflowDefinition is the state machine, stored as one JSON document.
type WorkflowDefinition struct {
	Initial      string       `json:"initial" yaml:"initial"`
	States       []State      `json:"states" yaml:"states"`
	Transitions  []Transition `json:"transitions" yaml:"transitions"`
	DefaultLease Duration     `json:"default_lease,omitempty" yaml:"default_lease,omitempty"`
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

// IsTerminal reports whether the named state satisfies a dependency.
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

// CanTransition reports whether a move between states is permitted.
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
	ID         string             `json:"id" yaml:"id"`
	TenantID   string             `json:"tenant_id" yaml:"tenant_id"`
	Key        string             `json:"key" yaml:"key"`
	Name       string             `json:"name" yaml:"name"`
	Definition WorkflowDefinition `json:"definition" yaml:"definition"`
	Builtin    bool               `json:"builtin" yaml:"builtin"`
	CreatedAt  time.Time          `json:"created_at" yaml:"created_at"`
	UpdatedAt  time.Time          `json:"updated_at" yaml:"updated_at"`
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
	ID          string    `json:"id" yaml:"id"`
	TenantID    string    `json:"tenant_id" yaml:"tenant_id"`
	ProjectID   string    `json:"project_id" yaml:"project_id"`
	Key         string    `json:"key" yaml:"key"`
	Label       string    `json:"label" yaml:"label"`
	Type        FieldType `json:"type" yaml:"type"`
	Required    bool      `json:"required" yaml:"required"`
	EnumOptions []string  `json:"enum_options,omitempty" yaml:"enum_options,omitempty"`
	Default     any       `json:"default,omitempty" yaml:"default,omitempty"`
	Indexed     bool      `json:"indexed" yaml:"indexed"`
	Position    int       `json:"position" yaml:"position"`
}

// Priority orders tasks in a queue.
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

// Task is the central record.
type Task struct {
	ID        string `json:"id" yaml:"id"`
	TenantID  string `json:"tenant_id" yaml:"tenant_id"`
	ProjectID string `json:"project_id" yaml:"project_id"`
	Seq       int64  `json:"seq" yaml:"seq"`
	Ref       string `json:"ref" yaml:"ref"`

	ParentID string   `json:"parent_id,omitempty" yaml:"parent_id,omitempty"`
	Title    string   `json:"title" yaml:"title"`
	Body     string   `json:"body,omitempty" yaml:"body,omitempty"`
	Status   string   `json:"status" yaml:"status"`
	Priority Priority `json:"priority" yaml:"priority"`
	Tags     []string `json:"tags,omitempty" yaml:"tags,omitempty"`

	AssigneeActorID string `json:"assignee_actor_id,omitempty" yaml:"assignee_actor_id,omitempty"`
	CreatorActorID  string `json:"creator_actor_id" yaml:"creator_actor_id"`

	DueAt       *time.Time `json:"due_at,omitempty" yaml:"due_at,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty" yaml:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty" yaml:"completed_at,omitempty"`

	ClaimedByActorID string     `json:"claimed_by_actor_id,omitempty" yaml:"claimed_by_actor_id,omitempty"`
	ClaimedAt        *time.Time `json:"claimed_at,omitempty" yaml:"claimed_at,omitempty"`
	LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty" yaml:"lease_expires_at,omitempty"`
	ClaimCount       int        `json:"claim_count" yaml:"claim_count"`

	CustomFields map[string]any `json:"custom_fields,omitempty" yaml:"custom_fields,omitempty"`

	Version   int        `json:"version" yaml:"version"`
	CreatedAt time.Time  `json:"created_at" yaml:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" yaml:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" yaml:"deleted_at,omitempty"`

	Blocked   bool     `json:"blocked" yaml:"blocked"`
	DependsOn []string `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
}

// ClaimedAtTime reports whether the task is effectively claimed at time now.
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
	TenantID  string    `json:"tenant_id" yaml:"tenant_id"`
	TaskID    string    `json:"task_id" yaml:"task_id"`
	DependsOn string    `json:"depends_on" yaml:"depends_on"`
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
}

// Tag is a free-form tag.
type Tag struct {
	ID        string `json:"id" yaml:"id"`
	TenantID  string `json:"tenant_id" yaml:"tenant_id"`
	ProjectID string `json:"project_id,omitempty" yaml:"project_id,omitempty"`
	Name      string `json:"name" yaml:"name"`
	Color     string `json:"color,omitempty" yaml:"color,omitempty"`
}

// Comment is a message on a task.
type Comment struct {
	ID            string     `json:"id" yaml:"id"`
	TenantID      string     `json:"tenant_id" yaml:"tenant_id"`
	TaskID        string     `json:"task_id" yaml:"task_id"`
	AuthorActorID string     `json:"author_actor_id" yaml:"author_actor_id"`
	Body          string     `json:"body" yaml:"body"`
	CreatedAt     time.Time  `json:"created_at" yaml:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" yaml:"updated_at"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty" yaml:"deleted_at,omitempty"`
}

// ArtifactKind classifies a worker's structured output.
type ArtifactKind string

// Artifact kinds.
const (
	ArtifactResult ArtifactKind = "result"
	ArtifactLog    ArtifactKind = "log"
	ArtifactFile   ArtifactKind = "file"
	ArtifactMetric ArtifactKind = "metric"
)

// Artifact is structured output attached to a task by a worker.
type Artifact struct {
	ID          string         `json:"id" yaml:"id"`
	TenantID    string         `json:"tenant_id" yaml:"tenant_id"`
	TaskID      string         `json:"task_id" yaml:"task_id"`
	ActorID     string         `json:"actor_id" yaml:"actor_id"`
	Kind        ArtifactKind   `json:"kind" yaml:"kind"`
	Name        string         `json:"name,omitempty" yaml:"name,omitempty"`
	Payload     map[string]any `json:"payload,omitempty" yaml:"payload,omitempty"`
	ContentType string         `json:"content_type,omitempty" yaml:"content_type,omitempty"`
	Blob        []byte         `json:"blob,omitempty" yaml:"blob,omitempty"`
	CreatedAt   time.Time      `json:"created_at" yaml:"created_at"`
}

// Claim is the result of successfully claiming a task.
type Claim struct {
	Task           *Task     `json:"task" yaml:"task"`
	LeaseToken     string    `json:"lease_token" yaml:"lease_token"`
	LeaseExpiresAt time.Time `json:"lease_expires_at" yaml:"lease_expires_at"`
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
	EventLabelAdded      EventType = "tag.added"

	EventProjectCreated  EventType = "project.created"
	EventProjectUpdated  EventType = "project.updated"
	EventWorkflowUpdated EventType = "workflow.updated"
	EventFieldUpdated    EventType = "field.updated"

	EventWebhookDelivered EventType = "webhook.delivered"
	EventImportCompleted  EventType = "import.completed"
)

// Event is one durable record in the outbox.
type Event struct {
	Seq         int64          `json:"seq" yaml:"seq"`
	ID          string         `json:"id" yaml:"id"`
	TenantID    string         `json:"tenant_id" yaml:"tenant_id"`
	Type        EventType      `json:"type" yaml:"type"`
	ProjectID   string         `json:"project_id,omitempty" yaml:"project_id,omitempty"`
	SubjectType string         `json:"subject_type" yaml:"subject_type"`
	SubjectID   string         `json:"subject_id" yaml:"subject_id"`
	ActorID     string         `json:"actor_id,omitempty" yaml:"actor_id,omitempty"`
	Payload     map[string]any `json:"payload,omitempty" yaml:"payload,omitempty"`
	OccurredAt  time.Time      `json:"occurred_at" yaml:"occurred_at"`
}

// AuditEntry is one append-only record of a change.
type AuditEntry struct {
	Seq         int64           `json:"seq" yaml:"seq"`
	TenantID    string          `json:"tenant_id" yaml:"tenant_id"`
	ActorID     string          `json:"actor_id,omitempty" yaml:"actor_id,omitempty"`
	Action      string          `json:"action" yaml:"action"`
	SubjectType string          `json:"subject_type" yaml:"subject_type"`
	SubjectID   string          `json:"subject_id" yaml:"subject_id"`
	Before      json.RawMessage `json:"before,omitempty" yaml:"before,omitempty"`
	After       json.RawMessage `json:"after,omitempty" yaml:"after,omitempty"`
	Source      Source          `json:"source" yaml:"source"`
	OccurredAt  time.Time       `json:"occurred_at" yaml:"occurred_at"`
}

// WebhookEndpoint is a registered delivery target.
type WebhookEndpoint struct {
	ID         string    `json:"id" yaml:"id"`
	TenantID   string    `json:"tenant_id" yaml:"tenant_id"`
	URL        string    `json:"url" yaml:"url"`
	Secret     string    `json:"-" yaml:"-"`
	EventTypes []string  `json:"event_types" yaml:"event_types"`
	Active     bool      `json:"active" yaml:"active"`
	CreatedAt  time.Time `json:"created_at" yaml:"created_at"`
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
	ID             string         `json:"id" yaml:"id"`
	TenantID       string         `json:"tenant_id" yaml:"tenant_id"`
	EndpointID     string         `json:"endpoint_id" yaml:"endpoint_id"`
	EventSeq       int64          `json:"event_seq" yaml:"event_seq"`
	Attempts       int            `json:"attempts" yaml:"attempts"`
	NextAttemptAt  time.Time      `json:"next_attempt_at" yaml:"next_attempt_at"`
	Status         DeliveryStatus `json:"status" yaml:"status"`
	LastError      string         `json:"last_error,omitempty" yaml:"last_error,omitempty"`
	LastStatusCode int            `json:"last_status_code,omitempty" yaml:"last_status_code,omitempty"`
	CreatedAt      time.Time      `json:"created_at" yaml:"created_at"`
}

// RetentionPolicy bounds how long a tenant keeps its append-only tables.
type RetentionPolicy struct {
	TenantID          string   `json:"tenant_id" yaml:"tenant_id"`
	Events            Duration `json:"events" yaml:"events"`
	AuditEntries      Duration `json:"audit_entries" yaml:"audit_entries"`
	WebhookDeliveries Duration `json:"webhook_deliveries" yaml:"webhook_deliveries"`
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

// ExternalRef ties a tix entity to its counterpart in an external system.
type ExternalRef struct {
	TenantID        string    `json:"tenant_id" yaml:"tenant_id"`
	EntityType      string    `json:"entity_type" yaml:"entity_type"`
	EntityID        string    `json:"entity_id" yaml:"entity_id"`
	System          string    `json:"system" yaml:"system"`
	ExternalID      string    `json:"external_id" yaml:"external_id"`
	ExternalURL     string    `json:"external_url,omitempty" yaml:"external_url,omitempty"`
	ExternalVersion string    `json:"external_version,omitempty" yaml:"external_version,omitempty"`
	LastSyncedAt    time.Time `json:"last_synced_at" yaml:"last_synced_at"`
}

// SyncSource records an import source and its cursor.
type SyncSource struct {
	ID         string     `json:"id" yaml:"id"`
	TenantID   string     `json:"tenant_id" yaml:"tenant_id"`
	System     string     `json:"system" yaml:"system"`
	Name       string     `json:"name" yaml:"name"`
	Cursor     string     `json:"cursor,omitempty" yaml:"cursor,omitempty"`
	LastRunAt  *time.Time `json:"last_run_at,omitempty" yaml:"last_run_at,omitempty"`
	LastStatus string     `json:"last_status,omitempty" yaml:"last_status,omitempty"`
	CreatedAt  time.Time  `json:"created_at" yaml:"created_at"`
}
