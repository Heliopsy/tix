// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"context"
	"io"
	"slices"
	"time"
)

// Service is the complete tix product surface.
//
// A method declared (*T, error) returns a non-nil pointer whenever it returns
// a nil error. There is no third answer: an absent record is an error, and a
// read that succeeded has something to hand back. Callers therefore branch on
// the error alone, and an implementation that returned a nil pair would panic
// them one dereference later rather than report its own fault.
type Service interface {
	// WhoAmI returns the actor the request authenticated as, with its tenant,
	// role and scopes.
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
	BundleService
	ConnectionService
	StatsService
	StatusService
	ViewService
	SavedFilterService
	CalendarService
	InviteService
	InboxService
	InboxChannelService
	TemplateService
	TimeService

	// Close releases the service's resources, such as the underlying store.
	Close() error
}

// TenantService covers tenants, their domains, and membership.
type TenantService interface {
	// CreateTenant creates a tenant with the given key and name, seeds it with
	// the builtin workflow, and makes the admins it names admins of the new
	// tenant. A new address becomes a user whose password, when left out, is
	// generated and returned once in initial_credentials. An address that
	// already has an account keeps its own password and second factor, and
	// supplying a password for it is refused. Only a global admin may call
	// it. A key already in use is a conflict, and nothing is created.
	CreateTenant(ctx context.Context, in CreateTenantInput) (*CreatedTenant, error)
	// GetTenant returns the caller's own tenant by identifier or key; an empty
	// reference means the caller's tenant. Any other tenant reads as not_found,
	// except to a global admin, who may read any.
	GetTenant(ctx context.Context, ref string) (*Tenant, error)
	// ListTenants lists the tenants the caller can see: every live tenant for a
	// global admin, and only its own for anybody else, since an actor is bound
	// to one tenant.
	ListTenants(ctx context.Context, page Page) ([]Tenant, string, error)
	// UpdateTenant changes the caller's tenant, or for a global admin any
	// tenant: its name, theme, lease bounds,
	// second-factor requirements and trash setting. Fields left out are
	// unchanged. A lease default longer than the maximum, an unknown theme, or a
	// second-factor requirement with no sealing key configured is refused.
	UpdateTenant(ctx context.Context, ref string, in UpdateTenantInput) (*Tenant, error)
	// DeleteTenant soft deletes the caller's tenant, or for a global admin any
	// tenant, so its data stops being reachable and its domains stop
	// resolving. No row is destroyed.
	DeleteTenant(ctx context.Context, ref string) error

	// ListTenantUsers lists the users of any tenant, with each one's role
	// there, keyset paginated. People removed from the tenant are left out
	// unless include_removed asks for them, marked removed and with no role.
	// Only a global admin may call it; for anybody else it is forbidden
	// whatever the tenant.
	ListTenantUsers(ctx context.Context, tenant string, f UserFilter) ([]TenantUser, string, error)
	// CreateTenantUser creates a user, its actor and its membership in any
	// tenant, as CreateUser does in the caller's own. Global admin only.
	CreateTenantUser(ctx context.Context, tenant string, in CreateUserInput) (*User, error)
	// GetAccountLog returns the caller's own account log, oldest first unless
	// the page asks for desc: every
	// change to their password, second factor, disabled state, global admin
	// standing and memberships, in every tenant they belong to, with who made
	// it and from which tenant. A token needs the account:read scope, which
	// "*" does not carry.
	GetAccountLog(ctx context.Context, page Page) ([]AccountLogEntry, string, error)
	// GetAccountLogFor returns the log of the account with this email, for a
	// global admin. Anybody else is refused.
	GetAccountLogFor(ctx context.Context, email string, page Page) ([]AccountLogEntry, string, error)
	// AddTenantMember adds an existing account, found by email, to any tenant
	// with a role: a new actor there and its membership, with no acceptance
	// asked of the person. An account that already has an actor there without
	// a membership gets the membership back. Global admin only; a tenant admin
	// invites instead, and the person accepts.
	AddTenantMember(ctx context.Context, tenant string, in AddTenantMemberInput) (*User, error)
	// UpdateTenantUser changes a user of any tenant: display name, password,
	// role or disabled state, as UpdateUser does. Global admin only.
	UpdateTenantUser(ctx context.Context, tenant, id string, in UpdateUserInput) (*User, error)
	// DeleteTenantUser deletes a user of any tenant and revokes its membership
	// there, as DeleteUser does. Global admin only.
	DeleteTenantUser(ctx context.Context, tenant, id string) error

	// AddDomain maps a hostname to the current tenant, so requests to that host
	// resolve to it. A certificate mode of file needs both a certificate and a
	// key path.
	AddDomain(ctx context.Context, in AddDomainInput) (*Domain, error)
	// ListDomains lists the hostnames mapped to the current tenant.
	ListDomains(ctx context.Context) ([]Domain, error)
	// RemoveDomain detaches a hostname from the current tenant. An unmapped
	// hostname is not_found.
	RemoveDomain(ctx context.Context, hostname string) error
	// ResolveDomain returns the tenant a hostname is mapped to. It runs before
	// authentication and reads across tenants.
	ResolveDomain(ctx context.Context, hostname string) (*Tenant, error)
	// UpdateDomain changes a tenant domain, whichever tenant it belongs to.
	// Marking it shared lets its host also serve the tenant a session, token or
	// the X-Tix-Tenant header names, and its own tenant otherwise. Global admin
	// only. An unmapped hostname is not_found.
	UpdateDomain(ctx context.Context, hostname string, in UpdateDomainInput) (*Domain, error)
	// ListTenantDomains lists the hostnames mapped to any tenant, each with
	// whether it is shared. Global admin only.
	ListTenantDomains(ctx context.Context, tenant string) ([]Domain, error)

	// AddMember grants an existing actor a role (viewer, member or admin) in the
	// current tenant. An actor who is already a member is a conflict.
	AddMember(ctx context.Context, actorID string, role Role) (*Membership, error)
	// ListMembers lists every membership of the current tenant with its role.
	ListMembers(ctx context.Context) ([]Membership, error)
	// RemoveMember revokes an actor's membership of the current tenant. An actor
	// who is not a member is not_found.
	RemoveMember(ctx context.Context, actorID string) error
}

