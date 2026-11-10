// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"time"
)

// Input types for Service methods.

// CreateTenantInput creates a tenant.
type CreateTenantInput struct {
	// Key is the tenant's short unique key, lowercased: letters, digits, hyphens
	// and underscores, starting with a letter. Required.
	Key string `json:"key" yaml:"key"`
	// Name is the tenant's display name. Required.
	Name string `json:"name" yaml:"name"`
	// Admins are the first administrators of the new tenant, each given the
	// admin role there in the same operation as the tenant. A new address
	// becomes a user; an address that already has an account keeps it, with
	// its password and second factor. Omitted creates a tenant with no members.
	Admins []TenantAdminInput `json:"admins,omitempty" yaml:"admins,omitempty"`
}

// MaxTenantAdmins bounds the administrators one tenant creation may name.
// Each one costs a password hash, so the bound is what keeps one request from
// buying an unbounded amount of hashing.
const MaxTenantAdmins = 10

// Validate checks the input.
func (in CreateTenantInput) Validate() error {
	if err := ValidateProjectKey(in.Key); err != nil {
		return Invalid("tenant key: %v", err)
	}
	if in.Name == "" {
		return Invalid("tenant name is required")
	}
	if len(in.Admins) > MaxTenantAdmins {
		return Invalid("a tenant can be created with at most %d admins", MaxTenantAdmins)
	}
	emails := map[string]bool{}
	handles := map[string]bool{}
	for _, a := range in.Admins {
		if err := a.Validate(); err != nil {
			return err
		}
		email := NormalEmail(a.Email)
		if emails[email] {
			return Invalid("admin %q is named twice", email).WithDetail(DetailField, "admins")
		}
		emails[email] = true
		handle := a.HandleOrDefault()
		if handles[handle] {
			return Invalid("two admins would share the handle %q; give one a handle", handle).
				WithDetail(DetailField, "admins")
		}
		handles[handle] = true
	}
	return nil
}

