// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import "time"

// The types below describe the query parameters the handlers read, for the
// OpenAPI document. The handlers parse the URL themselves; the contract test
// fails when a request carries a parameter its operation does not declare.

// PageQuery is the keyset page every paginated listing reads.
type PageQuery struct {
	// Limit is the most items one page holds: default 50, at most 500.
	Limit int `query:"limit"`
	// Cursor is the next_cursor of the previous page, to read the page after
	// it. Leave it out for the first page.
	Cursor string `query:"cursor"`
	// Sort names the field the listing is ordered by. Each listing has its own
	// default and refuses a field it cannot sort by.
	Sort string `query:"sort"`
	// Direction is asc or desc, default asc.
	Direction string `query:"direction" enum:"asc,desc"`
}

// TaskListQuery is the task filter a task listing reads.
type TaskListQuery struct {
	PageQuery
	// Page reads the listing from page N, counting from one, of limit tasks
	// each, instead of from a cursor; the two cannot be combined. A number
	// past the end answers the last page, and the response says which page it
	// served. The cursor stays the way to walk a listing; this is for landing
	// on a page a person asked for.
	Page int `query:"page"`
	// Tally is true to have the response count the whole selection, every
	// page of it, in tally: tasks per project and status, tasks waiting on a
	// dependency and tasks under a live claim. It costs one grouped count.
	Tally bool `query:"tally"`
	// Project keeps tasks in a project, named by key, not case sensitive;
	// repeat it for several. An unknown key is an error.
	Project []string `query:"project"`
	// ProjectID keeps tasks in a project, named by ID; repeat it for several.
	ProjectID []string `query:"project_id"`
	// Status keeps tasks in a workflow state, named by key, not case
	// sensitive; repeat it for several. A state no workflow declares is an
	// error.
	Status []string `query:"status"`
	// Tag keeps tasks carrying a tag; repeat it for several, and a task must
	// carry every one.
	Tag []string `query:"tag"`
	// Assignee keeps tasks assigned to an actor, named by ID, handle or me;
	// repeat it for several.
	Assignee []string `query:"assignee"`
	// Creator keeps tasks created by an actor, named by ID, handle or me;
	// repeat it for several.
	Creator []string `query:"creator"`
	// ClaimedBy keeps tasks claimed by an actor, named by ID, handle or me;
	// repeat it for several.
	ClaimedBy []string `query:"claimed_by"`
	// ParentID keeps the direct subtasks of one task, named by reference such
	// as INFRA-42 or by ID. It cannot be combined with root_only.
	ParentID string `query:"parent_id"`
	// Q is a free text search over title and body: a substring match on
	// SQLite, a full text match on PostgreSQL.
	Q string `query:"q"`
	// RootOnly is true to keep only tasks without a parent.
	RootOnly bool `query:"root_only"`
	// IncludeDeleted is true to list tasks in the trash alongside live ones.
	IncludeDeleted bool `query:"include_deleted"`
	// Claimed is true for tasks under a live claim, false for the rest, and
	// absent for either.
	Claimed string `query:"claimed"`
	// Starred is true for tasks the caller starred, false for the rest, and
	// absent for either.
	Starred string `query:"starred"`
	// Blocked is true for tasks waiting on an unfinished dependency, false for
	// the rest, and absent for either.
	Blocked string `query:"blocked"`
	// Recurring is true for tasks with a repeat rule, false for the rest, and
	// absent for either.
	Recurring string `query:"recurring"`
	// Checklist is true for tasks whose body holds a checklist, false for the
	// rest, and absent for either.
	Checklist string `query:"checklist"`
	// ChecklistOpen is true for tasks with an unticked checklist item, false
	// for the rest, and absent for either.
	ChecklistOpen string `query:"checklist_open"`
	// Trashed is true for the trash alone, false for live tasks alone, and
	// absent to follow include_deleted.
	Trashed string `query:"trashed"`
	// Snoozed is true for snoozed tasks alone, false for the rest, or any for
	// both. Absent leaves snoozed tasks out.
	Snoozed string `query:"snoozed"`
	// Priority keeps tasks of a priority from 1, the highest, to 5, the
	// lowest; repeat it for several.
	Priority []int `query:"priority"`
	// NotProject leaves out tasks in a project, named by key; repeat it for
	// several.
	NotProject []string `query:"not_project"`
	// NotStatus leaves out tasks in a workflow state; repeat it for several.
	NotStatus []string `query:"not_status"`
	// NotTag leaves out tasks carrying a tag; repeat it for several.
	NotTag []string `query:"not_tag"`
	// NotAssignee leaves out tasks assigned to an actor, named by ID, handle
	// or me; repeat it for several.
	NotAssignee []string `query:"not_assignee"`
	// NotCreator leaves out tasks created by an actor, named by ID, handle or
	// me; repeat it for several.
	NotCreator []string `query:"not_creator"`
	// NotClaimedBy leaves out tasks claimed by an actor, named by ID, handle
	// or me; repeat it for several.
	NotClaimedBy []string `query:"not_claimed_by"`
	// NotPriority leaves out tasks of a priority from 1 to 5; repeat it for
	// several.
	NotPriority []int `query:"not_priority"`
	// Text is a title or body predicate: title, body or any, then : for the
	// whole field or ~ for a substring, then the value, as in title~api. A
	// leading - negates it. Case is ignored; repeat it, and every term must
	// hold.
	Text []string `query:"text"`
	// DueBefore keeps tasks due before an RFC 3339 instant, such as
	// 2026-05-01T00:00:00Z.
	DueBefore time.Time `query:"due_before"`
	// DueAfter keeps tasks due after an RFC 3339 instant, such as
	// 2026-05-01T00:00:00Z.
	DueAfter time.Time `query:"due_after"`
	// CustomFields is a JSON object of custom field values a task must hold,
	// such as {"severity":"high","points":3}. Each value is a string, number
	// or boolean. A parameter field.KEY matches one field and wins over this.
	CustomFields string `query:"custom_fields"`
	// Format is csv to answer the page as CSV instead of JSON.
	Format string `query:"format" enum:"csv"`
}