// ProjectService covers projects and their custom field definitions.
type ProjectService interface {
	// CreateProject creates a project with a unique key, on the named workflow or
	// the builtin one. A key in use, or kept as an old key of another project, is
	// a conflict.
	CreateProject(ctx context.Context, in CreateProjectInput) (*Project, error)
	// GetProject returns a project by identifier, key, or one of its old keys.
	GetProject(ctx context.Context, ref string) (*Project, error)
	// GetProjectIcon returns a project's uploaded icon image. A project without
	// an uploaded image is not_found.
	GetProjectIcon(ctx context.Context, ref string) (*ProjectIconImage, error)
	// ListProjects lists projects matching the filter, keyset paginated, with the
	// cursor for the next page. A token pinned to one project sees only that
	// project.
	ListProjects(ctx context.Context, f ProjectFilter) ([]Project, string, error)
	// UpdateProject changes a project's name, key, description, colour, icon,
	// workflow or lease bounds; fields left out are unchanged. A new key keeps
	// the old one as an alias, and a workflow that lacks a state some task is in
	// is a conflict.
	UpdateProject(ctx context.Context, ref string, in UpdateProjectInput) (*Project, error)
	// ArchiveProject hides a project from default listings without destroying its
	// tasks or history. Archiving an archived project is a conflict.
	ArchiveProject(ctx context.Context, ref string) error
	// DeleteProject removes a project permanently, moving or deleting its
	// tasks as the input says; a project holding tasks needs a choice.
	DeleteProject(ctx context.Context, ref string, in DeleteProjectInput) (*ProjectDeletion, error)
	// PlanProjectDeletion reports what DeleteProject would do, writing nothing.
	PlanProjectDeletion(ctx context.Context, ref string, in DeleteProjectInput) (*ProjectDeletion, error)

	// ListStarredProjects lists the projects the caller starred, in the order
	// their lanes are drawn. A star is the caller's own: nobody else's is ever
	// listed, and a project the caller can no longer read is left out.
	ListStarredProjects(ctx context.Context) ([]Project, error)
	// StarProject stars a project for the caller alone, which lists it first on
	// the projects screen and in the sidebar and leads its lane. Starring a
	// starred project writes nothing.
	StarProject(ctx context.Context, ref string) error
	// UnstarProject clears the caller's own star on a project. A project that is
	// not starred is left as it is, with nothing written.
	UnstarProject(ctx context.Context, ref string) error

	// PutFieldDef creates or replaces a custom field definition on a project,
	// keyed by its field key. A default value must fit the field's type.
	PutFieldDef(ctx context.Context, projectRef string, in FieldDefInput) (*FieldDef, error)
	// ListFieldDefs lists a project's custom field definitions in display order.
	ListFieldDefs(ctx context.Context, projectRef string) ([]FieldDef, error)
	// DeleteFieldDef removes a custom field definition from a project. An unknown
	// key is not_found.
	DeleteFieldDef(ctx context.Context, projectRef, key string) error
}

// WorkflowService covers state machine definitions.
type WorkflowService interface {
	// PutWorkflow creates or replaces a workflow by key: its states, initial
	// state and allowed transitions. Removing a state that tasks are in is a
	// conflict unless the input migrates them to another state.
	PutWorkflow(ctx context.Context, in WorkflowInput) (*Workflow, error)
	// GetWorkflow returns a workflow by key or identifier.
	GetWorkflow(ctx context.Context, key string) (*Workflow, error)
	// ListWorkflows lists every workflow in the current tenant.
	ListWorkflows(ctx context.Context) ([]Workflow, error)
	// DeleteWorkflow deletes a workflow by key or identifier. The builtin
	// workflow, and a workflow any project still uses, cannot be deleted and
	// answer conflict.
	DeleteWorkflow(ctx context.Context, key string) error
}