// TenantAdminInput names one first administrator of a new tenant.
type TenantAdminInput struct {
	// Email is the administrator's sign-in address, lowercased. Required.
	Email string `json:"email" yaml:"email"`
	// Handle is the administrator's short name in the new tenant, lowercased;
	// empty uses the part of the email before the "@".
	Handle string `json:"handle,omitempty" yaml:"handle,omitempty"`
	// DisplayName is the name shown for the administrator.
	DisplayName string `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	// Password is at least 12 characters. Empty generates a random one, which
	// the response carries once in initial_credentials and nothing stores.
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
}

// Validate checks one administrator.
func (in TenantAdminInput) Validate() error {
	if err := ValidateEmail(in.Email); err != nil {
		return err
	}
	if in.HandleOrDefault() == "" {
		return Invalid("admin %q needs a handle", in.Email)
	}
	if in.Password != "" && len(in.Password) < MinPasswordLength {
		return Invalid("password for admin %q must be at least %d characters", NormalEmail(in.Email), MinPasswordLength)
	}
	return nil
}

// HandleOrDefault is the handle the administrator will get: the one given, or
// the local part of the email.
func (in TenantAdminInput) HandleOrDefault() string {
	if h := strings.ToLower(strings.TrimSpace(in.Handle)); h != "" {
		return h
	}
	local, _, _ := strings.Cut(NormalEmail(in.Email), "@")
	return local
}

// NormalEmail is an address as it is stored: trimmed and lowercased.
func NormalEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// ValidateEmail refuses an address that is plainly not one: no "@", nothing on
// either side of it, or whitespace inside. Deliverability is not checked.
func ValidateEmail(email string) error {
	e := NormalEmail(email)
	if e == "" {
		return Invalid("email is required").WithDetail(DetailField, "email")
	}
	local, domain, ok := strings.Cut(e, "@")
	if !ok || local == "" || domain == "" || strings.Contains(domain, "@") ||
		strings.ContainsAny(e, " \t\r\n") {
		return Invalid("%q is not an email address", e).WithDetail(DetailField, "email")
	}
	return nil
}

// CreatedTenant is a tenant just created, with the administrators created
// alongside it.
type CreatedTenant struct {
	Tenant
	// Admins are the administrators created with the tenant, in the order
	// they were named.
	Admins []CreatedAdmin `json:"admins,omitempty" yaml:"admins,omitempty"`
	// InitialCredentials carries every password the server generated, in
	// plain text, exactly once. Nothing stores it and no later call returns
	// it; a caller that discards it has to set a new password.
	InitialCredentials []InitialCredential `json:"initial_credentials,omitempty" yaml:"initial_credentials,omitempty"`
}

// CreatedAdmin is one administrator created with a tenant.
type CreatedAdmin struct {
	// UserID is the new user's identifier, which is also its actor's.
	UserID string `json:"user_id" yaml:"user_id"`
	// Email is the administrator's sign-in address.
	Email string `json:"email" yaml:"email"`
	// Handle is the administrator's handle in the new tenant.
	Handle string `json:"handle" yaml:"handle"`
	// PasswordGenerated is true when the server chose the password, which is
	// then in InitialCredentials.
	PasswordGenerated bool `json:"password_generated" yaml:"password_generated"`
}

// InitialCredential is a generated password, shown once.
type InitialCredential struct {
	// Email is the address the password signs in with.
	Email string `json:"email" yaml:"email"`
	// Handle is the administrator's handle in the new tenant.
	Handle string `json:"handle" yaml:"handle"`
	// Password is the generated password in plain text.
	Password string `json:"password" yaml:"password"`
}

// TenantUser is a user of some tenant together with their role there.
type TenantUser struct {
	User
	// Role is the user's role in that tenant; empty when they hold no
	// membership there.
	Role Role `json:"role,omitempty" yaml:"role,omitempty"`
}

// GlobalAdminInput names the user a global admin grant concerns.
type GlobalAdminInput struct {
	// Email is the user's sign-in address. Required.
	Email string `json:"email" yaml:"email"`
}

// AddTenantMemberInput adds an existing account to a tenant.
type AddTenantMemberInput struct {
	// Email is the account's sign-in address. Required.
	Email string `json:"email" yaml:"email"`
	// Role is the role granted in the tenant; empty means "viewer".
	Role Role `json:"role,omitempty" yaml:"role,omitempty"`
	// Handle is the person's short name in the tenant; empty uses the part of
	// the email before the "@".
	Handle string `json:"handle,omitempty" yaml:"handle,omitempty"`
}

// Validate checks the input.
func (in AddTenantMemberInput) Validate() error {
	if err := ValidateEmail(in.Email); err != nil {
		return err
	}
	if in.Role != "" && !in.Role.Valid() {
		return Invalid("role %q is not recognised", in.Role).WithDetail(DetailField, "role")
	}
	return nil
}

// UpdateTenantInput changes a tenant.
type UpdateTenantInput struct {
	// Name renames the tenant; it must not be blank, and omitted leaves it.
	Name *string `json:"name,omitempty" yaml:"name,omitempty"`
	// Theme names the palette this tenant presents itself with. A pointer to
	// the empty string clears it, which returns the tenant to the accent
	// derived from its own identity; nil leaves it alone.
	Theme *string `json:"theme,omitempty" yaml:"theme,omitempty"`
	// RequireTOTPWeb turns the second-factor requirement for browser sign-in
	// on or off; omitted leaves it alone.
	//
	// The requirement is a property of the tenant rather than a command of its
	// own, so there is one place to get its authorization right. Turning one
	// on cannot lock the tenant out: a user subject to it who holds no
	// enrolment is taken through enrolment at sign-in rather than refused.
	RequireTOTPWeb *bool `json:"require_totp_web,omitempty" yaml:"require_totp_web,omitempty"`
	// RequireTOTPCLI turns the second-factor requirement for command line
	// sign-in on or off; omitted leaves it alone.
	RequireTOTPCLI *bool `json:"require_totp_cli,omitempty" yaml:"require_totp_cli,omitempty"`
	// RequireTOTPSSH turns the second-factor requirement for SSH sign-in on or
	// off; omitted leaves it alone.
	RequireTOTPSSH *bool `json:"require_totp_ssh,omitempty" yaml:"require_totp_ssh,omitempty"`
	// MFAPolicy sets the second factor policy, "off" or "required"; omitted
	// leaves it. Turning it required is refused while the caller is a person
	// who holds no second factor, so nobody locks themselves out.
	MFAPolicy *MFAPolicy `json:"mfa_policy,omitempty" yaml:"mfa_policy,omitempty"`
	// MFAGrace is how long after the policy turned required people without a
	// factor may still work, a duration such as "7d"; zero gates at once and
	// omitted leaves it.
	MFAGrace *Duration `json:"mfa_grace,omitempty" yaml:"mfa_grace,omitempty"`
	// LeaseDefault is the lease a claim on this tenant gets when nothing more
	// specific names one, a duration such as "30m". Zero clears it back to the
	// deployment configuration and omitted leaves it alone. A default longer
	// than the maximum the tenant would hold, sent or stored, is refused.
	LeaseDefault *Duration `json:"lease_default,omitempty" yaml:"lease_default,omitempty"`
	// LeaseMax is the longest lease a claim on this tenant may hold, a
	// duration such as "4h". Zero clears it back to the deployment
	// configuration and omitted leaves it alone.
	LeaseMax *Duration `json:"lease_max,omitempty" yaml:"lease_max,omitempty"`
	// ArtifactMaxBytes lowers the largest artifact blob this tenant accepts
	// below the server's limit. Zero inherits the server's and omitted leaves
	// it; a value above the server's is refused.
	ArtifactMaxBytes *int64 `json:"artifact_max_bytes,omitempty" yaml:"artifact_max_bytes,omitempty"`
	// ArtifactTypes narrows the media types an artifact blob may carry, such
	// as ["image/*", "application/pdf"]. An empty list inherits the server's
	// and omitted leaves it; a type the server does not accept is refused.
	ArtifactTypes *[]string `json:"artifact_types,omitempty" yaml:"artifact_types,omitempty"`
	// TrashOff turns the trash off (true) or back on (false); nil leaves it
	// alone. Turning it off keeps the tasks already in the trash.
	TrashOff *bool `json:"trash_off,omitempty" yaml:"trash_off,omitempty"`
	// TimeFormat is the date format this tenant's people read unless they
	// chose their own: iso, rfc3339, short, long, dotted, dayfirst, us or
	// relative. "" returns it to the deployment default; nil leaves it.
	TimeFormat *string `json:"time_format,omitempty" yaml:"time_format,omitempty"`
	// Timezone is the IANA zone this tenant's people read dates in unless
	// they chose their own. "" returns it to the deployment default; nil
	// leaves it.
	Timezone *string `json:"timezone,omitempty" yaml:"timezone,omitempty"`
}

// Validate checks the input.
func (in UpdateTenantInput) Validate() error {
	if err := ValidateTimePrefs(in.TimeFormat, in.Timezone); err != nil {
		return err
	}
	if in.MFAPolicy != nil && !in.MFAPolicy.Valid() {
		return Invalid("second factor policy %q is not one of off, required", *in.MFAPolicy).
			WithDetail(DetailField, "mfa_policy")
	}
	if in.MFAGrace != nil && *in.MFAGrace < 0 {
		return Invalid("second factor grace must not be negative").WithDetail(DetailField, "mfa_grace")
	}
	if in.LeaseDefault != nil && *in.LeaseDefault < 0 {
		return Invalid("lease default must not be negative")
	}
	if in.LeaseMax != nil && *in.LeaseMax < 0 {
		return Invalid("lease maximum must not be negative")
	}
	if in.ArtifactMaxBytes != nil && *in.ArtifactMaxBytes < 0 {
		return Invalid("artifact size limit must not be negative").WithDetail(DetailField, "artifact_max_bytes")
	}
	if in.ArtifactTypes != nil {
		if _, err := ParseArtifactTypes(*in.ArtifactTypes); err != nil {
			return err
		}
	}
	// A default above the tenant's own ceiling is refused here rather than
	// clamped, because both figures are in front of whoever is setting them:
	// this is the one moment the contradiction can be pointed at. Resolve
	// clamps instead, since by then the two may have drifted apart through a
	// deployment ceiling nobody on this tenant can see.
	if in.LeaseDefault != nil && in.LeaseMax != nil &&
		*in.LeaseDefault > 0 && *in.LeaseMax > 0 && *in.LeaseDefault > *in.LeaseMax {
		return Invalid("lease default %s is longer than the lease maximum %s", *in.LeaseDefault, *in.LeaseMax)
	}
	return nil
}

// AddDomainInput maps a hostname to the current tenant.
type AddDomainInput struct {
	// Hostname is the host requests for this tenant arrive on. It is
	// lowercased and any port or trailing dot is dropped. Required.
	Hostname string `json:"hostname" yaml:"hostname"`
	// CertMode says how the host's TLS certificate is obtained; empty means
	// "none".
	CertMode CertMode `json:"cert_mode,omitempty" yaml:"cert_mode,omitempty"`
	// CertPath is the server-side path of the PEM certificate, required when
	// CertMode is "file".
	CertPath string `json:"cert_path,omitempty" yaml:"cert_path,omitempty"`
	// KeyPath is the server-side path of the PEM private key, required when
	// CertMode is "file".
	KeyPath string `json:"key_path,omitempty" yaml:"key_path,omitempty"`
}

// CreateProjectInput creates a project.
type CreateProjectInput struct {
	// Key is the project's unique key and the prefix of its task references,
	// lowercased: letters, digits, hyphens and underscores, starting with a
	// letter. Required.
	Key string `json:"key" yaml:"key"`
	// Name is the project's display name. Required.
	Name string `json:"name" yaml:"name"`
	// Description is free text shown with the project.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// WorkflowKey names the workflow the project's tasks follow; empty uses the
	// builtin workflow.
	WorkflowKey string `json:"workflow_key,omitempty" yaml:"workflow_key,omitempty"`
	// WorkflowTemplate names a built-in workflow template instead of a
	// workflow: the project runs on the workflow the template creates under
	// its own key, reused when the tenant already holds it, and gets the
	// template's custom fields. At most one of WorkflowKey and
	// WorkflowTemplate may be set.
	WorkflowTemplate string `json:"workflow_template,omitempty" yaml:"workflow_template,omitempty"`
	// Color is a palette colour name or a custom "#rrggbb"; empty means none.
	Color string `json:"color,omitempty" yaml:"color,omitempty"`
	// Icon is a text icon: one emoji or a monogram of at most two characters.
	// At most one of Icon, IconName and IconFile may be set.
	Icon string `json:"icon,omitempty" yaml:"icon,omitempty"`
	// IconName names one of the built-in icons. At most one of Icon, IconName
	// and IconFile may be set.
	IconName string `json:"icon_name,omitempty" yaml:"icon_name,omitempty"`

	// IconFile is an uploaded image icon. At most one of Icon, IconName and
	// IconFile may be set.
	IconFile *ProjectIconFile `json:"icon_file,omitempty" yaml:"icon_file,omitempty"`
}

// Validate checks the input.
func (in CreateProjectInput) Validate() error {
	if in.Name == "" {
		return Invalid("project name is required").WithDetail(DetailField, "name")
	}
	if err := ValidateProjectKey(in.Key); err != nil {
		return OnField(err, "key")
	}
	if strings.TrimSpace(in.WorkflowKey) != "" && strings.TrimSpace(in.WorkflowTemplate) != "" {
		return Invalid("name a workflow or a workflow template, not both").WithDetail(DetailField, "workflow_template")
	}
	if _, err := NormalizeProjectColor(in.Color); err != nil {
		return OnField(err, "color")
	}
	if err := ValidateProjectIconChoice(in.Icon, in.IconName, in.IconFile); err != nil {
		return OnField(err, "icon")
	}
	return nil
}

// UpdateProjectInput changes a project. A pointer to an empty colour clears
// it. Naming any icon field replaces the whole icon with that kind, and an
// empty one clears it. A new key renames every task reference and keeps the
// old key as an alias of the project.
type UpdateProjectInput struct {
	// Key renames the project's key, rewriting every task reference and keeping
	// the old key as an alias; omitted leaves it.
	Key *string `json:"key,omitempty" yaml:"key,omitempty"`
	// Name renames the project; omitted leaves it.
	Name *string `json:"name,omitempty" yaml:"name,omitempty"`
	// Description replaces the description; omitted leaves it.
	Description *string `json:"description,omitempty" yaml:"description,omitempty"`
	// WorkflowKey switches the project to another workflow, refused while a
	// task sits in a status the new workflow lacks; omitted leaves it.
	WorkflowKey *string `json:"workflow_key,omitempty" yaml:"workflow_key,omitempty"`
	// Color is a palette colour name or "#rrggbb"; empty clears it and omitted
	// leaves it.
	Color *string `json:"color,omitempty" yaml:"color,omitempty"`
	// Icon sets a text icon, replacing any icon; empty clears the icon and
	// omitted leaves it.
	Icon *string `json:"icon,omitempty" yaml:"icon,omitempty"`
	// IconName sets a built-in icon, replacing any icon; empty clears the icon
	// and omitted leaves it.
	IconName *string `json:"icon_name,omitempty" yaml:"icon_name,omitempty"`

	// IconFile sets an uploaded image icon, replacing any icon; omitted leaves
	// it.
	IconFile *ProjectIconFile `json:"icon_file,omitempty" yaml:"icon_file,omitempty"`

	// LeaseDefault is the lease a claim on this project's tasks gets when the
	// claim and task name none, a duration such as "30m"; zero inherits the
	// tenant's again and omitted leaves it.
	LeaseDefault *Duration `json:"lease_default,omitempty" yaml:"lease_default,omitempty"`
	// LeaseMax caps a lease on this project's tasks below the tenant's
	// ceiling, a duration such as "4h"; zero inherits the tenant's again and
	// omitted leaves it.
	LeaseMax *Duration `json:"lease_max,omitempty" yaml:"lease_max,omitempty"`
}

// IconChanged reports whether the update names the project's icon at all.
func (in UpdateProjectInput) IconChanged() bool {
	return in.Icon != nil || in.IconName != nil || in.IconFile != nil
}

// ValidateIcon checks the icon fields the update names.
func (in UpdateProjectInput) ValidateIcon() error {
	return ValidateProjectIconChoice(deref(in.Icon), deref(in.IconName), in.IconFile)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// DeleteProjectInput says what happens to the tasks of a project being
// deleted. A project holding tasks needs exactly one of the two.
type DeleteProjectInput struct {
	// MoveTasksTo is the key or identifier of the project the tasks move to.
	MoveTasksTo string `json:"move_tasks_to,omitempty" yaml:"move_tasks_to,omitempty"`
	// DeleteTasks removes the tasks permanently with the project.
	DeleteTasks bool `json:"delete_tasks,omitempty" yaml:"delete_tasks,omitempty"`
}

// Validate checks the input for choices that contradict each other.
func (in DeleteProjectInput) Validate() error {
	if in.MoveTasksTo != "" && in.DeleteTasks {
		return Invalid("choose either moving the tasks or deleting them, not both").WithDetail(DetailField, "move_tasks_to")
	}
	return nil
}

// ProjectDeletion is what deleting a project does, or did, to its tasks.
type ProjectDeletion struct {
	// Project is the key of the project being deleted.
	Project string `json:"project" yaml:"project"`
	// Tasks counts the project's live tasks.
	Tasks int `json:"tasks" yaml:"tasks"`
	// Trashed counts the project's soft deleted tasks, in the trash.
	Trashed int `json:"trashed" yaml:"trashed"`
	// Deleted counts the tasks the deletion removed permanently, trashed ones
	// included. It is zero for a move and for a plan, which deletes nothing.
	Deleted int `json:"deleted" yaml:"deleted"`
	// WouldDelete counts the tasks a planned deletion would remove
	// permanently, trashed ones included. Only a plan sets it.
	WouldDelete int `json:"would_delete,omitempty" yaml:"would_delete,omitempty"`
	// MoveTo is the target's key when the tasks move.
	MoveTo string `json:"move_to,omitempty" yaml:"move_to,omitempty"`
	// DeleteTasks reports that the tasks go permanently with the project.
	DeleteTasks bool `json:"delete_tasks,omitempty" yaml:"delete_tasks,omitempty"`
	// Statuses is the mapping a move applies, one entry per source status.
	Statuses []StatusMove `json:"statuses,omitempty" yaml:"statuses,omitempty"`
	// DryRun marks a plan: what the deletion would do, nothing written.
	DryRun bool `json:"dry_run,omitempty" yaml:"dry_run,omitempty"`
}

// Total is every task the deletion reaches, soft deleted ones included.
func (d ProjectDeletion) Total() int { return d.Tasks + d.Trashed }

// StatusMove is one source status, the target status it maps to and how many
// tasks it carries.
type StatusMove struct {
	// From is the status in the deleted project's workflow.
	From string `json:"from" yaml:"from"`
	// To is the status the tasks take in the target project's workflow.
	To string `json:"to" yaml:"to"`
	// Tasks counts the tasks in From.
	Tasks int `json:"tasks" yaml:"tasks"`
}

// FieldDefInput defines or redefines a custom field.
type FieldDefInput struct {
	// Key identifies the field within the project and in a task's
	// custom_fields; an existing key is redefined. Required.
	Key string `json:"key" yaml:"key"`
	// Label is the name shown for the field; empty uses the key.
	Label string `json:"label" yaml:"label"`
	// Type is the kind of value the field holds. Required.
	Type FieldType `json:"type" yaml:"type"`
	// Required refuses a task that ends up with no value and no default.
	Required bool `json:"required" yaml:"required"`
	// EnumOptions lists the allowed values, required for an enum field and
	// refused on any other type.
	EnumOptions []string `json:"enum_options,omitempty" yaml:"enum_options,omitempty"`
	// Default is the value a new task gets when it names none, coerced to Type.
	Default any `json:"default,omitempty" yaml:"default,omitempty"`
	// Position orders the project's fields, lowest first.
	Position int `json:"position" yaml:"position"`
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
	// Key identifies the workflow; an existing key is redefined. Letters,
	// digits, hyphens and underscores, starting with a letter. Required.
	Key string `json:"key" yaml:"key"`
	// Name is the workflow's display name.
	Name string `json:"name" yaml:"name"`
	// Definition is the states and transitions. It needs an initial state that
	// it defines and at least one terminal state.
	Definition WorkflowDefinition `json:"definition" yaml:"definition"`
	// Migrate maps each removed state that still holds tasks to a state of the
	// new definition; a redefinition stranding tasks without one is a conflict.
	Migrate map[string]string `json:"migrate,omitempty" yaml:"migrate,omitempty"`
}

// Validate checks the workflow definition for internal consistency.
func (in WorkflowInput) Validate() error {
	if in.Key == "" {
		return Invalid("workflow key is required")
	}
	if err := ValidateWorkflowKey(in.Key); err != nil {
		return err
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
		if !s.Category.Valid() {
			return Invalid("state %q names category %q, which is not one of %s",
				s.Key, s.Category, JoinStateCategories())
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
	// ProjectRef is the key or identifier of the project to create the task
	// in. Empty picks the tenant's only project, or its "default" project
	// when it has several.
	ProjectRef string `json:"project_ref,omitempty" yaml:"project_ref,omitempty"`
	// Title is a one-line summary of at most 500 characters. Required.
	Title string `json:"title" yaml:"title"`
	// Body is the Markdown description, up to 1 MiB.
	Body string `json:"body,omitempty" yaml:"body,omitempty"`
	// Status is the starting state of the project's workflow; empty uses the
	// workflow's initial state.
	Status string `json:"status,omitempty" yaml:"status,omitempty"`
	// Priority runs from 1 (highest) to 5 (lowest); 0 or omitted means 3.
	Priority Priority `json:"priority,omitempty" yaml:"priority,omitempty"`
	// Tags names the tags to attach, creating any that do not exist.
	Tags []string `json:"tags,omitempty" yaml:"tags,omitempty"`

	// AssigneeActorID is the actor to assign, by identifier or handle.
	AssigneeActorID string `json:"assignee_actor_id,omitempty" yaml:"assignee_actor_id,omitempty"`
	// ParentRef makes the task a subtask of this task reference.
	ParentRef string `json:"parent_ref,omitempty" yaml:"parent_ref,omitempty"`
	// DueAt is when the task is due, an RFC 3339 timestamp.
	DueAt *time.Time `json:"due_at,omitempty" yaml:"due_at,omitempty"`
	// Recurrence is a repeat rule such as "every 2 weeks"; finishing the task
	// creates the next occurrence.
	Recurrence string `json:"recurrence,omitempty" yaml:"recurrence,omitempty"`

	// CustomFields holds values for the project's custom fields, by field key.
	// A field left out takes its default, and a required one without a default
	// is refused.
	CustomFields map[string]any `json:"custom_fields,omitempty" yaml:"custom_fields,omitempty"`
	// DependsOn lists references of tasks this task waits on.
	DependsOn []string `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	// Lease is how long a claim on the task holds; zero inherits the project's.
	Lease Duration `json:"lease,omitempty" yaml:"lease,omitempty"`
}

