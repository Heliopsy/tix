package core

import "time"

// Input types for Service methods.
//
// Update inputs use pointer fields so that "set this field to its zero value"
// is distinguishable from "leave this field alone". A partial update that could
// not express the difference would silently blank fields the caller never
// mentioned.

// CreateTenantInput creates a tenant.
type CreateTenantInput struct {
	Key  string `json:"key"`
	Name string `json:"name"`
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
	Name *string `json:"name,omitempty"`
}

// AddDomainInput maps a hostname to the current tenant.
type AddDomainInput struct {
	Hostname string   `json:"hostname"`
	CertMode CertMode `json:"cert_mode,omitempty"`
	CertPath string   `json:"cert_path,omitempty"`
	KeyPath  string   `json:"key_path,omitempty"`
}

// CreateProjectInput creates a project.
type CreateProjectInput struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// WorkflowKey names the workflow to assign. Empty assigns the builtin default.
	WorkflowKey string `json:"workflow_key,omitempty"`
}

// Validate checks the input.
func (in CreateProjectInput) Validate() error {
	if err := ValidateProjectKey(in.Key); err != nil {
		return err
	}
	if in.Name == "" {
		return Invalid("project name is required")
	}
	return nil
}

// UpdateProjectInput changes a project.
type UpdateProjectInput struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	WorkflowKey *string `json:"workflow_key,omitempty"`
}

// FieldDefInput defines or redefines a custom field.
type FieldDefInput struct {
	Key         string    `json:"key"`
	Label       string    `json:"label"`
	Type        FieldType `json:"type"`
	Required    bool      `json:"required"`
	EnumOptions []string  `json:"enum_options,omitempty"`
	Default     any       `json:"default,omitempty"`
	Indexed     bool      `json:"indexed"`
	Position    int       `json:"position"`
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
	Key        string             `json:"key"`
	Name       string             `json:"name"`
	Definition WorkflowDefinition `json:"definition"`
	// Migrate maps states being removed onto surviving ones. Without it, a
	// change that would orphan existing tasks is rejected.
	Migrate map[string]string `json:"migrate,omitempty"`
}

// Validate checks the workflow definition for internal consistency. It does not
// check it against existing tasks; the service does that.
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

	// A workflow with no terminal state can never satisfy a dependency, so
	// every task in it would block its dependents forever.
	if len(d.TerminalStates()) == 0 {
		return Invalid("a workflow requires at least one terminal state")
	}
	return nil
}