// QueryPrefixes names the parameter families a task listing reads by prefix.
func (TaskListQuery) QueryPrefixes() []string { return []string{FieldFilterPrefix} }

// AuditQuery is the filter an audit listing reads.
type AuditQuery struct {
	PageQuery
	// SubjectType keeps entries about one kind of record, such as task,
	// project or workflow.
	SubjectType string `query:"subject_type"`
	// SubjectID keeps entries about one record, named by its ID.
	SubjectID string `query:"subject_id"`
	// ActorID keeps entries recorded by an actor, named by ID; repeat it for
	// several.
	ActorID []string `query:"actor_id"`
	// Action keeps entries of an action such as task.create or
	// project.update; repeat it for several.
	Action []string `query:"action"`
	// Text is a case-insensitive substring searched for in the action, the
	// kind of record, the source and the recorded before and after values.
	Text string `query:"text"`
	// Source keeps entries that arrived from a surface: cli, api, web, tui or
	// system; repeat it for several.
	Source []string `query:"source"`
	// Via keeps entries that arrived through an access channel: web, api,
	// cli-local, cli-remote, ssh, mcp, sync, import, system or unknown; repeat
	// it for several.
	Via []string `query:"via"`
	// Token keeps entries made with an API token, named by the token's name;
	// repeat it for several.
	Token []string `query:"token"`
	// Since keeps entries recorded at or after an RFC 3339 instant.
	Since time.Time `query:"since"`
	// Until keeps entries recorded at or before an RFC 3339 instant.
	Until time.Time `query:"until"`
}

// TaskAuditQuery is the filter a task's own audit trail reads.
type TaskAuditQuery struct {
	PageQuery
	// ActorID keeps entries recorded by an actor, named by ID; repeat it for
	// several.
	ActorID []string `query:"actor_id"`
	// Action keeps entries of an action such as task.create or
	// project.update; repeat it for several.
	Action []string `query:"action"`
	// Text is a case-insensitive substring searched for in the action, the
	// kind of record, the source and the recorded before and after values.
	Text string `query:"text"`
	// Source keeps entries that arrived from a surface: cli, api, web, tui or
	// system; repeat it for several.
	Source []string `query:"source"`
	// Via keeps entries that arrived through an access channel: web, api,
	// cli-local, cli-remote, ssh, mcp, sync, import, system or unknown; repeat
	// it for several.
	Via []string `query:"via"`
	// Token keeps entries made with an API token, named by the token's name;
	// repeat it for several.
	Token []string `query:"token"`
	// Since keeps entries recorded at or after an RFC 3339 instant.
	Since time.Time `query:"since"`
	// Until keeps entries recorded at or before an RFC 3339 instant.
	Until time.Time `query:"until"`
}

// DeleteTaskQuery is how a task deletion is asked for.
type DeleteTaskQuery struct {
	// Hard is true to delete the task for good instead of moving it to the
	// trash.
	Hard bool `query:"hard"`
	// Cascade is true to delete the task's whole subtree with it. Without it,
	// a task with subtasks is refused.
	Cascade bool `query:"cascade"`
}

// TreeQuery bounds a task tree.
type TreeQuery struct {
	// Depth is how many levels below the task to include; 0 or absent is
	// every level.
	Depth int `query:"depth"`
}

// ProjectQuery names one project, or none for the tenant default.
type ProjectQuery struct {
	// Project is the key or ID of the project whose policy to show; absent
	// shows the tenant's.
	Project string `query:"project"`
}

// ActorQuery names whose credentials to list.
type ActorQuery struct {
	// ActorID is the ID of the actor whose credentials to list; absent means
	// the caller.
	ActorID string `query:"actor_id"`
}