// Validate checks the input.
func (in CreateTaskInput) Validate() error {
	if in.Title == "" {
		return Invalid("task title is required").WithDetail(DetailField, "title")
	}
	if len(in.Title) > MaxTitleLength {
		return Invalid("task title must be at most %d characters", MaxTitleLength).WithDetail(DetailField, "title")
	}
	if in.Priority != 0 && !in.Priority.Valid() {
		return Invalid("priority %d is out of range", in.Priority).WithDetail(DetailField, "priority")
	}
	return nil
}

// Field length limits.
const (
	MaxTitleLength   = 500
	MaxBodyLength    = 1 << 20
	MaxCommentLength = 1 << 16
)

// DeleteTaskInput says how far a delete reaches. Cascade is required to
// delete a task that still has children; without it such a delete is refused
// so a subtree is never removed by accident.
type DeleteTaskInput struct {
	// Hard removes the task permanently instead of moving it to the trash. A
	// tenant with the trash turned off always deletes hard.
	Hard bool `json:"hard,omitempty" yaml:"hard,omitempty"`
	// Cascade deletes the task's subtasks with it.
	Cascade bool `json:"cascade,omitempty" yaml:"cascade,omitempty"`
}

// CopyTaskInput says where a copy of a task goes and what travels with it.
// An empty ProjectRef copies into the source's own project and an empty Title
// keeps the source's.
type CopyTaskInput struct {
	// ProjectRef is the key or identifier of the project to copy into; empty
	// copies into the source's own project.
	ProjectRef string `json:"project_ref,omitempty" yaml:"project_ref,omitempty"`
	// Title is the copy's title; empty keeps the source's.
	Title string `json:"title,omitempty" yaml:"title,omitempty"`
	// KeepStatus starts the copy in the source's state when the target
	// workflow has one of that name, rather than in its initial state.
	KeepStatus bool `json:"keep_status,omitempty" yaml:"keep_status,omitempty"`
	// WithComments copies the source's comments onto the copy.
	WithComments bool `json:"with_comments,omitempty" yaml:"with_comments,omitempty"`
	// WithSubtasks copies the source's subtasks too, as subtasks of the copy.
	WithSubtasks bool `json:"with_subtasks,omitempty" yaml:"with_subtasks,omitempty"`
	// WithDependencies copies dependency edges, pointing an edge at the copy of
	// its far end when that was copied too.
	WithDependencies bool `json:"with_dependencies,omitempty" yaml:"with_dependencies,omitempty"`
	// Link records the source on each copy as its CopiedFromID.
	Link bool `json:"link,omitempty" yaml:"link,omitempty"`
}