// CreateTaskInput creates a task. Only Title is required.
type CreateTaskInput struct {
	// ProjectRef names the project by key or identifier. Empty uses the
	// context's default project.
	ProjectRef string `json:"project_ref,omitempty"`
	Title      string `json:"title"`
	Body       string `json:"body,omitempty"`
	// Status defaults to the workflow's initial state.
	Status   string   `json:"status,omitempty"`
	Priority Priority `json:"priority,omitempty"`
	Labels   []string `json:"labels,omitempty"`

	AssigneeActorID string     `json:"assignee_actor_id,omitempty"`
	ParentRef       string     `json:"parent_ref,omitempty"`
	DueAt           *time.Time `json:"due_at,omitempty"`

	CustomFields map[string]any `json:"custom_fields,omitempty"`
	DependsOn    []string       `json:"depends_on,omitempty"`
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

// Field length limits, enforced so a single record cannot be used to bloat the
// database or a response.
const (
	MaxTitleLength   = 500
	MaxBodyLength    = 1 << 20
	MaxCommentLength = 1 << 16
	MaxBlobSize      = 1 << 20
)

// UpdateTaskInput changes a task. Nil fields are left alone.
type UpdateTaskInput struct {
	Title           *string        `json:"title,omitempty"`
	Body            *string        `json:"body,omitempty"`
	Priority        *Priority      `json:"priority,omitempty"`
	AssigneeActorID *string        `json:"assignee_actor_id,omitempty"`
	ParentRef       *string        `json:"parent_ref,omitempty"`
	DueAt           **time.Time    `json:"due_at,omitempty"`
	CustomFields    map[string]any `json:"custom_fields,omitempty"`
	Labels          *[]string      `json:"labels,omitempty"`

	// Version, when non-zero, enforces optimistic concurrency: the update
	// applies only if the task is still at this version.
	Version int `json:"version,omitempty"`
}

// TransitionInput moves a task to a new status.
type TransitionInput struct {
	To      string `json:"to"`
	Comment string `json:"comment,omitempty"`
	// LeaseToken is required when the task is under a live lease.
	LeaseToken   string         `json:"lease_token,omitempty"`
	CustomFields map[string]any `json:"custom_fields,omitempty"`
	Version      int            `json:"version,omitempty"`
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
	Kind        ArtifactKind   `json:"kind"`
	Name        string         `json:"name,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
	ContentType string         `json:"content_type,omitempty"`
	Blob        []byte         `json:"blob,omitempty"`
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
	// TTL overrides the workflow's default lease duration.
	TTL Duration `json:"ttl,omitempty"`
	// ActorID claims on behalf of another actor, which requires admin scope.
	ActorID string `json:"actor_id,omitempty"`
}

// ClaimNextInput claims the next eligible task.
type ClaimNextInput struct {
	ProjectRefs []string `json:"project_refs,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	Statuses    []string `json:"statuses,omitempty"`
	TTL         Duration `json:"ttl,omitempty"`
	ActorID     string   `json:"actor_id,omitempty"`
}

// ReleaseInput gives up a lease.
type ReleaseInput struct {
	// Status, when set, transitions the task as part of releasing.
	Status string `json:"status,omitempty"`
	// Result, when set, is recorded as a result artifact.
	Result map[string]any `json:"result,omitempty"`
	// Comment, when set, is added to the task.
	Comment string `json:"comment,omitempty"`
}

// CreateUserInput creates a user.
type CreateUserInput struct {
	Email       string `json:"email"`
	Password    string `json:"password,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Handle      string `json:"handle,omitempty"`
	// Role grants membership in the current tenant.
	Role Role `json:"role,omitempty"`
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
	DisplayName *string `json:"display_name,omitempty"`
	Password    *string `json:"password,omitempty"`
	Role        *Role   `json:"role,omitempty"`
	Disabled    *bool   `json:"disabled,omitempty"`
}

// CreateTokenInput mints an API token.
type CreateTokenInput struct {
	Name string `json:"name"`
	// ActorID defaults to the calling actor.
	ActorID   string     `json:"actor_id,omitempty"`
	Scopes    []Scope    `json:"scopes"`
	ProjectID string     `json:"project_id,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
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
	ID         string   `json:"id,omitempty"`
	URL        string   `json:"url"`
	Secret     string   `json:"secret,omitempty"`
	EventTypes []string `json:"event_types,omitempty"`
	Active     bool     `json:"active"`
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
	ProjectRefs []string `json:"project_refs,omitempty"`
	// IncludeArtifacts includes artifact payloads and inline blobs, which can
	// be large.
	IncludeArtifacts bool `json:"include_artifacts,omitempty"`
	IncludeComments  bool `json:"include_comments,omitempty"`
}

// ImportInput controls an import.
type ImportInput struct {
	Mode   ImportMode `json:"mode"`
	DryRun bool       `json:"dry_run,omitempty"`
}

// Validate checks the input.
func (in ImportInput) Validate() error {
	switch in.Mode {
	case ImportMerge, ImportReplace:
		return nil
	case "":
		return Invalid("import mode is required; there is no default because replace is destructive")
	default:
		return Invalid("import mode %q must be %q or %q", in.Mode, ImportMerge, ImportReplace)
	}
}

// SyncSourceInput registers an external import source.
type SyncSourceInput struct {
	ID     string `json:"id,omitempty"`
	System string `json:"system"`
	Name   string `json:"name"`
	// Config holds adapter settings such as the base URL and project mapping.
	// Credentials are supplied by environment or config, never stored here.
	Config map[string]any `json:"config,omitempty"`
	// MappingPath points at the declarative YAML mapping file.
	MappingPath string `json:"mapping_path,omitempty"`
}

// Supported external systems.
const (
	SystemGeneric     = "generic"
	SystemJira        = "jira"
	SystemOpenProject = "openproject"
)

// Validate checks the input.
func (in SyncSourceInput) Validate() error {
	switch in.System {
	case SystemGeneric, SystemJira, SystemOpenProject:
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
	SourceID string `json:"source_id"`
	// DryRun reports what would change without writing anything.
	DryRun bool `json:"dry_run,omitempty"`
	// Full ignores the stored cursor and re-reads everything.
	Full bool `json:"full,omitempty"`
}

// PruneInput removes records past their retention window.
type PruneInput struct {
	DryRun bool `json:"dry_run,omitempty"`
	// Limit bounds how many rows a single run removes, so pruning a large
	// backlog can be spread over several runs.
	Limit int `json:"limit,omitempty"`
}