// TaskService covers tasks and everything attached to them.
type TaskService interface {
	// CreateTask creates a task in a project and returns it. Only the title is
	// required: status defaults to the workflow's initial state, and the project
	// may be left out when the tenant has exactly one.
	CreateTask(ctx context.Context, in CreateTaskInput) (*Task, error)
	// GetTask returns one live task by identifier or reference such as infra-42.
	// A deleted task is not_found.
	GetTask(ctx context.Context, ref TaskRef) (*Task, error)
	// ListTasks lists tasks matching the filter, one keyset-paginated page at a
	// time, with the cursor for the next page. Snoozed tasks are left out unless
	// the filter asks for them.
	ListTasks(ctx context.Context, f TaskFilter) (TaskPage, error)
	// UpdateTask changes only the task attributes the input supplies and returns
	// the task. With a version set, a task that has changed since is a conflict
	// carrying the current version.
	UpdateTask(ctx context.Context, ref TaskRef, in UpdateTaskInput) (*Task, error)
	// TransitionTask moves a task to another status along a transition its
	// workflow allows. A move the workflow does not allow is precondition_failed,
	// a transition that requires a comment needs one, and a stale version is a
	// conflict.
	TransitionTask(ctx context.Context, ref TaskRef, in TransitionInput) (*Task, error)
	// TransitionRoute walks a task through a route of statuses, one ordinary
	// transition per hop, each with its own audit entry and event. A route that
	// stops partway reports where and why; only a refused first hop is an error.
	TransitionRoute(ctx context.Context, ref TaskRef, in RouteInput) (*RouteResult, error)
	// DeleteTask moves a task to the trash, or deletes it permanently when hard
	// is set or the tenant keeps no trash. A task with children is refused unless
	// cascade is set, which takes the whole subtree.
	DeleteTask(ctx context.Context, ref TaskRef, in DeleteTaskInput) error
	// RestoreTask brings a task back out of the trash and returns it. It is
	// refused with precondition_failed when the tenant keeps no trash.
	RestoreTask(ctx context.Context, ref TaskRef) (*Task, error)
	// PurgeTask permanently deletes a task already in the trash, with its
	// trashed subtasks, the way the trash rule does when it expires them. A
	// live task is refused with precondition_failed and reason not_trashed,
	// and a tenant whose trash is off refuses it with reason trash_off.
	PurgeTask(ctx context.Context, ref TaskRef) error
	// TrashStatus reports whether the tenant keeps deleted tasks in a trash, and
	// how many it holds.
	TrashStatus(ctx context.Context) (*TrashStatus, error)
	// TaskTree returns a task followed by its live descendants in depth-first
	// pre-order, down to the given depth. A subtree too large to walk is
	// precondition_failed.
	TaskTree(ctx context.Context, ref TaskRef, depth int) ([]Task, error)
	// CopyTask creates a copy of a task, in its own project or another one,
	// and returns the copy. See CopyTaskInput for what travels.
	CopyTask(ctx context.Context, ref TaskRef, in CopyTaskInput) (*Task, error)
	// BulkTasks applies one action to many tasks, each in its own transaction,
	// and reports an outcome per task rather than failing on the first refusal.
	BulkTasks(ctx context.Context, in BulkTaskInput) (*BulkResult, error)

	// AddDependency records that a task waits for another task. A self edge or
	// one that would close a cycle is invalid, and the caller needs write access
	// to both tasks.
	AddDependency(ctx context.Context, ref, dependsOn TaskRef) error
	// RemoveDependency removes the edge saying a task waits for another.
	RemoveDependency(ctx context.Context, ref, dependsOn TaskRef) error
	// ListDependencies lists the tasks a task waits for, leaving out edges to
	// tasks the caller cannot read.
	ListDependencies(ctx context.Context, ref TaskRef) ([]Dependency, error)

	// AddTag puts a tag on a task, creating the tag in the tenant when it is new.
	// Tagging a task that already carries the tag changes nothing.
	AddTag(ctx context.Context, ref TaskRef, tag string) error
	// RemoveTag takes a tag off a task. A tag the tenant does not have is
	// not_found.
	RemoveTag(ctx context.Context, ref TaskRef, tag string) error
	// ListTags lists every tag defined in the tenant.
	ListTags(ctx context.Context) ([]Tag, error)

	// StarTask stars a task for the caller alone. Starring a starred task writes
	// nothing.
	StarTask(ctx context.Context, ref TaskRef) error
	// UnstarTask clears the caller's own star on a task. A task that is not
	// starred is left as it is, with nothing written.
	UnstarTask(ctx context.Context, ref TaskRef) error

	// CheckTaskItem ticks or clears one checklist item of a task's description.
	CheckTaskItem(ctx context.Context, ref TaskRef, in CheckItemInput) (*Task, error)
	// SnoozeTask hides a task from default listings, for every reader, until a
	// future time. Snoozing a snoozed task moves its wake time, and a snoozed
	// task cannot be claimed.
	SnoozeTask(ctx context.Context, ref TaskRef, in SnoozeInput) (*Task, error)
	// UnsnoozeTask clears a task's snooze and returns the task. A task that is
	// not snoozed is returned as it is, with nothing written.
	UnsnoozeTask(ctx context.Context, ref TaskRef) (*Task, error)
	// WakeSnoozed clears up to limit snoozes whose wake time has passed, emitting
	// a task.woke event for each, and reports how many it cleared.
	WakeSnoozed(ctx context.Context, limit int) (int, error)

	// AddComment adds a comment to a task and returns it, notifying the people it
	// mentions, the assignee and anyone who starred the task.
	AddComment(ctx context.Context, ref TaskRef, body string) (*Comment, error)
	// ListComments lists a task's comments that are not deleted, oldest first.
	ListComments(ctx context.Context, ref TaskRef) ([]Comment, error)
	// EditComment replaces a comment's body and marks it edited. A deleted
	// comment is not_found.
	EditComment(ctx context.Context, id, body string) (*Comment, error)
	// DeleteComment soft deletes a comment, so it no longer lists.
	DeleteComment(ctx context.Context, id string) error

	// PutArtifact attaches a new piece of structured output, such as a result or
	// a log, to a task. While the task is claimed it needs the current lease
	// token, and a stale one is lease_expired.
	PutArtifact(ctx context.Context, ref TaskRef, in ArtifactInput) (*Artifact, error)
	// ListArtifacts lists a task's artifacts, oldest first.
	ListArtifacts(ctx context.Context, ref TaskRef) ([]Artifact, error)
}