// Validate checks the input.
func (in CopyTaskInput) Validate() error {
	if len(in.Title) > MaxTitleLength {
		return Invalid("task title must be at most %d characters", MaxTitleLength).WithDetail(DetailField, "title")
	}
	return nil
}

// UpdateTaskInput changes a task.
type UpdateTaskInput struct {
	// Title replaces the title, at most 500 characters; omitted leaves it.
	Title *string `json:"title,omitempty" yaml:"title,omitempty"`
	// Body replaces the Markdown description; omitted leaves it.
	Body *string `json:"body,omitempty" yaml:"body,omitempty"`
	// Priority sets the priority, 1 (highest) to 5 (lowest); omitted leaves it.
	Priority *Priority `json:"priority,omitempty" yaml:"priority,omitempty"`
	// AssigneeActorID assigns an actor by identifier or handle; empty
	// unassigns and omitted leaves it.
	AssigneeActorID *string `json:"assignee_actor_id,omitempty" yaml:"assignee_actor_id,omitempty"`
	// ParentRef moves the task under another task, refusing a cycle; empty
	// detaches it and omitted leaves it.
	ParentRef *string `json:"parent_ref,omitempty" yaml:"parent_ref,omitempty"`
	// DueAt sets the due time, an RFC 3339 timestamp; null clears it and
	// omitted leaves it.
	DueAt **time.Time `json:"due_at,omitempty" yaml:"due_at,omitempty"`
	// CustomFields merges values over the stored ones by field key; a null
	// value clears that field.
	CustomFields map[string]any `json:"custom_fields,omitempty" yaml:"custom_fields,omitempty"`
	// Tags replaces the task's whole tag set; an empty list removes every tag
	// and omitted leaves them.
	Tags *[]string `json:"tags,omitempty" yaml:"tags,omitempty"`
	// Recurrence replaces the repeat rule; a pointer to the empty string
	// removes it and nil leaves it alone.
	Recurrence *string `json:"recurrence,omitempty" yaml:"recurrence,omitempty"`
	// Lease sets how long a claim on the task holds; a pointer to zero clears
	// it back to the project's.
	Lease *Duration `json:"lease,omitempty" yaml:"lease,omitempty"`

	// Version is the task version the caller last read. When set and stale the
	// update is refused as a conflict; 0 skips the check.
	Version int `json:"version,omitempty" yaml:"version,omitempty"`
}

