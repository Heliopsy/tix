package core

import (
	"slices"
	"time"
)

// Input types for Service methods.

// CreateTenantInput creates a tenant.
type CreateTenantInput struct {
	Key  string `json:"key" yaml:"key"`
	Name string `json:"name" yaml:"name"`
}

// Validate checks the input.
func (in CreateTenantInput) Validate() error {
	if err := ValidateProjectKey(in.Key); err != nil {
		return Invalid("tenant key: %v", err)
	}
	if in.Name == "" {
		return Invalid("tenant name is required")
	}
	return nil
}

// UpdateTenantInput changes a tenant.
type UpdateTenantInput struct {
	Name *string `json:"name,omitempty" yaml:"name,omitempty"`
}

// AddDomainInput maps a hostname to the current tenant.
type AddDomainInput struct {
	Hostname string   `json:"hostname" yaml:"hostname"`
	CertMode CertMode `json:"cert_mode,omitempty" yaml:"cert_mode,omitempty"`
	CertPath string   `json:"cert_path,omitempty" yaml:"cert_path,omitempty"`
	KeyPath  string   `json:"key_path,omitempty" yaml:"key_path,omitempty"`
}

// CreateProjectInput creates a project.
type CreateProjectInput struct {
	Key         string `json:"key" yaml:"key"`
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	WorkflowKey string `json:"workflow_key,omitempty" yaml:"workflow_key,omitempty"`
	Color       string `json:"color,omitempty" yaml:"color,omitempty"`
	Icon        string `json:"icon,omitempty" yaml:"icon,omitempty"`
}

// Validate checks the input.
func (in CreateProjectInput) Validate() error {
	if err := ValidateProjectKey(in.Key); err != nil {
		return err
	}
	if in.Name == "" {
		return Invalid("project name is required")
	}
	if _, err := NormalizeProjectColor(in.Color); err != nil {
		return err
	}
	if _, err := NormalizeProjectIcon(in.Icon); err != nil {
		return err
	}
	return nil
}

// UpdateProjectInput changes a project. A pointer to an empty colour or icon
// clears it.
type UpdateProjectInput struct {
	Name        *string `json:"name,omitempty" yaml:"name,omitempty"`
	Description *string `json:"description,omitempty" yaml:"description,omitempty"`
	WorkflowKey *string `json:"workflow_key,omitempty" yaml:"workflow_key,omitempty"`
	Color       *string `json:"color,omitempty" yaml:"color,omitempty"`
	Icon        *string `json:"icon,omitempty" yaml:"icon,omitempty"`
}

// FieldDefInput defines or redefines a custom field.
type FieldDefInput struct {
	Key         string    `json:"key" yaml:"key"`
	Label       string    `json:"label" yaml:"label"`
	Type        FieldType `json:"type" yaml:"type"`
	Required    bool      `json:"required" yaml:"required"`
	EnumOptions []string  `json:"enum_options,omitempty" yaml:"enum_options,omitempty"`
	Default     any       `json:"default,omitempty" yaml:"default,omitempty"`
	Indexed     bool      `json:"indexed" yaml:"indexed"`
	Position    int       `json:"position" yaml:"position"`
}

// Validate checks the input.
func (in FieldDefInput) Validate() error {
	if in.Key == "" {
		return Invalid("field key is required")
	}
	if !in.Type.Valid() {
		return Invalid("field type %q is not supported", in.Type)
	}
	if in.Type == FieldEnum && len(in.EnumOptions) == 0 {
		return Invalid("an enum field requires at least one option")
	}
	if in.Type != FieldEnum && len(in.EnumOptions) > 0 {
		return Invalid("enum options are only valid on an enum field")
	}
	return nil
}

// WorkflowInput defines or redefines a workflow.
type WorkflowInput struct {
	Key        string             `json:"key" yaml:"key"`
	Name       string             `json:"name" yaml:"name"`
	Definition WorkflowDefinition `json:"definition" yaml:"definition"`
	Migrate    map[string]string  `json:"migrate,omitempty" yaml:"migrate,omitempty"`
}