// ClaimService covers the agent work queue.
type ClaimService interface {
	// ClaimTask takes a lease on one task for the caller, or for the actor named,
	// and returns the lease token to renew and release it with. A task another
	// worker holds, or one already finished, is a conflict; a snoozed task is
	// precondition_failed.
	ClaimTask(ctx context.Context, ref TaskRef, in ClaimInput) (*Claim, error)
	// TaskLease reports the lease a claim on the task naming no ttl would be
	// granted now, and which level decides it.
	TaskLease(ctx context.Context, ref TaskRef) (*LeaseGrant, error)
	// ClaimNext claims the highest-priority unblocked, unclaimed task matching
	// the filter and returns it with its lease token. An empty queue answers
	// no_task_available.
	ClaimNext(ctx context.Context, in ClaimNextInput) (*Claim, error)
	// RenewLease extends the lease the caller holds on a task under its token, by
	// the ttl given or the task's default lease. A token that no longer holds the
	// lease is lease_expired, and the worker should claim again.
	RenewLease(ctx context.Context, ref TaskRef, token string, ttl Duration) (*Claim, error)
	// ReleaseLease gives up the lease held under the caller's token. It can
	// also move the task to a final status and attach a result artifact and a
	// closing comment. A token that no longer holds the lease is lease_expired.
	ReleaseLease(ctx context.Context, ref TaskRef, token string, in ReleaseInput) error
	// ForceReclaim ends the lease another actor holds on a task, for an
	// administrator whose worker has died. The task keeps its status.
	ForceReclaim(ctx context.Context, ref TaskRef, in ForceReclaimInput) (*Task, error)
	// SweepLeases clears up to limit expired leases and reports how many it
	// cleared. Each is recorded, and the status reverts where the workflow asks.
	SweepLeases(ctx context.Context, limit int) (int, error)
}

// HistoryService covers the audit log, the event stream, and retention.
type HistoryService interface {
	// ListAudit lists audit entries matching the filter, one keyset-paginated
	// page at a time, with the cursor for the next page.
	ListAudit(ctx context.Context, f AuditFilter) ([]AuditEntry, string, error)
	// Subscribe streams committed events matching the filter, from the given
	// sequence number or from now. A sequence number already pruned is invalid.
	Subscribe(ctx context.Context, f EventFilter) (<-chan Event, error)
	// PollEvents returns the committed events after since_seq that match the
	// types and project_ids filters, in sequence order and at most limit
	// (default 200, at most 1000), with next_seq to pass back as since_seq.
	// next_seq is the last sequence number read, which is past the last event
	// returned when later ones were filtered out. since_seq is exclusive; absent or 0 starts at the newest event. wait, up
	// to 25s, holds an empty answer until an event commits or the wait runs out.
	// A cursor older than the oldest retained event is invalid.
	PollEvents(ctx context.Context, in EventPollInput) (*EventPage, error)
	// Prune deletes events, audit entries and webhook deliveries older than the
	// retention policy allows. A dry run reports the counts and writes nothing,
	// not even an audit entry.
	Prune(ctx context.Context, in PruneInput) (*PruneResult, error)
	// GetRetention returns the tenant's effective retention policy.
	GetRetention(ctx context.Context) (*RetentionPolicy, error)
	// PutRetention changes the tenant's retention windows and returns the policy
	// now in force. Windows left unset keep their current value.
	PutRetention(ctx context.Context, p RetentionPolicy) (*RetentionPolicy, error)

	// GetHousekeeping returns the housekeeping policy, for one project when
	// project is set, and how many tasks it would act on now.
	GetHousekeeping(ctx context.Context, project string) (*HousekeepingView, error)
	// PutHousekeeping changes the tenant default or one project's override.
	PutHousekeeping(ctx context.Context, in HousekeepingInput) (*HousekeepingView, error)
	// RunHousekeeping applies the policy once, in bounded batches, or
	// previews it.
	RunHousekeeping(ctx context.Context, in HousekeepingRunInput) (*HousekeepingResult, error)
}