// UnmarshalJSON decodes the update, keeping "due_at": null as a clear. The
// standard decoder turns null into a nil outer pointer, which reads as omitted.
func (in *UpdateTaskInput) UnmarshalJSON(b []byte) error {
	type plain UpdateTaskInput
	var decoded plain
	if err := json.Unmarshal(b, &decoded); err != nil {
		return err
	}
	if decoded.DueAt == nil {
		var present struct {
			DueAt json.RawMessage `json:"due_at"`
		}
		if err := json.Unmarshal(b, &present); err != nil {
			return err
		}
		if bytes.Equal(bytes.TrimSpace(present.DueAt), []byte("null")) {
			decoded.DueAt = new(*time.Time)
		}
	}
	*in = UpdateTaskInput(decoded)
	return nil
}

// TransitionInput moves a task to a new status.
type TransitionInput struct {
	// To is the target status, which the workflow must allow from the current
	// one. Required.
	To string `json:"to" yaml:"to"`
	// Comment is added to the task as a comment; required when the transition
	// demands one.
	Comment string `json:"comment,omitempty" yaml:"comment,omitempty"`
	// LeaseToken is the claim's token, required while the task is claimed.
	LeaseToken string `json:"lease_token,omitempty" yaml:"lease_token,omitempty"`
	// CustomFields merges values over the stored ones by field key; a null
	// value clears that field.
	CustomFields map[string]any `json:"custom_fields,omitempty" yaml:"custom_fields,omitempty"`
	// Version is the task version the caller last read. When set and stale the
	// move is refused as a conflict; 0 skips the check.
	Version int `json:"version,omitempty" yaml:"version,omitempty"`
}

