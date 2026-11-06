// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"encoding/json"
	"slices"
	"strings"
	"time"
)

// Tenant is the top-level isolation boundary.
type Tenant struct {
	// ID is the tenant's unique identifier.
	ID string `json:"id" yaml:"id"`
	// Key is the tenant's short, unique, human-readable key, used to refer to it
	// in place of the ID.
	Key string `json:"key" yaml:"key"`
	// Name is the tenant's display name.
	Name string `json:"name" yaml:"name"`
	// Theme names the palette this tenant presents itself with, on every
	// surface. Empty means nobody chose, and the accent is derived from the
	// tenant's own identity instead; see ThemeRegistry.Resolve.
	Theme string `json:"theme,omitempty" yaml:"theme,omitempty"`
	// RequireTOTPWeb requires a second factor when signing in to the browser
	// interface. Each surface is set independently and defaults to false.
	RequireTOTPWeb bool `json:"require_totp_web" yaml:"require_totp_web"`
	// RequireTOTPCLI requires a second factor when signing in from the command
	// line. Defaults to false.
	RequireTOTPCLI bool `json:"require_totp_cli" yaml:"require_totp_cli"`
	// RequireTOTPSSH requires a second factor when opening the board over SSH,
	// on top of the key. Defaults to false.
	RequireTOTPSSH bool `json:"require_totp_ssh" yaml:"require_totp_ssh"`
	// LeaseDefault is how long a claim on this tenant's tasks holds when the
	// claim, task, project and workflow name none, as a duration such as "4h".
	// Zero or absent defers to the deployment configuration, then the built-in
	// default.
	LeaseDefault Duration `json:"lease_default,omitempty" yaml:"lease_default,omitempty"`
	// LeaseMax caps any lease granted on this tenant's tasks, as a duration
	// such as "24h". It can lower the deployment's ceiling but never raise it;
	// zero or absent inherits the deployment's.
	LeaseMax Duration `json:"lease_max,omitempty" yaml:"lease_max,omitempty"`
	// TrashOff makes deleting a task permanent on this tenant. False, the
	// default, keeps deleted tasks in a trash they can be restored from.
	TrashOff bool `json:"trash_off" yaml:"trash_off"`
	// TimeFormat is the date format this tenant's people read unless they
	// chose their own. Empty inherits the deployment's output.time_format.
	TimeFormat string `json:"time_format,omitempty" yaml:"time_format,omitempty"`
	// Timezone is the IANA zone this tenant's people read dates in unless they
	// chose their own. Empty inherits the deployment's output.timezone.
	Timezone string `json:"timezone,omitempty" yaml:"timezone,omitempty"`

	// CreatedAt is when the tenant was created, RFC 3339.
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	// UpdatedAt is when the tenant was last changed, RFC 3339.
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
	// DeletedAt is when the tenant was deleted, RFC 3339. Absent on a live
	// tenant.
	DeletedAt *time.Time `json:"deleted_at,omitempty" yaml:"deleted_at,omitempty"`
}

// RequiresTOTP reports whether this tenant demands a second factor on the
// given sign-in surface.
func (t Tenant) RequiresTOTP(s Surface) bool {
	switch s {
	case SurfaceWeb:
		return t.RequireTOTPWeb
	case SurfaceCLI:
		return t.RequireTOTPCLI
	case SurfaceSSH:
		return t.RequireTOTPSSH
	default:
		return false
	}
}

// CertMode describes how a domain obtains its TLS certificate.
type CertMode string

// Certificate modes.
const (
	CertNone CertMode = "none"
	CertFile CertMode = "file"
)

// CertModes lists every certificate mode.
var CertModes = []CertMode{CertNone, CertFile}

// Valid reports whether m is a known certificate mode.
func (m CertMode) Valid() bool { return slices.Contains(CertModes, m) }

// Domain maps a hostname to a tenant.
type Domain struct {
	// ID is the domain's unique identifier.
	ID string `json:"id" yaml:"id"`
	// TenantID is the tenant the hostname resolves to.
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	// Hostname is the host name requests arrive on, such as "tix.example.com".
	Hostname string `json:"hostname" yaml:"hostname"`
	// VerifiedAt is when the hostname was verified, RFC 3339. Absent until it
	// is.
	VerifiedAt *time.Time `json:"verified_at,omitempty" yaml:"verified_at,omitempty"`
	// CertMode is how the hostname gets its TLS certificate: "none", or "file"
	// to serve the certificate and key at CertPath and KeyPath.
	CertMode CertMode `json:"cert_mode" yaml:"cert_mode"`
	// CertPath is the server-side path of the PEM certificate, set when CertMode
	// is "file".
	CertPath string `json:"cert_path,omitempty" yaml:"cert_path,omitempty"`
	// KeyPath is the server-side path of the PEM private key, set when CertMode
	// is "file".
	KeyPath string `json:"key_path,omitempty" yaml:"key_path,omitempty"`
	// CreatedAt is when the domain was added, RFC 3339.
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
}

// Verified reports whether the domain has completed verification.
func (d Domain) Verified() bool { return d.VerifiedAt != nil }