// Validate checks the workflow definition for internal consistency.
func (in WorkflowInput) Validate() error {
	if in.Key == "" {
		return Invalid("workflow key is required")
	}
	d := in.Definition
	if len(d.States) == 0 {
		return Invalid("a workflow requires at least one state")
	}
	if d.Initial == "" {
		return Invalid("a workflow requires an initial state")
	}
	if !d.HasState(d.Initial) {
		return Invalid("initial state %q is not defined", d.Initial)
	}

	seen := make(map[string]bool, len(d.States))
	for _, s := range d.States {
		if s.Key == "" {
			return Invalid("every state requires a key")
		}
		if seen[s.Key] {
			return Invalid("state %q is defined more than once", s.Key)
		}
		seen[s.Key] = true
		if s.RevertTo != "" && !d.HasState(s.RevertTo) {
			return Invalid("state %q reverts to undefined state %q", s.Key, s.RevertTo)
		}
	}

	for _, t := range d.Transitions {
		if !seen[t.From] {
			return Invalid("transition references undefined state %q", t.From)
		}
		if !seen[t.To] {
			return Invalid("transition references undefined state %q", t.To)
		}
		if t.From == t.To {
			return Invalid("transition from %q to itself is not meaningful", t.From)
		}
	}

	if len(d.TerminalStates()) == 0 {
		return Invalid("a workflow requires at least one terminal state")
	}
	return nil
}

// CreateTaskInput creates a task.
type CreateTaskInput struct {
	ProjectRef string   `json:"project_ref,omitempty" yaml:"project_ref,omitempty"`
	Title      string   `json:"title" yaml:"title"`
	Body       string   `json:"body,omitempty" yaml:"body,omitempty"`
	Status     string   `json:"status,omitempty" yaml:"status,omitempty"`
	Priority   Priority `json:"priority,omitempty" yaml:"priority,omitempty"`
	Tags       []string `json:"tags,omitempty" yaml:"tags,omitempty"`

	AssigneeActorID string     `json:"assignee_actor_id,omitempty" yaml:"assignee_actor_id,omitempty"`
	ParentRef       string     `json:"parent_ref,omitempty" yaml:"parent_ref,omitempty"`
	DueAt           *time.Time `json:"due_at,omitempty" yaml:"due_at,omitempty"`

	CustomFields map[string]any `json:"custom_fields,omitempty" yaml:"custom_fields,omitempty"`
	DependsOn    []string       `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
}

// Validate checks the input.
func (in CreateTaskInput) Validate() error {
	if in.Title == "" {
		return Invalid("task title is required")
	}
	if len(in.Title) > MaxTitleLength {
		return Invalid("task title must be at most %d characters", MaxTitleLength)
	}
	if in.Priority != 0 && !in.Priority.Valid() {
		return Invalid("priority %d is out of range", in.Priority)
	}
	return nil
}

// Field length limits.
const (
	MaxTitleLength   = 500
	MaxBodyLength    = 1 << 20
	MaxCommentLength = 1 << 16
	MaxBlobSize      = 1 << 20
)

// DeleteTaskInput says how far a delete reaches. Cascade is required to
// delete a task that still has children; without it such a delete is refused
// so a subtree is never removed by accident.
type DeleteTaskInput struct {
	Hard    bool `json:"hard,omitempty" yaml:"hard,omitempty"`
	Cascade bool `json:"cascade,omitempty" yaml:"cascade,omitempty"`
}

// UpdateTaskInput changes a task.
type UpdateTaskInput struct {
	Title           *string        `json:"title,omitempty" yaml:"title,omitempty"`
	Body            *string        `json:"body,omitempty" yaml:"body,omitempty"`
	Priority        *Priority      `json:"priority,omitempty" yaml:"priority,omitempty"`
	AssigneeActorID *string        `json:"assignee_actor_id,omitempty" yaml:"assignee_actor_id,omitempty"`
	ParentRef       *string        `json:"parent_ref,omitempty" yaml:"parent_ref,omitempty"`
	DueAt           **time.Time    `json:"due_at,omitempty" yaml:"due_at,omitempty"`
	CustomFields    map[string]any `json:"custom_fields,omitempty" yaml:"custom_fields,omitempty"`
	Tags            *[]string      `json:"tags,omitempty" yaml:"tags,omitempty"`

	Version int `json:"version,omitempty" yaml:"version,omitempty"`
}

// TransitionInput moves a task to a new status.
type TransitionInput struct {
	To           string         `json:"to" yaml:"to"`
	Comment      string         `json:"comment,omitempty" yaml:"comment,omitempty"`
	LeaseToken   string         `json:"lease_token,omitempty" yaml:"lease_token,omitempty"`
	CustomFields map[string]any `json:"custom_fields,omitempty" yaml:"custom_fields,omitempty"`
	Version      int            `json:"version,omitempty" yaml:"version,omitempty"`
}

// Validate checks the input.
func (in TransitionInput) Validate() error {
	if in.To == "" {
		return Invalid("target status is required")
	}
	return nil
}

// ArtifactInput records structured output on a task.
type ArtifactInput struct {
	Kind        ArtifactKind   `json:"kind" yaml:"kind"`
	Name        string         `json:"name,omitempty" yaml:"name,omitempty"`
	Payload     map[string]any `json:"payload,omitempty" yaml:"payload,omitempty"`
	ContentType string         `json:"content_type,omitempty" yaml:"content_type,omitempty"`
	Blob        []byte         `json:"blob,omitempty" yaml:"blob,omitempty"`
}

// Validate checks the input.
func (in ArtifactInput) Validate() error {
	if in.Kind == "" {
		return Invalid("artifact kind is required")
	}
	if len(in.Blob) > MaxBlobSize {
		return Invalid("inline artifact blob must be at most %d bytes; store large content externally and reference it from the payload", MaxBlobSize)
	}
	return nil
}

// ClaimInput claims one task.
type ClaimInput struct {
	TTL     Duration `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	ActorID string   `json:"actor_id,omitempty" yaml:"actor_id,omitempty"`
}