// Validate checks the input.
func (in TransitionInput) Validate() error {
	if in.To == "" {
		return Invalid("target status is required")
	}
	return nil
}

// ArtifactInput records structured output on a task. An artifact is the
// holder's work product, so a task carrying a live lease requires its token.
type ArtifactInput struct {
	// Kind classifies the output. Required.
	Kind ArtifactKind `json:"kind" yaml:"kind"`
	// Name labels the artifact among the task's others.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// Payload is structured output as a JSON object.
	Payload map[string]any `json:"payload,omitempty" yaml:"payload,omitempty"`
	// ContentType is the media type of Blob.
	ContentType string `json:"content_type,omitempty" yaml:"content_type,omitempty"`
	// Blob is inline content, base64 in JSON, held to the tenant's artifact
	// limits on size and media type; larger content belongs elsewhere,
	// referenced from Payload.
	Blob []byte `json:"blob,omitempty" yaml:"blob,omitempty"`
	// LeaseToken is the claim's token, required while the task is claimed.
	LeaseToken string `json:"lease_token,omitempty" yaml:"lease_token,omitempty"`
}

// Validate checks the input.
func (in ArtifactInput) Validate() error {
	if in.Kind == "" {
		return Invalid("artifact kind is required")
	}
	return nil
}

// SnoozeInput says until when a task sleeps.
type SnoozeInput struct {
	// Until is when the task wakes, an RFC 3339 timestamp in the future,
	// truncated to the second. Required.
	Until time.Time `json:"until" yaml:"until"`
}

// ClaimInput claims one task.
type ClaimInput struct {
	// TTL is how long the lease holds, a duration such as "30m". Omitted takes
	// the task, project, workflow or tenant default; a value above the maximum
	// is refused.
	TTL Duration `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	// ActorID claims on behalf of another actor, by identifier or handle;
	// empty claims for the caller.
	ActorID string `json:"actor_id,omitempty" yaml:"actor_id,omitempty"`
}

// ClaimNextInput claims the next eligible task.
type ClaimNextInput struct {
	// ProjectRefs restricts the queue to these projects, by key or identifier;
	// empty means every project the caller may claim in.
	ProjectRefs []string `json:"project_refs,omitempty" yaml:"project_refs,omitempty"`
	// Tags restricts the queue to tasks carrying any of these tags.
	Tags []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	// Statuses restricts the queue to tasks in any of these statuses; empty
	// means every status that is not terminal.
	Statuses []string `json:"statuses,omitempty" yaml:"statuses,omitempty"`
	// TTL is how long the lease holds, a duration such as "30m". Omitted takes
	// the configured default; a value above the maximum is refused.
	TTL Duration `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	// ActorID claims on behalf of another actor, by identifier or handle;
	// empty claims for the caller.
	ActorID string `json:"actor_id,omitempty" yaml:"actor_id,omitempty"`
}