// AuthService covers users, sessions and API tokens.
type AuthService interface {
	// GetActor returns the actor an identifier of this tenant names, so a
	// record can show who acted rather than a raw identifier. It returns the
	// naming fields only, never authority.
	GetActor(ctx context.Context, id string) (*Actor, error)

	// ListActors lists this tenant's actors, agents included, keyset
	// paginated. A caller can offer them instead of asking for an identifier
	// typed from memory. Like GetActor it returns the naming fields only,
	// never authority.
	ListActors(ctx context.Context, page Page) ([]Actor, string, error)

	// CreateUser creates a user, its actor in this tenant and a membership with
	// the given role. A handle already in use is a conflict.
	CreateUser(ctx context.Context, in CreateUserInput) (*User, error)
	// GetUser returns a user of this tenant by identifier.
	GetUser(ctx context.Context, id string) (*User, error)
	// ListUsers lists the users of this tenant, keyset paginated, with the cursor
	// for the next page. People removed from the tenant, whose actor stays for
	// history, are left out unless include_removed asks for them, each then
	// marked removed.
	ListUsers(ctx context.Context, f UserFilter) ([]User, string, error)
	// UpdateUser changes a user's display name, password, role or disabled state;
	// fields left out are unchanged. Disabling a user also revokes its API
	// tokens.
	UpdateUser(ctx context.Context, id string, in UpdateUserInput) (*User, error)
	// DeleteUser deletes a user and revokes its membership of this tenant, which
	// stops its actor acting.
	DeleteUser(ctx context.Context, id string) error

	// GrantGlobalAdmin makes the user with this email a global admin, who
	// administers every tenant. Only a global admin may call it; the first
	// one is granted from a shell holding the database.
	GrantGlobalAdmin(ctx context.Context, in GlobalAdminInput) (*User, error)
	// RevokeGlobalAdmin takes global admin away from the user with this email.
	// Revoking the last global admin is refused.
	RevokeGlobalAdmin(ctx context.Context, email string) error
	// ListGlobalAdmins lists every global admin of the installation.
	ListGlobalAdmins(ctx context.Context) ([]User, error)

	// Login is one step of an interactive sign-in. It answers with a session
	// when nothing further is needed, and with a challenge when the tenant
	// requires a second factor on the surface the caller named. The second
	// call presents the challenge's ticket and the code.
	//
	// It is two steps rather than one call taking a code beside the password,
	// because a surface that could pass both at once is a surface that could
	// take the code from a flag or an environment variable, which puts a live
	// second factor in shell history and in the process table.
	Login(ctx context.Context, in LoginInput) (*LoginResult, error)
	// Logout ends the session the caller presented. With no session to end it
	// answers unauthenticated.
	Logout(ctx context.Context) error
	// SwitchTenant moves the caller's browser session to another tenant they
	// are a member of, served on the same host. The answer carries a session
	// for the person's actor there, and the session presented stops working.
	// When the target requires a second factor the presented session did not
	// pass, the answer is a code challenge instead, and the same request sent
	// again with the code completes the switch. A tenant the caller is not a
	// member of is not found, the same answer as one that does not exist. A
	// token cannot switch: tokens stay in the tenant they were minted for.
	SwitchTenant(ctx context.Context, in SwitchTenantInput) (*LoginResult, error)
	// ListSessions lists the caller's own unexpired sessions in this tenant,
	// browser and terminal sign-ins, newest first and keyset paginated. Each
	// names when it began and expires, how it signed in and the user agent it
	// signed in with, and marks the one the caller presented; never its token.
	// A token needs account:read, which "*" does not carry.
	ListSessions(ctx context.Context, page Page) ([]SessionInfo, string, error)
	// RevokeSession ends one of the caller's own sessions by identifier, so
	// it stops signing anybody in. Another person's session is not found. A
	// token needs account:read, which "*" does not carry.
	RevokeSession(ctx context.Context, id string) error

	// SSHLoginChallenge issues a short-lived nonce for an SSH key sign-in,
	// bound to this tenant and to the host the request was made to. Nothing
	// is stored.
	SSHLoginChallenge(ctx context.Context) (*SSHChallenge, error)
	// SSHLogin is the first step of a sign-in that proves an enrolled key
	// instead of a password. It answers as Login does: a session, or the
	// second-factor challenge the password step would have returned.
	SSHLogin(ctx context.Context, in SSHLoginInput) (*LoginResult, error)

	// TOTPStatus reports the caller's own enrolment and what their tenant
	// requires of them. It discloses nothing that could produce a code.
	TOTPStatus(ctx context.Context) (*TOTPStatus, error)
	// BeginTOTPEnrolment generates a secret for the caller and hands back the
	// forms an authenticator takes. It takes effect only once a code produced
	// from it is confirmed, and is refused outright when no sealing key is
	// configured rather than storing a secret in the clear.
	BeginTOTPEnrolment(ctx context.Context) (*TOTPEnrolment, error)
	// ConfirmTOTPEnrolment proves possession and issues the recovery codes,
	// which are returned exactly once and cannot be asked for again.
	ConfirmTOTPEnrolment(ctx context.Context, code string) (*TOTPRecoveryCodes, error)
	// VerifySecondFactor proves the caller's existing second factor on its
	// own, minting nothing. It is what a surface whose first factor is not a
	// password calls: an SSH session has already proved a key when the code
	// is asked for, so there is no password step for a ticket to hang off.
	// A recovery code is accepted here as it is on the sign-in path.
	VerifySecondFactor(ctx context.Context, code string) error
	// RemoveTOTP drops the caller's own enrolment and every code beside it.
	RemoveTOTP(ctx context.Context) error
	// RegenerateRecoveryCodes replaces the caller's codes, spent or not, and
	// returns the new set once.
	RegenerateRecoveryCodes(ctx context.Context) (*TOTPRecoveryCodes, error)
	// ClearUserTOTP returns another user to an unenrolled state. It is the
	// escape from a lost device with no codes left, and it is the authority
	// an administrator holds that a member does not.
	ClearUserTOTP(ctx context.Context, userID string) error

	// CreateToken mints an API token with the given scopes, optionally pinned to
	// one project, and returns its secret value exactly once. A token cannot
	// carry scopes its creator lacks, and a name already in use is a conflict.
	CreateToken(ctx context.Context, in CreateTokenInput) (*IssuedToken, error)
	// ListTokens lists an actor's API tokens as metadata only, never their
	// values. An empty actor means the caller.
	ListTokens(ctx context.Context, actorID string) ([]APIToken, error)
	// RevokeToken revokes an API token of this tenant. Revoking a revoked token
	// changes nothing.
	RevokeToken(ctx context.Context, id string) error
	// RotateToken replaces a live API token, addressed by identifier or name,
	// with a new one of the same name, scopes and project and the same
	// lifetime counted from now, and returns its value once. The old token
	// stops at once, or after a grace of up to 24h so whatever holds it can
	// swap. A token's own actor may rotate it; another actor's needs
	// token:admin, and the new token never carries a scope its rotator lacks.
	RotateToken(ctx context.Context, id string, in RotateTokenInput) (*IssuedToken, error)

	// EnrolSSHKey registers a public key against an actor of this tenant, so
	// that key can reach the terminal interface over SSH.
	EnrolSSHKey(ctx context.Context, in EnrolSSHKeyInput) (*SSHKey, error)
	// ListSSHKeys returns an actor's enrolled keys, revoked ones included: a
	// key that stopped working is the thing an operator most wants to see.
	ListSSHKeys(ctx context.Context, actorID string) ([]SSHKey, error)
	// RevokeSSHKey stops a key authenticating. Sessions it already holds are
	// bounded by the listener's idle timeout rather than cut.
	RevokeSSHKey(ctx context.Context, id string) error
}