// InboxQuery is the filter an inbox listing reads.
type InboxQuery struct {
	PageQuery
	// Unread is true to keep unread items only.
	Unread bool `query:"unread"`
}

// ProjectListQuery is the filter a project listing reads.
type ProjectListQuery struct {
	PageQuery
	// Key keeps a project, named by key; repeat it for several.
	Key []string `query:"key"`
	// IncludeArchived is true to list archived projects too.
	IncludeArchived bool `query:"include_archived"`
}

// UserListQuery is the filter a user listing reads.
type UserListQuery struct {
	PageQuery
	// IncludeRemoved is true to list people removed from the tenant too,
	// each marked removed.
	IncludeRemoved bool `query:"include_removed"`
	// WithoutTOTP is true to list only people who hold no confirmed second
	// factor.
	WithoutTOTP bool `query:"without_totp"`
}

// ProjectDeleteQuery is how a project deletion is asked for.
type ProjectDeleteQuery struct {
	// Archive is true to archive the project instead of deleting it, which
	// keeps its tasks and answers 204.
	Archive bool `query:"archive"`
	// MoveTasksTo is the key or ID of the project the tasks move to. A project
	// holding tasks needs this or delete_tasks, not both.
	MoveTasksTo string `query:"move_tasks_to"`
	// DeleteTasks is true to delete the project's tasks permanently with it.
	DeleteTasks bool `query:"delete_tasks"`
}

// DeletionPlanQuery is the deletion a plan is drawn up for.
type DeletionPlanQuery struct {
	// MoveTasksTo is the key or ID of the project the tasks would move to.
	MoveTasksTo string `query:"move_tasks_to"`
	// DeleteTasks is true to plan deleting the tasks with the project.
	DeleteTasks bool `query:"delete_tasks"`
}

// StatsQuery is the window a statistics report covers.
type StatsQuery struct {
	// Project is the key or ID of one project to report on; absent covers
	// the whole tenant.
	Project string `query:"project"`
	// Since is the RFC 3339 instant the window starts at. It overrides window.
	Since time.Time `query:"since"`
	// Window is how far back the report reaches, a duration such as 7d or
	// 168h: default 14 days, at most 3650.
	Window string `query:"window"`
	// Top is how many actors the report ranks: default 5, at most 100.
	Top int `query:"top"`
	// Oldest is how many of the oldest open tasks the report lists: default
	// 5, at most 100.
	Oldest int `query:"oldest"`
}

// EventPollQuery is what a poll of the event log reads.
type EventPollQuery struct {
	// SinceSeq is the exclusive cursor: events after this sequence number are
	// returned. Absent or 0 starts at the newest event.
	SinceSeq int64 `query:"since_seq"`
	// Types keeps events of these types, comma separated or repeated; a
	// trailing * matches a prefix, as in task.*.
	Types []string `query:"types"`
	// ProjectIDs keeps events of these projects, comma separated or repeated.
	ProjectIDs []string `query:"project_ids"`
	// Limit is how many events to return at most: default 200, at most 1000.
	Limit int `query:"limit"`
	// Wait is how long to hold the request when no event is available yet, a
	// duration such as 25s or a number of seconds: default 0, at most 25s.
	Wait string `query:"wait"`
}

// TemplateListQuery is the filter a template listing reads.
type TemplateListQuery struct {
	PageQuery
	// Project is the key or ID of a project: the listing keeps its templates
	// and the tenant-wide ones.
	Project string `query:"project"`
}

// BundleImportQuery is how a bundle import is asked for.
type BundleImportQuery struct {
	// OnCollision is what happens to a component whose key is taken: skip
	// keeps the existing one, rename imports under a new key, replace
	// overwrites. Required when a key collides; there is no default.
	OnCollision string `query:"on_collision"`
	// Preview is true to report the plan and write nothing.
	Preview bool `query:"preview"`
	// ProjectRef is the key or ID of the project that project-scoped
	// components, such as field definitions, land in.
	ProjectRef string `query:"project_ref"`
}

// ImportQuery is how a snapshot import is asked for.
type ImportQuery struct {
	// Mode is required: merge adds and updates, while replace also removes the
	// tasks and field definitions of an imported project that the snapshot
	// does not carry.
	Mode string `query:"mode"`
	// DryRun is true to report what the import would do and write nothing.
	DryRun bool `query:"dry_run"`
}

// GroupQuery names one group of a view.
type GroupQuery struct {
	// Group is the group to reset, spelled as in the view's groups; absent
	// resets every group of the view.
	Group string `query:"group"`
}

// DeliveryListQuery is the filter a webhook delivery listing reads.
type DeliveryListQuery struct {
	PageQuery
	// EndpointID keeps the deliveries of one webhook, named by ID.
	EndpointID string `query:"endpoint_id"`
	// Status keeps deliveries in a state: pending, delivered or failed;
	// repeat it for several.
	Status []string `query:"status"`
}