// ClaimNextInput claims the next eligible task.
type ClaimNextInput struct {
	ProjectRefs []string `json:"project_refs,omitempty" yaml:"project_refs,omitempty"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Statuses    []string `json:"statuses,omitempty" yaml:"statuses,omitempty"`
	TTL         Duration `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	ActorID     string   `json:"actor_id,omitempty" yaml:"actor_id,omitempty"`
}

// ReleaseInput gives up a lease.
type ReleaseInput struct {
	Status  string         `json:"status,omitempty" yaml:"status,omitempty"`
	Result  map[string]any `json:"result,omitempty" yaml:"result,omitempty"`
	Comment string         `json:"comment,omitempty" yaml:"comment,omitempty"`
}

// CreateUserInput creates a user.
type CreateUserInput struct {
	Email       string `json:"email" yaml:"email"`
	Password    string `json:"password,omitempty" yaml:"password,omitempty"`
	DisplayName string `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	Handle      string `json:"handle,omitempty" yaml:"handle,omitempty"`
	Role        Role   `json:"role,omitempty" yaml:"role,omitempty"`
}

// Validate checks the input.
func (in CreateUserInput) Validate() error {
	if in.Email == "" {
		return Invalid("email is required")
	}
	if in.Role != "" && !in.Role.Valid() {
		return Invalid("role %q is not recognised", in.Role)
	}
	if in.Password != "" && len(in.Password) < MinPasswordLength {
		return Invalid("password must be at least %d characters", MinPasswordLength)
	}
	return nil
}

// MinPasswordLength is the shortest accepted password.
const MinPasswordLength = 12

// UpdateUserInput changes a user.
type UpdateUserInput struct {
	DisplayName *string `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	Password    *string `json:"password,omitempty" yaml:"password,omitempty"`
	Role        *Role   `json:"role,omitempty" yaml:"role,omitempty"`
	Disabled    *bool   `json:"disabled,omitempty" yaml:"disabled,omitempty"`
}