// WebhookService covers outgoing delivery.
type WebhookService interface {
	// PutWebhook registers an https delivery endpoint, or updates one by
	// identifier. A registration with no secret gets a generated one, returned
	// as secret in this response and never again; a supplied or kept secret is
	// not returned.
	PutWebhook(ctx context.Context, in WebhookInput) (*IssuedWebhook, error)
	// ListWebhooks lists this tenant's delivery endpoints, without their secrets.
	ListWebhooks(ctx context.Context) ([]WebhookEndpoint, error)
	// DeleteWebhook removes a delivery endpoint and its pending deliveries.
	DeleteWebhook(ctx context.Context, id string) error
	// ListDeliveries lists the webhook delivery log, one keyset-paginated page at
	// a time, with the cursor for the next page.
	ListDeliveries(ctx context.Context, f DeliveryFilter) ([]WebhookDelivery, string, error)
	// RedeliverWebhook queues an existing delivery for another attempt against
	// the same endpoint with the same event. The attempt happens on the next
	// delivery run, not inside this call.
	RedeliverWebhook(ctx context.Context, deliveryID string) error
	// PingWebhook sends one active endpoint a signed webhook.ping test event,
	// whatever its event type filter; no other endpoint receives it. It answers
	// the queued delivery, already attempted when this process drains inline,
	// which the delivery log lists and redeliver repeats like any other.
	PingWebhook(ctx context.Context, id string) (*WebhookDelivery, error)
}

// TransferService covers snapshot export and import.
//
// Both directions stream. A tenant at the scale tix targets does not fit
// comfortably in memory, and a snapshot assembled before the first byte is
// written would bound export by RAM rather than by disk.
type TransferService interface {
	// ExportTo writes a snapshot of the tenant's data as a stream of records,
	// walking the data as it writes.
	ExportTo(ctx context.Context, in ExportInput, w io.Writer) error
	// ImportFrom reads a snapshot record by record and merges it into, or
	// replaces, the tenant's data. A dry run reports the counts and writes
	// nothing.
	ImportFrom(ctx context.Context, r io.Reader, in ImportInput) (*ImportResult, error)
	// ExportTasks writes one project's tasks as CSV, JSON, NDJSON or a
	// Markdown table, as it walks them. The selection takes the task list's
	// query language, and the columns, their order, the date style and the
	// CSV delimiter are the caller's to choose. A CSV or Markdown cell that a
	// spreadsheet would read as a formula starts with a quote.
	ExportTasks(ctx context.Context, in TaskExportInput, w io.Writer) error
	// ImportTasks reads a file of tasks into one project: CSV, JSON or NDJSON,
	// including files Jira, GitHub, Trello and Linear write, or a list of
	// quick-add lines. Columns and values are mapped by the mapping file the
	// input carries, or by tix's own suggestion, which reads a tix export and
	// the common headers as they are. A row matched by external id or
	// reference is updated, skipped or created again as the mode says. A dry
	// run reports what would happen and writes nothing, and so does an import
	// with any row in error: the import is all or nothing. To add a handful of
	// tasks, creating each one is simpler.
	ImportTasks(ctx context.Context, r io.Reader, in TaskImportInput) (*TaskImportResult, error)
}

// BundleService covers sharing reusable components between projects, tenants
// and installations.
//
// A bundle carries configuration and never work items, so sharing a way of
// working cannot disclose what anyone is working on.
type BundleService interface {
	// ExportBundle writes the selected components as it walks them.
	ExportBundle(ctx context.Context, in BundleExportInput, w io.Writer) error
	// ImportBundle reads a bundle and applies it atomically. With Preview set
	// it reports what it would do and writes nothing at all.
	ImportBundle(ctx context.Context, r io.Reader, in BundleImportInput) (*BundleResult, error)
}

// ComponentKind names a kind of reusable component a bundle can carry.
type ComponentKind string