// ForceReclaimInput carries the administrator's reason for taking a lease.
// The reason is optional and is recorded on the audit entry, because the
// question asked of a forced reclaim afterwards is never whether it happened,
// which the entry says, but why somebody else's work was taken.
type ForceReclaimInput struct {
	// Reason says why the lease was taken, at most 500 characters.
	Reason string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// Validate checks the input.
func (in ForceReclaimInput) Validate() error {
	if len(in.Reason) > MaxTitleLength {
		return Invalid("reclaim reason must be at most %d characters", MaxTitleLength)
	}
	return nil
}

// ReleaseInput gives up a lease.
type ReleaseInput struct {
	// Status moves the task to this status as it is released; empty leaves
	// the status as it is.
	Status string `json:"status,omitempty" yaml:"status,omitempty"`
	// Result is recorded as a "result" artifact named "result".
	Result map[string]any `json:"result,omitempty" yaml:"result,omitempty"`
	// Comment is added to the task as a comment.
	Comment string `json:"comment,omitempty" yaml:"comment,omitempty"`
}

// CreateUserInput creates a user.
type CreateUserInput struct {
	// Email is the user's sign-in address, lowercased. Required.
	Email string `json:"email" yaml:"email"`
	// Password is at least 12 characters; empty creates a user with no
	// password.
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
	// DisplayName is the name shown for the user.
	DisplayName string `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	// Handle is the user's unique short name, lowercased; empty uses the
	// part of the email before the "@".
	Handle string `json:"handle,omitempty" yaml:"handle,omitempty"`
	// Role is the user's role in this tenant; empty means "viewer".
	Role Role `json:"role,omitempty" yaml:"role,omitempty"`
	// RequirePasswordChange makes the first password sign-in choose a new
	// password before it is let in. It needs a password.
	RequirePasswordChange bool `json:"require_password_change,omitempty" yaml:"require_password_change,omitempty"`
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
	if in.RequirePasswordChange && in.Password == "" {
		return Invalid("a password change can only be required of an account with a password; set one, or invite the address instead").
			WithDetail(DetailField, "require_password_change")
	}
	return nil
}

// MinPasswordLength is the shortest accepted password.
const MinPasswordLength = 12

// UpdateUserInput changes a user.
type UpdateUserInput struct {
	// DisplayName replaces the display name; omitted leaves it.
	DisplayName *string `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	// Password sets a new password; omitted leaves it.
	Password *string `json:"password,omitempty" yaml:"password,omitempty"`
	// Role changes the user's role in this tenant; omitted leaves it.
	Role *Role `json:"role,omitempty" yaml:"role,omitempty"`
	// Disabled true disables the user, ending their sessions and revoking
	// their tokens at once; false enables them; omitted leaves it.
	Disabled *bool `json:"disabled,omitempty" yaml:"disabled,omitempty"`
	// RequirePasswordChange true makes the next password sign-in choose a new
	// password first; false lifts that; omitted leaves it.
	RequirePasswordChange *bool `json:"require_password_change,omitempty" yaml:"require_password_change,omitempty"`
}

// CreateTokenInput mints an API token.
type CreateTokenInput struct {
	// Name labels the token, unique among the tenant's live tokens. Required.
	Name string `json:"name" yaml:"name"`
	// ActorID is the actor the token acts as, by identifier or handle; empty
	// means the caller.
	ActorID string `json:"actor_id,omitempty" yaml:"actor_id,omitempty"`
	// Scopes are the permissions the token carries, each one the creator must
	// hold. At least one is required.
	Scopes []Scope `json:"scopes" yaml:"scopes"`
	// ProjectID confines the token to one project, by key or identifier.
	ProjectID string `json:"project_id,omitempty" yaml:"project_id,omitempty"`
	// ExpiresAt is when the token stops working, an RFC 3339 timestamp; omitted
	// means it does not expire.
	ExpiresAt *time.Time `json:"expires_at,omitempty" yaml:"expires_at,omitempty"`
}

// Validate checks the input.
//
// Each refusal names the field it is about, so a surface rendering a form can
// put the message beside the control that caused it rather than on a page of
// its own.
func (in CreateTokenInput) Validate() error {
	if in.Name == "" {
		return Invalid("token name is required").WithDetail(DetailField, "name")
	}
	if len(in.Scopes) == 0 {
		return Invalid("a token requires at least one scope").WithDetail(DetailField, "scopes")
	}
	return nil
}

// MaxTokenRotateGrace is the longest a rotated token may keep working beside
// its replacement. A longer window would keep a leaked value alive.
const MaxTokenRotateGrace = 24 * time.Hour

// RotateTokenInput replaces a live token with a new one of the same name,
// scopes and project.
type RotateTokenInput struct {
	// Grace is how long the old token keeps working beside the new one, from
	// 0 (it stops at once, the default) up to 24h.
	Grace Duration `json:"grace,omitempty" yaml:"grace,omitempty"`
	// ActorID, when set, is the actor the token must belong to, by identifier
	// or handle; a token of anybody else is reported missing.
	ActorID string `json:"actor_id,omitempty" yaml:"actor_id,omitempty"`
}

// Validate checks the input.
func (in RotateTokenInput) Validate() error {
	if in.Grace < 0 || in.Grace.D() > MaxTokenRotateGrace {
		return Invalid("a rotated token keeps working for between 0 and 24h, not %s", in.Grace).
			WithDetail(DetailField, "grace")
	}
	return nil
}

// WebhookInput registers or updates a delivery endpoint.
type WebhookInput struct {
	// ID names the endpoint to update; empty registers a new one.
	ID string `json:"id,omitempty" yaml:"id,omitempty"`
	// URL receives the deliveries. It must use https unless it is a loopback
	// address. Required.
	URL string `json:"url" yaml:"url"`
	// Secret signs each delivery. Empty keeps an existing endpoint's secret,
	// or generates one that the response returns once.
	Secret string `json:"secret,omitempty" yaml:"secret,omitempty"`
	// EventTypes filters the events delivered; a pattern may end in "*".
	// Empty delivers every event.
	EventTypes []string `json:"event_types,omitempty" yaml:"event_types,omitempty"`
	// Active turns delivery on; an inactive endpoint receives nothing.
	Active bool `json:"active" yaml:"active"`
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
	// ProjectRefs restricts the snapshot to these projects, by key or
	// identifier; empty exports every project.
	ProjectRefs []string `json:"project_refs,omitempty" yaml:"project_refs,omitempty"`
	// IncludeArtifacts adds the tasks' artifacts.
	IncludeArtifacts bool `json:"include_artifacts,omitempty" yaml:"include_artifacts,omitempty"`
	// IncludeComments adds the tasks' comments.
	IncludeComments bool `json:"include_comments,omitempty" yaml:"include_comments,omitempty"`
}

// ImportInput controls an import.
type ImportInput struct {
	// Mode is required: "merge" adds and updates, while "replace" also removes
	// the tasks and field definitions of an imported project that the
	// snapshot does not carry.
	Mode ImportMode `json:"mode" yaml:"mode"`
	// DryRun reports what the import would do and writes nothing.
	DryRun bool `json:"dry_run,omitempty" yaml:"dry_run,omitempty"`
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
	// ID names the source to update; empty registers a new one.
	ID string `json:"id,omitempty" yaml:"id,omitempty"`
	// System is the external system: "generic", "jira" or "openproject".
	System string `json:"system" yaml:"system"`
	// Name labels the source. Required.
	Name string `json:"name" yaml:"name"`
	// Config holds the source's non-secret settings, stored and preferred over
	// TIX_SYNC_<SOURCE>_* at run time: "file" and "page_size" for generic,
	// "url", "project", "query" and "page_size" for jira and openproject. A
	// credential key or any other key is refused. Updating replaces them whole.
	// On tix serve "file" must lie in the tenant's directory under the
	// operator's sync.file_root, and the variables apply only to sources the
	// operator binds in sync.env_sources.
	Config map[string]any `json:"config,omitempty" yaml:"config,omitempty"`
	// MappingPath is a mapping file on the server, stored and preferred over
	// TIX_SYNC_<SOURCE>_MAPPING at run time. On tix serve it must lie in the
	// tenant's directory under sync.file_root; without one it is refused.
	MappingPath string `json:"mapping_path,omitempty" yaml:"mapping_path,omitempty"`
	// Mapping is the mapping document itself, YAML, stored on the source. It
	// wins over MappingPath. Left empty, the stored document is kept.
	Mapping string `json:"mapping,omitempty" yaml:"mapping,omitempty"`
	// ClearMapping removes the stored mapping document.
	ClearMapping bool `json:"clear_mapping,omitempty" yaml:"clear_mapping,omitempty"`
	// Token is a bearer credential to store on the server, sealed. Write-only.
	Token string `json:"token,omitempty" yaml:"token,omitempty"`
	// User is a basic auth user to store on the server, sealed. Write-only.
	User string `json:"user,omitempty" yaml:"user,omitempty"`
	// Password is a basic auth password or API key to store on the server,
	// sealed. Write-only. Any of Token, User or Password replaces the stored
	// set whole; with none of them the stored set is kept.
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
	// ClearCredentials removes the stored credentials.
	ClearCredentials bool `json:"clear_credentials,omitempty" yaml:"clear_credentials,omitempty"`
	// KeepSettings leaves the stored Config and MappingPath as they are, for an
	// update that only sets the mapping document or the credentials.
	KeepSettings bool `json:"keep_settings,omitempty" yaml:"keep_settings,omitempty"`
	// Check validates the source as it would be saved, describes its mapping
	// and what a run would use, and writes nothing.
	Check bool `json:"check,omitempty" yaml:"check,omitempty"`
}

// HasCredentials reports whether the input carries any credential to store.
func (in SyncSourceInput) HasCredentials() bool {
	return in.Token != "" || in.User != "" || in.Password != ""
}

// Sync source settings a source may store in its Config.
const (
	SyncSettingURL      = "url"
	SyncSettingProject  = "project"
	SyncSettingQuery    = "query"
	SyncSettingFile     = "file"
	SyncSettingPageSize = "page_size"
)

// SyncSettingKeys returns the Config keys a system accepts, in display order.
func SyncSettingKeys(system string) []string {
	switch system {
	case SystemGeneric:
		return []string{SyncSettingFile, SyncSettingPageSize}
	case SystemJira, SystemOpenProject:
		return []string{SyncSettingURL, SyncSettingProject, SyncSettingQuery, SyncSettingPageSize}
	default:
		return nil
	}
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
	// SourceID is the identifier of the registered source to run. Required.
	SourceID string `json:"source_id" yaml:"source_id"`
	// DryRun reports what the run would import and writes nothing.
	DryRun bool `json:"dry_run,omitempty" yaml:"dry_run,omitempty"`
	// Full reads the source from the beginning instead of resuming from the
	// stored cursor.
	Full bool `json:"full,omitempty" yaml:"full,omitempty"`
}

// PruneInput removes records past their retention window.
type PruneInput struct {
	// DryRun reports what would be removed and removes nothing.
	DryRun bool `json:"dry_run,omitempty" yaml:"dry_run,omitempty"`
	// Limit caps how many rows of each kind one run removes; 0 means 500.
	Limit int `json:"limit,omitempty" yaml:"limit,omitempty"`
}