// User is a credentialed human.
type User struct {
	// ID is the user's actor identifier.
	ID string `json:"id" yaml:"id"`
	// Email is the user's sign-in address, unique within the installation.
	Email string `json:"email" yaml:"email"`
	// DisplayName is the name shown for the user. Empty means the email is shown
	// instead.
	DisplayName string `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	// CreatedAt is when the user was created, RFC 3339.
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	// UpdatedAt is when the user was last changed, RFC 3339.
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
	// DisabledAt is when the user was disabled, RFC 3339. A disabled user cannot
	// sign in; absent on an active user.
	DisabledAt *time.Time `json:"disabled_at,omitempty" yaml:"disabled_at,omitempty"`
	// SSOProvider names the single sign-on provider that vouches for the user.
	// Empty for a password user.
	SSOProvider string `json:"sso_provider,omitempty" yaml:"sso_provider,omitempty"`
	// SSOSubject is the user's subject identifier at SSOProvider.
	SSOSubject string `json:"sso_subject,omitempty" yaml:"sso_subject,omitempty"`
	// TOTPEnrolled reports whether this user holds a confirmed second factor.
	// It is state, not a credential: nothing here can produce a code, and it
	// is what lets an administrative listing say who holds one rather than
	// only offering to clear it. Set by the listings and by GetUser; a user
	// written back from one carries it and nothing reads it as an input.
	TOTPEnrolled bool `json:"totp_enrolled" yaml:"totp_enrolled"`
	// GlobalAdmin reports whether the user is a global admin, who administers
	// every tenant of the installation rather than only their own.
	GlobalAdmin bool `json:"global_admin" yaml:"global_admin"`
	// RequirePasswordChange reports whether the next password sign-in must
	// choose a new password before a session is minted.
	RequirePasswordChange bool `json:"require_password_change" yaml:"require_password_change"`
}

// APIToken is a scoped bearer credential, normally held by an agent.
type APIToken struct {
	// ID is the token's identifier, used to list and revoke it. It is not the
	// secret.
	ID string `json:"id" yaml:"id"`
	// TenantID is the tenant the token authenticates to.
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	// ActorID is the actor the token acts as.
	ActorID string `json:"actor_id" yaml:"actor_id"`
	// Name is the label given to the token when it was created.
	Name string `json:"name" yaml:"name"`
	// Scopes are the permissions the token carries; a request outside them is
	// refused.
	Scopes []Scope `json:"scopes" yaml:"scopes"`
	// ProjectID restricts the token to one project when set. Empty means every
	// project the actor can reach.
	ProjectID string `json:"project_id,omitempty" yaml:"project_id,omitempty"`
	// CreatedAt is when the token was issued, RFC 3339.
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	// ExpiresAt is when the token stops authenticating, RFC 3339. Absent means
	// it never expires.
	ExpiresAt *time.Time `json:"expires_at,omitempty" yaml:"expires_at,omitempty"`
	// LastUsedAt is when the token last authenticated a request, RFC 3339.
	// Absent if never used.
	LastUsedAt *time.Time `json:"last_used_at,omitempty" yaml:"last_used_at,omitempty"`
	// RevokedAt is when the token was revoked, RFC 3339. A revoked token never
	// authenticates again.
	RevokedAt *time.Time `json:"revoked_at,omitempty" yaml:"revoked_at,omitempty"`
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
	// Token is the secret bearer value. It is returned only when the token is
	// issued and cannot be read again.
	Token string `json:"token" yaml:"token"`
}

// Project groups tasks and is assigned exactly one workflow.
type Project struct {
	// ID is the project's unique identifier.
	ID string `json:"id" yaml:"id"`
	// TenantID is the tenant that owns the project.
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	// Key is the project's short unique key, which prefixes its task references
	// such as "INFRA-42".
	Key string `json:"key" yaml:"key"`
	// Name is the project's display name.
	Name string `json:"name" yaml:"name"`
	// Description is free text about the project.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// WorkflowID is the ID of the workflow the project's tasks follow.
	WorkflowID string `json:"workflow_id" yaml:"workflow_id"`

	// Color is the project's colour: a palette name or a custom #rrggbb
	// value. Empty means the project has none.
	Color ProjectColor `json:"color,omitempty" yaml:"color,omitempty"`
	// Icon is a text icon, one emoji or a one or two letter monogram. At most
	// one of Icon, IconName and IconImage is set.
	Icon string `json:"icon,omitempty" yaml:"icon,omitempty"`
	// IconName names one of the built-in icons, such as "rocket" or "bug".
	IconName string `json:"icon_name,omitempty" yaml:"icon_name,omitempty"`
	// IconImage is the digest of an uploaded icon image, whose bytes are read
	// from the project's icon route. Empty when the project has no image.
	IconImage string `json:"icon_image,omitempty" yaml:"icon_image,omitempty"`

	// LeaseDefault is how long a claim on this project's tasks holds when
	// neither the claim nor the task says, as a duration such as "4h". It
	// outranks the workflow and the tenant; zero or absent inherits.
	LeaseDefault Duration `json:"lease_default,omitempty" yaml:"lease_default,omitempty"`
	// LeaseMax caps any lease granted on this project's tasks, as a duration
	// such as "24h". It narrows the tenant's ceiling and never raises it; zero
	// or absent inherits.
	LeaseMax Duration `json:"lease_max,omitempty" yaml:"lease_max,omitempty"`

	// ArchivedAt is when the project was archived, RFC 3339. Absent on an active
	// project.
	ArchivedAt *time.Time `json:"archived_at,omitempty" yaml:"archived_at,omitempty"`
	// CreatedAt is when the project was created, RFC 3339.
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	// UpdatedAt is when the project was last changed, RFC 3339.
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
}

// Archived reports whether the project is archived.
func (p Project) Archived() bool { return p.ArchivedAt != nil }

// StateCategory groups workflow states for display and reporting.
//
// The vocabulary is fixed rather than open, and a state carries a category
// rather than a colour of its own, so that a colour means the same thing in
// every deployment: one tenant's red cannot be another tenant's green, and a
// terminal that renders no colour at all still has a word to fall back on.
type StateCategory string

// State categories.
const (
	CategoryTodo       StateCategory = "todo"
	CategoryInProgress StateCategory = "in_progress"
	CategoryBlocked    StateCategory = "blocked"
	CategoryWaiting    StateCategory = "waiting"
	CategoryDone       StateCategory = "done"
	CategoryCancelled  StateCategory = "cancelled"
)

// stateCategories is the whole vocabulary, in the order work moves through it:
// what has not started, what is moving, what has stalled here, what is stalled
// elsewhere, what finished, and what will not be done.
var stateCategories = []StateCategory{
	CategoryTodo, CategoryInProgress, CategoryBlocked,
	CategoryWaiting, CategoryDone, CategoryCancelled,
}

// StateCategories returns the vocabulary a state category may be chosen from.
func StateCategories() []StateCategory {
	out := make([]StateCategory, len(stateCategories))
	copy(out, stateCategories)
	return out
}

// Valid reports whether c is the empty category or a member of the vocabulary.
// The empty category is valid because it is how a state declines to say, and
// the surface reading it derives one from what else the state carries.
func (c StateCategory) Valid() bool {
	if c == "" {
		return true
	}
	for _, known := range stateCategories {
		if c == known {
			return true
		}
	}
	return false
}

// String renders the category token.
func (c StateCategory) String() string { return string(c) }

// JoinStateCategories renders the vocabulary for an error message.
func JoinStateCategories() string {
	parts := make([]string, 0, len(stateCategories))
	for _, c := range stateCategories {
		parts = append(parts, string(c))
	}
	return strings.Join(parts, ", ")
}

// State is one node of a workflow's state machine.
type State struct {
	// Key identifies the state within its workflow and is what a task's status
	// holds.
	Key string `json:"key" yaml:"key"`
	// Label is the state's display name.
	Label string `json:"label" yaml:"label"`
	// Terminal marks a state that ends the work: reaching it completes the task
	// and satisfies dependencies on it.
	Terminal bool `json:"terminal" yaml:"terminal"`
	// Category groups the state for display and reporting. Empty derives one:
	// done when terminal, todo for the initial state, in progress otherwise.
	Category StateCategory `json:"category,omitempty" yaml:"category,omitempty"`
	// RevertOnLeaseExpiry moves a task in this state to RevertTo when its claim
	// runs out.
	RevertOnLeaseExpiry bool `json:"revert_on_lease_expiry,omitempty" yaml:"revert_on_lease_expiry,omitempty"`
	// RevertTo is the state a task returns to when RevertOnLeaseExpiry applies.
	RevertTo string `json:"revert_to,omitempty" yaml:"revert_to,omitempty"`
}

// Transition is one permitted edge of a workflow's state machine.
type Transition struct {
	// From is the state key the move starts from.
	From string `json:"from" yaml:"from"`
	// To is the state key the move ends in.
	To string `json:"to" yaml:"to"`
	// RequiresScope is a scope the actor must hold to make this move. Empty
	// means no extra scope.
	RequiresScope Scope `json:"requires_scope,omitempty" yaml:"requires_scope,omitempty"`
	// RequiresComment refuses the move unless it carries a comment.
	RequiresComment bool `json:"requires_comment,omitempty" yaml:"requires_comment,omitempty"`
}

// WorkflowDefinition is the state machine, stored as one JSON document.
type WorkflowDefinition struct {
	// Initial is the key of the state a new task starts in.
	Initial string `json:"initial" yaml:"initial"`
	// States are the workflow's states.
	States []State `json:"states" yaml:"states"`
	// Transitions are the permitted moves between states. A move not listed is
	// refused.
	Transitions []Transition `json:"transitions" yaml:"transitions"`
	// DefaultLease is how long a claim on a task under this workflow holds when
	// the claim, task and project name none, as a duration such as "4h". Zero or
	// absent defers to the tenant.
	DefaultLease Duration `json:"default_lease,omitempty" yaml:"default_lease,omitempty"`
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

// CategoryOf returns the category a state belongs to: the one it names, or
// done for a terminal state that names none, todo for the initial state and
// in progress for any other.
func (d WorkflowDefinition) CategoryOf(key string) StateCategory {
	s, ok := d.State(key)
	switch {
	case !ok:
		return ""
	case s.Category != "":
		return s.Category
	case s.Terminal:
		return CategoryDone
	case s.Key == d.Initial:
		return CategoryTodo
	default:
		return CategoryInProgress
	}
}

// CompletesWork reports whether reaching the named state finishes the work,
// as opposed to abandoning it: a terminal state whose category is done.
func (d WorkflowDefinition) CompletesWork(key string) bool {
	return d.IsTerminal(key) && d.CategoryOf(key) == CategoryDone
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
	// ID is the workflow's unique identifier.
	ID string `json:"id" yaml:"id"`
	// TenantID is the tenant that owns the workflow.
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	// Key is the workflow's unique key within the tenant.
	Key string `json:"key" yaml:"key"`
	// Name is the workflow's display name.
	Name string `json:"name" yaml:"name"`
	// Definition is the state machine itself.
	Definition WorkflowDefinition `json:"definition" yaml:"definition"`
	// Builtin marks a workflow shipped with tix. It can be changed but not
	// deleted.
	Builtin bool `json:"builtin" yaml:"builtin"`
	// CreatedAt is when the workflow was created, RFC 3339.
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	// UpdatedAt is when the workflow was last changed, RFC 3339.
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
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

// FieldTypes lists every custom field type.
var FieldTypes = []FieldType{
	FieldString, FieldText, FieldInt, FieldFloat, FieldBool,
	FieldDate, FieldDateTime, FieldEnum, FieldActor, FieldJSON,
}

// Valid reports whether t is a known field type.
func (t FieldType) Valid() bool { return slices.Contains(FieldTypes, t) }

// FieldDef defines one custom field on a project's tasks.
type FieldDef struct {
	// ID is the field definition's unique identifier.
	ID string `json:"id" yaml:"id"`
	// TenantID is the tenant that owns the field.
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	// ProjectID is the project whose tasks carry the field.
	ProjectID string `json:"project_id" yaml:"project_id"`
	// Key names the field in a task's custom_fields, unique within the project.
	Key string `json:"key" yaml:"key"`
	// Label is the field's display name.
	Label string `json:"label" yaml:"label"`
	// Type is the type every value of the field must have.
	Type FieldType `json:"type" yaml:"type"`
	// Required refuses a new task that sets no value for the field and has no
	// Default to fall back on.
	Required bool `json:"required" yaml:"required"`
	// EnumOptions are the allowed values when Type is "enum".
	EnumOptions []string `json:"enum_options,omitempty" yaml:"enum_options,omitempty"`
	// Default is the value a new task gets when it sets none, coerced to Type.
	Default any `json:"default,omitempty" yaml:"default,omitempty"`
	// Position orders the project's fields, lowest first, ties broken by key.
	Position int `json:"position" yaml:"position"`
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
	// ID is the task's unique identifier.
	ID string `json:"id" yaml:"id"`
	// TenantID is the tenant that owns the task.
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	// ProjectID is the project the task belongs to.
	ProjectID string `json:"project_id" yaml:"project_id"`
	// Seq is the task's number within its project, the numeric part of Ref.
	Seq int64 `json:"seq" yaml:"seq"`
	// Ref is the human reference, the project key and Seq such as "INFRA-42".
	// Anywhere a task is named, Ref or ID is accepted.
	Ref string `json:"ref" yaml:"ref"`

	// ParentID is the ID of the task this one is a subtask of. Empty for a top-
	// level task.
	ParentID string `json:"parent_id,omitempty" yaml:"parent_id,omitempty"`
	// Title is the task's one-line summary.
	Title string `json:"title" yaml:"title"`
	// Body is the task's description, in Markdown. Lines of the form "- [ ]
	// item" form its checklist.
	Body string `json:"body,omitempty" yaml:"body,omitempty"`
	// Status is the key of the task's current workflow state.
	Status string `json:"status" yaml:"status"`
	// Priority is from 1 (highest) to 5 (lowest); 3 is normal.
	Priority Priority `json:"priority" yaml:"priority"`
	// Tags are the names of the tags on the task.
	Tags []string `json:"tags,omitempty" yaml:"tags,omitempty"`

	// AssigneeActorID is the actor the task is assigned to. Empty when
	// unassigned.
	AssigneeActorID string `json:"assignee_actor_id,omitempty" yaml:"assignee_actor_id,omitempty"`
	// CreatorActorID is the actor who created the task.
	CreatorActorID string `json:"creator_actor_id" yaml:"creator_actor_id"`

	// DueAt is when the task is due, RFC 3339. Absent when it has no due date.
	DueAt *time.Time `json:"due_at,omitempty" yaml:"due_at,omitempty"`
	// StartedAt is when the task first left its workflow's initial state, RFC
	// 3339.
	StartedAt *time.Time `json:"started_at,omitempty" yaml:"started_at,omitempty"`
	// CompletedAt is when the task reached a terminal state, RFC 3339. Cleared
	// when it leaves one.
	CompletedAt *time.Time `json:"completed_at,omitempty" yaml:"completed_at,omitempty"`

	// ClaimedByActorID is the actor holding the current claim. A claim whose
	// LeaseExpiresAt has passed holds nothing.
	ClaimedByActorID string `json:"claimed_by_actor_id,omitempty" yaml:"claimed_by_actor_id,omitempty"`
	// ClaimedAt is when the current claim was taken, RFC 3339.
	ClaimedAt *time.Time `json:"claimed_at,omitempty" yaml:"claimed_at,omitempty"`
	// LeaseExpiresAt is when the current claim runs out unless renewed, RFC
	// 3339.
	LeaseExpiresAt *time.Time `json:"lease_expires_at,omitempty" yaml:"lease_expires_at,omitempty"`
	// ClaimCount is how many times the task has been claimed.
	ClaimCount int `json:"claim_count" yaml:"claim_count"`

	// LeaseExpiredAt is when the last claim on the task ran out rather than
	// being released, RFC 3339. The sweeper writes it as it clears the lease,
	// so the fact that a holder dropped the task survives the clearing.
	LeaseExpiredAt *time.Time `json:"lease_expired_at,omitempty" yaml:"lease_expired_at,omitempty"`
	// LeaseExpiredByActorID is the actor whose claim ran out at LeaseExpiredAt.
	LeaseExpiredByActorID string `json:"lease_expired_by_actor_id,omitempty" yaml:"lease_expired_by_actor_id,omitempty"`

	// CustomFields holds the values of the project's custom fields, keyed by
	// field key.
	CustomFields map[string]any `json:"custom_fields,omitempty" yaml:"custom_fields,omitempty"`

	// Recurrence is the canonical repeat rule, empty for a task that does not
	// repeat. Finishing a repeating task spawns its next occurrence.
	Recurrence string `json:"recurrence,omitempty" yaml:"recurrence,omitempty"`
	// RecurrenceNextID is the ID of the occurrence this task spawned when it
	// was finished. Once set, the task never spawns again.
	RecurrenceNextID string `json:"recurrence_next_id,omitempty" yaml:"recurrence_next_id,omitempty"`

	// CopiedFromID names the task this one was copied from, when the copy was
	// asked to keep that link.
	CopiedFromID string `json:"copied_from_id,omitempty" yaml:"copied_from_id,omitempty"`

	// Lease is how long a claim on this task holds when the claim names no
	// ttl of its own, as a duration such as "4h". Zero or absent falls through
	// to the project, workflow and tenant.
	Lease Duration `json:"lease,omitempty" yaml:"lease,omitempty"`

	// SnoozedUntil hides the task from default listings while it is in the
	// future, RFC 3339. A past value is not a snooze.
	SnoozedUntil *time.Time `json:"snoozed_until,omitempty" yaml:"snoozed_until,omitempty"`
	// SnoozedByActorID is the actor who set the snooze.
	SnoozedByActorID string `json:"snoozed_by_actor_id,omitempty" yaml:"snoozed_by_actor_id,omitempty"`

	// Version increases by one on every change. Pass it back on an update to
	// have a stale write refused with a conflict.
	Version int `json:"version" yaml:"version"`
	// CreatedAt is when the task was created, RFC 3339.
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	// UpdatedAt is when the task was last changed, RFC 3339.
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
	// DeletedAt is when the task was moved to the trash, RFC 3339. Absent on a
	// live task.
	DeletedAt *time.Time `json:"deleted_at,omitempty" yaml:"deleted_at,omitempty"`

	// Blocked reports that at least one task this one depends on is neither
	// completed nor deleted.
	Blocked bool `json:"blocked" yaml:"blocked"`
	// DependsOn lists the IDs of the tasks this one waits for.
	DependsOn []string `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`

	// Starred is the reading actor's own star, never anybody else's. Only a
	// read that knows its reader sets it, so it is omitted rather than false
	// wherever nobody asked, which keeps it out of audit snapshots and exports.
	Starred bool `json:"starred,omitempty" yaml:"starred,omitempty"`

	// Checklist counts the checklist items of the description. A read sets it
	// from the body, so it is absent from audit snapshots and exports, and nil
	// when the description has no items.
	Checklist *Checklist `json:"checklist,omitempty" yaml:"checklist,omitempty"`
}

// ClaimedAtTime reports whether the task is effectively claimed at time now.
func (t Task) ClaimedAtTime(now time.Time) bool {
	if t.ClaimedByActorID == "" || t.LeaseExpiresAt == nil {
		return false
	}
	return t.LeaseExpiresAt.After(now)
}

// LeaseExpiryEvidenceWindow is how long a task keeps reporting that its last
// claim expired.
//
// The signal answers one operational question: is a holder dying and dropping
// work? That is read by a person opening a list, so the window has to span the
// gap between one person looking and the next, and a claim that lapsed
// overnight must still be visible in the morning. A day covers that, and it is
// also a full MaxTTL lease cycle, so no single lease can outlive its own
// evidence. Much longer and a backlog of long-lived tasks carries the mark
// permanently, at which point it no longer means "recently" and stops being
// worth looking at.
//
// The columns themselves are not cleared when the window passes. The window is
// a read-time judgement about what is still interesting; the row keeps the
// fact so an operator can still ask about older expiries directly.
const LeaseExpiryEvidenceWindow = 24 * time.Hour

// ClaimExpiredRecently reports whether the task's last claim expired within
// LeaseExpiryEvidenceWindow of now and nothing has claimed it since.
func (t Task) ClaimExpiredRecently(now time.Time) bool {
	if t.LeaseExpiredAt == nil || t.ClaimedAtTime(now) {
		return false
	}
	return now.Sub(*t.LeaseExpiredAt) < LeaseExpiryEvidenceWindow
}

// Deleted reports whether the task is soft deleted.
func (t Task) Deleted() bool { return t.DeletedAt != nil }

// SnoozedAt reports whether the task is snoozed at time now.
func (t Task) SnoozedAt(now time.Time) bool {
	return t.SnoozedUntil != nil && t.SnoozedUntil.After(now)
}

// Dependency is a directed edge: Task waits for DependsOn to reach a terminal state.
type Dependency struct {
	// TenantID is the tenant that owns both tasks.
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	// TaskID is the ID of the task that waits.
	TaskID string `json:"task_id" yaml:"task_id"`
	// DependsOn is the ID of the task waited for, which must reach a terminal
	// state.
	DependsOn string `json:"depends_on" yaml:"depends_on"`
	// CreatedAt is when the dependency was added, RFC 3339.
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
}

// Tag is a free-form tag.
type Tag struct {
	// ID is the tag's unique identifier.
	ID string `json:"id" yaml:"id"`
	// TenantID is the tenant that owns the tag.
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	// ProjectID limits the tag to one project when set. Empty for a tenant-wide
	// tag.
	ProjectID string `json:"project_id,omitempty" yaml:"project_id,omitempty"`
	// Name is the tag as written on tasks.
	Name string `json:"name" yaml:"name"`
	// Color is the tag's display colour. Empty when none was chosen.
	Color string `json:"color,omitempty" yaml:"color,omitempty"`
}

// Comment is a message on a task.
type Comment struct {
	// ID is the comment's unique identifier.
	ID string `json:"id" yaml:"id"`
	// TenantID is the tenant that owns the comment.
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	// TaskID is the ID of the task commented on.
	TaskID string `json:"task_id" yaml:"task_id"`
	// AuthorActorID is the actor who wrote the comment.
	AuthorActorID string `json:"author_actor_id" yaml:"author_actor_id"`
	// Body is the comment text, in Markdown.
	Body string `json:"body" yaml:"body"`
	// CreatedAt is when the comment was written, RFC 3339.
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	// UpdatedAt is when the comment was last edited, RFC 3339.
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
	// DeletedAt is when the comment was deleted, RFC 3339. Absent on a live
	// comment.
	DeletedAt *time.Time `json:"deleted_at,omitempty" yaml:"deleted_at,omitempty"`
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

// ArtifactKinds lists every artifact kind.
var ArtifactKinds = []ArtifactKind{
	ArtifactResult, ArtifactLog, ArtifactFile, ArtifactMetric,
}

// Valid reports whether k is a known artifact kind.
func (k ArtifactKind) Valid() bool { return slices.Contains(ArtifactKinds, k) }

// Artifact is structured output attached to a task by a worker.
type Artifact struct {
	// ID is the artifact's unique identifier.
	ID string `json:"id" yaml:"id"`
	// TenantID is the tenant that owns the artifact.
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	// TaskID is the ID of the task the artifact is attached to.
	TaskID string `json:"task_id" yaml:"task_id"`
	// ActorID is the actor that wrote the artifact, normally the claim holder.
	ActorID string `json:"actor_id" yaml:"actor_id"`
	// Kind classifies the output.
	Kind ArtifactKind `json:"kind" yaml:"kind"`
	// Name labels the artifact among the task's others.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// Payload is structured output as a JSON object.
	Payload map[string]any `json:"payload,omitempty" yaml:"payload,omitempty"`
	// ContentType is the media type of Blob, such as "text/plain".
	ContentType string `json:"content_type,omitempty" yaml:"content_type,omitempty"`
	// Blob is inline content, base64 in JSON, at most 1 MiB. Larger content
	// belongs elsewhere, referenced from Payload.
	Blob []byte `json:"blob,omitempty" yaml:"blob,omitempty"`
	// CreatedAt is when the artifact was written, RFC 3339.
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
}

// Claim is the result of successfully claiming a task.
type Claim struct {
	// Task is the claimed task as it stands after the claim.
	Task *Task `json:"task" yaml:"task"`
	// LeaseToken proves the claim. Renewing, releasing and writing artifacts
	// need it; keep it secret.
	LeaseToken string `json:"lease_token" yaml:"lease_token"`
	// LeaseExpiresAt is when the claim runs out unless renewed, RFC 3339.
	LeaseExpiresAt time.Time `json:"lease_expires_at" yaml:"lease_expires_at"`
	// Lease is how long the lease granted runs and which level decided it.
	Lease *LeaseGrant `json:"lease,omitempty" yaml:"lease,omitempty"`
}

// LeaseSource names the level a lease length or ceiling was taken from.
type LeaseSource string

// Lease sources, most specific first.
const (
	LeaseFromClaim      LeaseSource = "claim"
	LeaseFromTask       LeaseSource = "task"
	LeaseFromProject    LeaseSource = "project"
	LeaseFromWorkflow   LeaseSource = "workflow"
	LeaseFromTenant     LeaseSource = "tenant"
	LeaseFromDeployment LeaseSource = "deployment"
	LeaseFromBuiltin    LeaseSource = "builtin"
)

// LeaseGrant is the lease a claim is, or would be, granted on one task: its
// length and the level it came from, and the ceiling and the level setting it.
// Clamped reports a length cut down to the ceiling.
type LeaseGrant struct {
	// TTL is the lease length granted, as a duration such as "4h".
	TTL Duration `json:"ttl" yaml:"ttl"`
	// Source is the level TTL was taken from.
	Source LeaseSource `json:"source" yaml:"source"`
	// Max is the ceiling a lease on the task may not exceed, as a duration such
	// as "24h".
	Max Duration `json:"max" yaml:"max"`
	// MaxSource is the level Max was taken from.
	MaxSource LeaseSource `json:"max_source" yaml:"max_source"`
	// Clamped reports that the requested or inherited length was cut down to
	// Max.
	Clamped bool `json:"clamped,omitempty" yaml:"clamped,omitempty"`
}

// leaseSourceText is where a lease length came from, as a phrase.
var leaseSourceText = map[LeaseSource]string{
	LeaseFromClaim:      "from the claim",
	LeaseFromTask:       "from the task",
	LeaseFromProject:    "from the project",
	LeaseFromWorkflow:   "from the workflow",
	LeaseFromTenant:     "from the tenant",
	LeaseFromDeployment: "from the server configuration",
	LeaseFromBuiltin:    "the built-in default",
}

// Sentence says how long a claim holds and why, such as
// "claims last 4h, from the project".
func (g LeaseGrant) Sentence() string {
	if g.Clamped {
		return "claims last " + g.TTL.Human() + ", capped by the " + string(g.MaxSource) + " maximum"
	}
	return "claims last " + g.TTL.Human() + ", " + leaseSourceText[g.Source]
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
	// EventTaskReclaimed says an administrator ended somebody else's lease.
	// It is not EventTaskLeaseExpired: a lapse is a worker that stopped
	// answering and a reclaim is a person who decided, and a subscriber that
	// cannot tell them apart will read every override as an agent failure.
	EventTaskReclaimed EventType = "task.reclaimed"
	EventTaskDeleted   EventType = "task.deleted"
	// EventTaskStarred and EventTaskUnstarred record one actor's personal
	// star. They are tenant-visible, like the audit entry beside them.
	EventTaskStarred   EventType = "task.starred"
	EventTaskUnstarred EventType = "task.unstarred"
	// EventTaskSnoozed and EventTaskUnsnoozed record a snooze being set and
	// cleared by an actor; EventTaskWoke records a snooze that ran out.
	EventTaskSnoozed   EventType = "task.snoozed"
	EventTaskUnsnoozed EventType = "task.unsnoozed"
	EventTaskWoke      EventType = "task.woke"

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

	// EventViewArranged records one reader changing their own arrangement of
	// a view's groups. Like a star it changes nothing anybody else sees.
	EventViewArranged EventType = "view.arranged"

	// EventFilterSaved, EventFilterUpdated and EventFilterDeleted record a
	// reader creating, changing or removing one of their saved filters.
	EventFilterSaved   EventType = "filter.saved"
	EventFilterUpdated EventType = "filter.updated"
	EventFilterDeleted EventType = "filter.deleted"
	// EventInboxUpdated says one or more actors' inboxes changed: items were
	// created for them, read or unread, or their mutes changed. Its payload
	// names the actors under "recipients".
	EventInboxUpdated EventType = "inbox.updated"
	// EventTemplateUpdated records a task template saved, edited or deleted;
	// the payload's action says which.
	EventTemplateUpdated EventType = "template.updated"
)

// eventTypes is the complete event vocabulary, in declaration order.
var eventTypes = []EventType{
	EventTaskCreated, EventTaskUpdated, EventTaskTransitioned,
	EventTaskClaimed, EventTaskReleased, EventTaskLeaseExpired,
	EventTaskReclaimed,
	EventTaskDeleted, EventTaskStarred, EventTaskUnstarred,
	EventTaskSnoozed, EventTaskUnsnoozed, EventTaskWoke,
	EventCommentAdded, EventArtifactAdded,
	EventDependencyAdded, EventLabelAdded, EventProjectCreated,
	EventProjectUpdated, EventWorkflowUpdated, EventFieldUpdated,
	EventWebhookDelivered, EventImportCompleted, EventViewArranged,
	EventFilterSaved, EventFilterUpdated, EventFilterDeleted,
	EventInboxUpdated,
	EventTemplateUpdated,
}

// EventTypes returns the complete event vocabulary. Callers outside this
// package derive their closed sets from it rather than copying the constants,
// so a new event type reaches every layer at once. The slice is a copy, which
// keeps the vocabulary read-only without exporting a mutable package variable.
func EventTypes() []EventType { return slices.Clone(eventTypes) }

// Event is one durable record in the outbox.
type Event struct {
	// Seq is the event's position in the tenant's stream, increasing. Resume a
	// stream after the last Seq seen.
	Seq int64 `json:"seq" yaml:"seq"`
	// ID is the event's unique identifier.
	ID string `json:"id" yaml:"id"`
	// TenantID is the tenant the event belongs to.
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	// Type is the kind of event.
	Type EventType `json:"type" yaml:"type"`
	// ProjectID is the project the event concerns. Empty for a tenant-level
	// event.
	ProjectID string `json:"project_id,omitempty" yaml:"project_id,omitempty"`
	// SubjectType is the kind of record the event is about, such as "task".
	SubjectType string `json:"subject_type" yaml:"subject_type"`
	// SubjectID is the ID of the record the event is about.
	SubjectID string `json:"subject_id" yaml:"subject_id"`
	// ActorID is the actor who caused the event. Empty for a system event.
	ActorID string `json:"actor_id,omitempty" yaml:"actor_id,omitempty"`
	// Payload carries event-specific details, including the subject's ref when
	// it has one.
	Payload map[string]any `json:"payload,omitempty" yaml:"payload,omitempty"`
	// Via is the access channel the change arrived through: its kind and the
	// token's name. Network details stay in the audit log.
	Via *Via `json:"via,omitempty" yaml:"via,omitempty"`
	// OccurredAt is when the event happened, RFC 3339.
	OccurredAt time.Time `json:"occurred_at" yaml:"occurred_at"`
}

// AuditEntry is one append-only record of a change.
type AuditEntry struct {
	// Seq is the entry's position in the tenant's audit log, increasing.
	Seq int64 `json:"seq" yaml:"seq"`
	// TenantID is the tenant the entry belongs to.
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	// ActorID is the actor who made the change. Empty for a system change.
	ActorID string `json:"actor_id,omitempty" yaml:"actor_id,omitempty"`
	// Action names the change, such as "task.create".
	Action string `json:"action" yaml:"action"`
	// SubjectType is the kind of record changed, such as "task".
	SubjectType string `json:"subject_type" yaml:"subject_type"`
	// SubjectID is the ID of the record changed.
	SubjectID string `json:"subject_id" yaml:"subject_id"`
	// Before is the record as JSON before the change. Absent for a creation.
	Before json.RawMessage `json:"before,omitempty" yaml:"before,omitempty"`
	// After is the record as JSON after the change. Absent for a removal.
	After json.RawMessage `json:"after,omitempty" yaml:"after,omitempty"`
	// Source is the surface the change came through.
	Source Source `json:"source" yaml:"source"`
	// Via is the access channel the change arrived through and what
	// identified the caller on it. Kind is unknown on an entry recorded
	// before channels were.
	Via Via `json:"via" yaml:"via"`
	// OccurredAt is when the change was made, RFC 3339.
	OccurredAt time.Time `json:"occurred_at" yaml:"occurred_at"`
}

// WebhookEndpoint is a registered delivery target.
type WebhookEndpoint struct {
	// ID is the endpoint's unique identifier.
	ID string `json:"id" yaml:"id"`
	// TenantID is the tenant that owns the endpoint.
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	// URL is where deliveries are POSTed.
	URL string `json:"url" yaml:"url"`
	// Secret signs each delivery. It is never returned.
	Secret string `json:"-" yaml:"-"`
	// EventTypes are the event types delivered, exact or with a trailing "*"
	// such as "task.*". Empty delivers every event.
	EventTypes []string `json:"event_types" yaml:"event_types"`
	// Active turns deliveries on. An inactive endpoint receives nothing.
	Active bool `json:"active" yaml:"active"`
	// CreatedAt is when the endpoint was registered, RFC 3339.
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
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
	// ID is the delivery's unique identifier, used to redeliver it.
	ID string `json:"id" yaml:"id"`
	// TenantID is the tenant that owns the delivery.
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	// EndpointID is the ID of the endpoint delivered to.
	EndpointID string `json:"endpoint_id" yaml:"endpoint_id"`
	// EventSeq is the Seq of the event delivered.
	EventSeq int64 `json:"event_seq" yaml:"event_seq"`
	// Attempts is how many times delivery has been tried.
	Attempts int `json:"attempts" yaml:"attempts"`
	// NextAttemptAt is when the next attempt is due, RFC 3339.
	NextAttemptAt time.Time `json:"next_attempt_at" yaml:"next_attempt_at"`
	// Status is where the delivery stands.
	Status DeliveryStatus `json:"status" yaml:"status"`
	// LastError describes why the last attempt failed. Empty after a success.
	LastError string `json:"last_error,omitempty" yaml:"last_error,omitempty"`
	// LastStatusCode is the HTTP status the endpoint answered the last attempt
	// with. Absent when no response arrived.
	LastStatusCode int `json:"last_status_code,omitempty" yaml:"last_status_code,omitempty"`
	// CreatedAt is when the delivery was queued, RFC 3339.
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
}

// RetentionPolicy bounds how long a tenant keeps its append-only tables.
type RetentionPolicy struct {
	// TenantID is the tenant the policy applies to.
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	// Events is how long events are kept, as a duration such as "720h". The
	// default is 30 days.
	Events Duration `json:"events" yaml:"events"`
	// AuditEntries is how long audit entries are kept, as a duration such as
	// "8760h". The default is 365 days.
	AuditEntries Duration `json:"audit_entries" yaml:"audit_entries"`
	// WebhookDeliveries is how long webhook delivery records are kept, as a
	// duration. The default is 30 days.
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
	// TenantID is the tenant that owns the link.
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	// EntityType is the kind of tix record linked, such as "task".
	EntityType string `json:"entity_type" yaml:"entity_type"`
	// EntityID is the ID of the tix record linked.
	EntityID string `json:"entity_id" yaml:"entity_id"`
	// System names the external system.
	System string `json:"system" yaml:"system"`
	// ExternalID is the record's identifier in the external system.
	ExternalID string `json:"external_id" yaml:"external_id"`
	// ExternalURL links to the record in the external system.
	ExternalURL string `json:"external_url,omitempty" yaml:"external_url,omitempty"`
	// ExternalVersion is the external record's version as last imported.
	ExternalVersion string `json:"external_version,omitempty" yaml:"external_version,omitempty"`
	// LastSyncedAt is when the record was last imported, RFC 3339.
	LastSyncedAt time.Time `json:"last_synced_at" yaml:"last_synced_at"`
}

// SyncSource records an import source and its cursor.
type SyncSource struct {
	// ID is the source's unique identifier.
	ID string `json:"id" yaml:"id"`
	// TenantID is the tenant that owns the source.
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	// System names the external system imported from.
	System string `json:"system" yaml:"system"`
	// Name identifies the source within its system.
	Name string `json:"name" yaml:"name"`
	// Config is the source's stored non-secret settings, such as "url" or
	// "project". It never holds a credential.
	Config map[string]string `json:"config,omitempty" yaml:"config,omitempty"`
	// MappingPath is the stored mapping file, empty when the environment names it.
	MappingPath string `json:"mapping_path,omitempty" yaml:"mapping_path,omitempty"`
	// Cursor is how far the import has read, opaque and owned by the system's
	// importer.
	Cursor string `json:"cursor,omitempty" yaml:"cursor,omitempty"`
	// LastRunAt is when the source was last run, RFC 3339.
	LastRunAt *time.Time `json:"last_run_at,omitempty" yaml:"last_run_at,omitempty"`
	// LastStatus is how the last run went: "running", "ok" or "failed".
	LastStatus string `json:"last_status,omitempty" yaml:"last_status,omitempty"`
	// CreatedAt is when the source was added, RFC 3339.
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
}

// TrashStatus says whether a tenant keeps deleted tasks, and how many it holds.
// Held counts the trash even while it is off, because turning it off keeps
// what is already there rather than destroying it.
type TrashStatus struct {
	// Enabled reports whether deleted tasks go to the trash rather than being
	// removed for good.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Held is how many deleted tasks the trash holds, counted even while the
	// trash is off.
	Held int `json:"held" yaml:"held"`
}

// ErrTrashOff is the detail key a refusal carries when the tenant's trash is
// turned off, so a client can tell it from any other failed precondition.
const ErrTrashOff = "trash_off"

// TrashIsOff is the refusal for asking a tenant with no trash for its trash.
func TrashIsOff(what string) *Error {
	return Precondition("trash is turned off for this tenant, so %s; a tenant admin can turn it back on", what).
		WithDetail("reason", ErrTrashOff)
}