// Component kinds. Work items are deliberately absent.
const (
	ComponentWorkflow ComponentKind = "workflow"
	ComponentFieldDef ComponentKind = "field_def"
	ComponentTag      ComponentKind = "tag"
	ComponentProject  ComponentKind = "project_template"
	ComponentWebhook  ComponentKind = "webhook"
)

// ComponentKinds lists every kind a bundle may carry.
var ComponentKinds = []ComponentKind{
	ComponentWorkflow, ComponentFieldDef, ComponentTag, ComponentProject, ComponentWebhook,
}

// Valid reports whether k is a known component kind.
func (k ComponentKind) Valid() bool {
	for _, known := range ComponentKinds {
		if k == known {
			return true
		}
	}
	return false
}

// BundleVersion is the current bundle schema version.
const BundleVersion = 1

// BundleExportInput selects what a bundle carries.
type BundleExportInput struct {
	// Name labels the bundle for the people who receive it.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// Kinds restricts the export. Empty exports every kind the selectors reach.
	Kinds []ComponentKind `json:"kinds,omitempty" yaml:"kinds,omitempty"`
	// WorkflowKeys selects individual workflows by key.
	WorkflowKeys []string `json:"workflow_keys,omitempty" yaml:"workflow_keys,omitempty"`
	// ProjectRefs selects projects by key or identifier, exported as project
	// templates.
	ProjectRefs []string `json:"project_refs,omitempty" yaml:"project_refs,omitempty"`
	// WebhookIDs selects individual webhook endpoints by identifier. Secrets
	// never travel in a bundle.
	WebhookIDs []string `json:"webhook_ids,omitempty" yaml:"webhook_ids,omitempty"`
}

// Validate checks the selection.
func (in BundleExportInput) Validate() error {
	for _, k := range in.Kinds {
		if !k.Valid() {
			return Invalid("component kind %q is not one this build can share", k)
		}
	}
	return nil
}

// CollisionPolicy decides what an import does when a component's key is taken.
type CollisionPolicy string

// Collision policies. There is deliberately no default, because replacing a
// component somebody else is using should never be implicit.
const (
	CollisionSkip    CollisionPolicy = "skip"
	CollisionRename  CollisionPolicy = "rename"
	CollisionReplace CollisionPolicy = "replace"
)

// CollisionPolicies lists every collision policy.
var CollisionPolicies = []CollisionPolicy{CollisionSkip, CollisionRename, CollisionReplace}

// Valid reports whether p is a known collision policy.
func (p CollisionPolicy) Valid() bool { return slices.Contains(CollisionPolicies, p) }

// BundleImportInput controls an import.
type BundleImportInput struct {
	// OnCollision decides what happens to a component whose key is already
	// taken: skip it, rename the incoming one, or replace the existing one.
	// It is required.
	OnCollision CollisionPolicy `json:"on_collision" yaml:"on_collision"`
	// Preview reports the plan and writes nothing, not even an audit entry.
	Preview bool `json:"preview,omitempty" yaml:"preview,omitempty"`
	// ProjectRef targets project-scoped components such as field definitions.
	ProjectRef string `json:"project_ref,omitempty" yaml:"project_ref,omitempty"`
}

// Validate checks the input.
func (in BundleImportInput) Validate() error {
	switch {
	case in.OnCollision.Valid():
		return nil
	case in.OnCollision == "":
		return Invalid("a collision policy is required; there is no default because replace overwrites a component others may be using")
	default:
		return Invalid("collision policy %q must be %q, %q or %q",
			in.OnCollision, CollisionSkip, CollisionRename, CollisionReplace)
	}
}

// ComponentAction names what an import did, or would do, to one component.
type ComponentAction string

// Component actions.
const (
	ActionCreated ComponentAction = "created"
	ActionUpdated ComponentAction = "updated"
	ActionRenamed ComponentAction = "renamed"
	ActionSkipped ComponentAction = "skipped"
)