// CreateTokenInput mints an API token.
type CreateTokenInput struct {
	Name      string     `json:"name" yaml:"name"`
	ActorID   string     `json:"actor_id,omitempty" yaml:"actor_id,omitempty"`
	Scopes    []Scope    `json:"scopes" yaml:"scopes"`
	ProjectID string     `json:"project_id,omitempty" yaml:"project_id,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty" yaml:"expires_at,omitempty"`
}

// Validate checks the input.
func (in CreateTokenInput) Validate() error {
	if in.Name == "" {
		return Invalid("token name is required")
	}
	if len(in.Scopes) == 0 {
		return Invalid("a token requires at least one scope")
	}
	return nil
}

// WebhookInput registers or updates a delivery endpoint.
type WebhookInput struct {
	ID         string   `json:"id,omitempty" yaml:"id,omitempty"`
	URL        string   `json:"url" yaml:"url"`
	Secret     string   `json:"secret,omitempty" yaml:"secret,omitempty"`
	EventTypes []string `json:"event_types,omitempty" yaml:"event_types,omitempty"`
	Active     bool     `json:"active" yaml:"active"`
}

// Validate checks the input.
func (in WebhookInput) Validate() error {
	if in.URL == "" {
		return Invalid("webhook url is required")
	}
	return nil
}

// ExportInput selects what to export.
type ExportInput struct {
	ProjectRefs      []string `json:"project_refs,omitempty" yaml:"project_refs,omitempty"`
	IncludeArtifacts bool     `json:"include_artifacts,omitempty" yaml:"include_artifacts,omitempty"`
	IncludeComments  bool     `json:"include_comments,omitempty" yaml:"include_comments,omitempty"`
}

// ImportInput controls an import.
type ImportInput struct {
	Mode   ImportMode `json:"mode" yaml:"mode"`
	DryRun bool       `json:"dry_run,omitempty" yaml:"dry_run,omitempty"`
}

// Validate checks the input.
func (in ImportInput) Validate() error {
	switch {
	case in.Mode.Valid():
		return nil
	case in.Mode == "":
		return Invalid("import mode is required; there is no default because replace is destructive")
	default:
		return Invalid("import mode %q must be %q or %q", in.Mode, ImportMerge, ImportReplace)
	}
}

// SyncSourceInput registers an external import source.
type SyncSourceInput struct {
	ID          string         `json:"id,omitempty" yaml:"id,omitempty"`
	System      string         `json:"system" yaml:"system"`
	Name        string         `json:"name" yaml:"name"`
	Config      map[string]any `json:"config,omitempty" yaml:"config,omitempty"`
	MappingPath string         `json:"mapping_path,omitempty" yaml:"mapping_path,omitempty"`
}

// Supported external systems.
const (
	SystemGeneric     = "generic"
	SystemJira        = "jira"
	SystemOpenProject = "openproject"
)

// SyncSystems lists every supported external system.
var SyncSystems = []string{SystemGeneric, SystemJira, SystemOpenProject}

// Validate checks the input.
func (in SyncSourceInput) Validate() error {
	switch {
	case slices.Contains(SyncSystems, in.System):
	default:
		return Invalid("system %q must be one of %q, %q or %q",
			in.System, SystemGeneric, SystemJira, SystemOpenProject)
	}
	if in.Name == "" {
		return Invalid("sync source name is required")
	}
	return nil
}

// RunSyncInput runs an import from a configured source.
type RunSyncInput struct {
	SourceID string `json:"source_id" yaml:"source_id"`
	DryRun   bool   `json:"dry_run,omitempty" yaml:"dry_run,omitempty"`
	Full     bool   `json:"full,omitempty" yaml:"full,omitempty"`
}

// PruneInput removes records past their retention window.
type PruneInput struct {
	DryRun bool `json:"dry_run,omitempty" yaml:"dry_run,omitempty"`
	Limit  int  `json:"limit,omitempty" yaml:"limit,omitempty"`
}