// ComponentOutcome reports what happened to one component.
type ComponentOutcome struct {
	// Kind is the kind of component.
	Kind ComponentKind `json:"kind" yaml:"kind"`
	// Key is the component's key as the bundle carries it.
	Key string `json:"key" yaml:"key"`
	// Action is what the import did, or would do, to the component.
	Action ComponentAction `json:"action" yaml:"action"`
	// NewKey is set when the component was renamed to avoid a collision.
	NewKey string `json:"new_key,omitempty" yaml:"new_key,omitempty"`
	// Reason explains a rename or a skip.
	Reason string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// BundleResult reports an import, or what a preview would have done.
type BundleResult struct {
	// BundleName is the name the bundle was exported under, if any.
	BundleName string `json:"bundle_name,omitempty" yaml:"bundle_name,omitempty"`
	// BundleVersion is the bundle format version that was read.
	BundleVersion int `json:"bundle_version" yaml:"bundle_version"`
	// Outcomes lists what happened to each component, in bundle order.
	Outcomes []ComponentOutcome `json:"outcomes" yaml:"outcomes"`
	// Warnings lists things the import noticed but did not refuse.
	Warnings []string `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	// Preview is true when nothing was written.
	Preview bool `json:"preview" yaml:"preview"`
}

// SyncService covers one-way import from external systems.
type SyncService interface {
	// PutSyncSource registers an external system to import from, or updates one
	// in place. Sync runs on the server, so everything a run needs can be stored
	// here: the mapping document itself and the credentials, which are sealed
	// with the server's key and never returned. Anything not stored is read from
	// TIX_SYNC_<SOURCE>_* in the server's environment. With check set it
	// validates and describes the source and writes nothing.
	PutSyncSource(ctx context.Context, in SyncSourceInput) (*SyncSource, error)
	// ListSyncSources lists this tenant's external import sources, each with
	// where its mapping, credentials and settings come from on the server and
	// what a run would still lack.
	ListSyncSources(ctx context.Context) ([]SyncSource, error)
	// DeleteSyncSource removes an import source and its cursor.
	DeleteSyncSource(ctx context.Context, id string) error
	// RunSync imports from a configured source, creating what is new, updating
	// what changed and skipping the rest. A rerun refreshes rather than
	// duplicating, and a dry run writes nothing. The run executes on the server
	// and is refused, naming what is missing there, before anything is read.
	RunSync(ctx context.Context, in RunSyncInput) (*SyncResult, error)
}

// Session is a browser or terminal login.
type Session struct {
	// Token is the session secret, presented as the session cookie.
	Token string `json:"token" yaml:"token"`
	// ActorID is the actor the session acts as.
	ActorID string `json:"actor_id" yaml:"actor_id"`
	// TenantID is the tenant the session belongs to.
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	// ExpiresAt is when the session stops working, in RFC 3339.
	ExpiresAt time.Time `json:"expires_at" yaml:"expires_at"`
}

// SessionInfo describes one of the caller's sessions without its secret.
type SessionInfo struct {
	// ID identifies the session for revoking it. It is not a credential.
	ID string `json:"id" yaml:"id"`
	// CreatedAt is when the sign-in happened.
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	// ExpiresAt is when the session stops working.
	ExpiresAt time.Time `json:"expires_at" yaml:"expires_at"`
	// Method is how it signed in: password, ssh-key or switch.
	Method string `json:"method" yaml:"method"`
	// SecondFactor reports that the sign-in passed a second factor.
	SecondFactor bool `json:"second_factor" yaml:"second_factor"`
	// UserAgent is the user agent the sign-in request sent, truncated, or
	// empty for a session that predates recording it.
	UserAgent string `json:"user_agent" yaml:"user_agent"`
	// Current marks the session the caller presented with this request.
	Current bool `json:"current" yaml:"current"`
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

// SnapshotVersion is the current snapshot format version. Version 2 added
// tombstone task records for soft-deleted tasks; a version 1 document never
// contains one, so this build reads both.
const SnapshotVersion = 2

// SnapshotVersionMin is the oldest format version this build still reads.
const SnapshotVersionMin = 1

// ImportMode selects how an import treats existing data.
type ImportMode string

// Import modes.
const (
	ImportMerge   ImportMode = "merge"
	ImportReplace ImportMode = "replace"
)

// ImportModes lists every import mode.
var ImportModes = []ImportMode{ImportMerge, ImportReplace}

// Valid reports whether m is a known import mode.
func (m ImportMode) Valid() bool { return slices.Contains(ImportModes, m) }

// ImportResult reports what an import did, or would do under a dry run.
type ImportResult struct {
	// Created counts the records created, by record kind.
	Created map[string]int `json:"created" yaml:"created"`
	// Updated counts the records updated, by record kind.
	Updated map[string]int `json:"updated" yaml:"updated"`
	// Skipped counts the records left alone, by record kind.
	Skipped map[string]int `json:"skipped" yaml:"skipped"`
	// Deleted counts the records deleted, by record kind.
	Deleted map[string]int `json:"deleted,omitempty" yaml:"deleted,omitempty"`
	// Warnings lists records the import could not take as they were.
	Warnings []string `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	// DryRun is true when nothing was written.
	DryRun bool `json:"dry_run" yaml:"dry_run"`
}

// SyncResult reports what an external import did, or would do under a dry run.
type SyncResult struct {
	ImportResult
	// System is the external system the source imports from.
	System string `json:"system" yaml:"system"`
	// Source is the name of the sync source that ran.
	Source string `json:"source" yaml:"source"`
	// Cursor is where the next incremental run resumes.
	Cursor string `json:"cursor,omitempty" yaml:"cursor,omitempty"`
	// UnmatchedUsers lists, sorted and once each, the source assignees no tix
	// actor matched; they were left unassigned or given placeholder actors.
	UnmatchedUsers []string `json:"unmatched_users,omitempty" yaml:"unmatched_users,omitempty"`
}

// PruneResult reports what pruning removed, or would remove under a dry run.
type PruneResult struct {
	// Events counts the events removed.
	Events int64 `json:"events" yaml:"events"`
	// AuditEntries counts the audit entries removed.
	AuditEntries int64 `json:"audit_entries" yaml:"audit_entries"`
	// WebhookDeliveries counts the webhook delivery records removed.
	WebhookDeliveries int64 `json:"webhook_deliveries" yaml:"webhook_deliveries"`
	// IdempotencyKeys counts the expired idempotency keys removed.
	IdempotencyKeys int64 `json:"idempotency_keys" yaml:"idempotency_keys"`
	// RetainedForSubscribers counts events past their window kept because a
	// subscriber has not consumed them yet.
	RetainedForSubscribers int64 `json:"retained_for_subscribers" yaml:"retained_for_subscribers"`
	// DryRun is true when nothing was removed.
	DryRun bool `json:"dry_run" yaml:"dry_run"`
}
