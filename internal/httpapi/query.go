// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import "time"

// The types below describe the query parameters the handlers read, for the
// OpenAPI document. The handlers parse the URL themselves; the contract test
// fails when a request carries a parameter its operation does not declare.

// PageQuery is the keyset page every paginated listing reads.
type PageQuery struct {
	// Limit is the most items one page holds.
	Limit int `query:"limit"`
	// Cursor is the next_cursor of the previous page.
	Cursor string `query:"cursor"`
	// Sort names the field the listing is ordered by.
	Sort string `query:"sort"`
	// Direction is asc or desc.
	Direction string `query:"direction" enum:"asc,desc"`
}

// TaskListQuery is the task filter a task listing reads.
type TaskListQuery struct {
	PageQuery
	// Project is a project key; repeat it for several.
	Project []string `query:"project"`
	// ProjectID is a project identifier; repeat it for several.
	ProjectID []string `query:"project_id"`
	// Status is a status key; repeat it for several.
	Status []string `query:"status"`
	// Tag is a tag every task must carry; repeat it for several.
	Tag []string `query:"tag"`
	// Assignee is an assignee actor identifier; repeat it for several.
	Assignee []string `query:"assignee"`
	// Creator is a creator actor identifier; repeat it for several.
	Creator []string `query:"creator"`
	// ClaimedBy is the actor holding a claim; repeat it for several.
	ClaimedBy []string `query:"claimed_by"`
	// ParentID limits the listing to one task's children.
	ParentID string `query:"parent_id"`
	// Q is a free text search.
	Q string `query:"q"`
	// RootOnly keeps tasks without a parent.
	RootOnly bool `query:"root_only"`
	// IncludeDeleted keeps deleted tasks.
	IncludeDeleted bool `query:"include_deleted"`
	// Claimed is true, false or empty for either.
	Claimed string `query:"claimed"`
	// Starred is true, false or empty for either.
	Starred string `query:"starred"`
	// Blocked is true, false or empty for either.
	Blocked string `query:"blocked"`
	// Recurring is true, false or empty for either.
	Recurring string `query:"recurring"`
	// Checklist is true, false or empty for either.
	Checklist string `query:"checklist"`
	// ChecklistOpen is true, false or empty for either.
	ChecklistOpen string `query:"checklist_open"`
	// Trashed is true, false or empty for either.
	Trashed string `query:"trashed"`
	// Snoozed is true, false, or any to include snoozed tasks with the rest.
	Snoozed string `query:"snoozed"`
	// Priority is a priority; repeat it for several.
	Priority []int `query:"priority"`
	// NotProject excludes a project key; repeat it for several.
	NotProject []string `query:"not_project"`
	// NotStatus excludes a status key; repeat it for several.
	NotStatus []string `query:"not_status"`
	// NotTag excludes a tag; repeat it for several.
	NotTag []string `query:"not_tag"`
	// NotAssignee excludes an assignee; repeat it for several.
	NotAssignee []string `query:"not_assignee"`
	// NotCreator excludes a creator; repeat it for several.
	NotCreator []string `query:"not_creator"`
	// NotClaimedBy excludes a claim holder; repeat it for several.
	NotClaimedBy []string `query:"not_claimed_by"`
	// NotPriority excludes a priority; repeat it for several.
	NotPriority []int `query:"not_priority"`
	// Text is a text term such as title~api or -body~legacy; repeat it.
	Text []string `query:"text"`
	// DueBefore keeps tasks due before an RFC 3339 instant.
	DueBefore time.Time `query:"due_before"`
	// DueAfter keeps tasks due after an RFC 3339 instant.
	DueAfter time.Time `query:"due_after"`
	// CustomFields is a JSON object of custom field values to match. A
	// parameter field.KEY matches one field and wins over this one.
	CustomFields string `query:"custom_fields"`
	// Format is csv to answer the page as CSV.
	Format string `query:"format" enum:"csv"`
}

// QueryPrefixes names the parameter families a task listing reads by prefix.
func (TaskListQuery) QueryPrefixes() []string { return []string{FieldFilterPrefix} }

// AuditQuery is the filter an audit listing reads.
type AuditQuery struct {
	PageQuery
	// SubjectType is the kind of record the entries are about.
	SubjectType string `query:"subject_type"`
	// SubjectID is the record the entries are about.
	SubjectID string `query:"subject_id"`
	// ActorID is an actor who acted; repeat it for several.
	ActorID []string `query:"actor_id"`
	// Action is an action name; repeat it for several.
	Action []string `query:"action"`
	// Text is a free text search.
	Text string `query:"text"`
	// Source is the surface an entry came from; repeat it for several.
	Source []string `query:"source"`
	// Since is the earliest RFC 3339 instant.
	Since time.Time `query:"since"`
	// Until is the latest RFC 3339 instant.
	Until time.Time `query:"until"`
}

// TaskAuditQuery is the filter a task's own audit trail reads.
type TaskAuditQuery struct {
	PageQuery
	// ActorID is an actor who acted; repeat it for several.
	ActorID []string `query:"actor_id"`
	// Action is an action name; repeat it for several.
	Action []string `query:"action"`
	// Text is a free text search.
	Text string `query:"text"`
	// Source is the surface an entry came from; repeat it for several.
	Source []string `query:"source"`
	// Since is the earliest RFC 3339 instant.
	Since time.Time `query:"since"`
	// Until is the latest RFC 3339 instant.
	Until time.Time `query:"until"`
}

// DeleteTaskQuery is how a task deletion is asked for.
type DeleteTaskQuery struct {
	// Hard deletes the task for good instead of moving it to the trash.
	Hard bool `query:"hard"`
	// Cascade deletes the task's subtasks with it.
	Cascade bool `query:"cascade"`
}

// TreeQuery bounds a task tree.
type TreeQuery struct {
	// Depth is how many levels below the task to include; 0 is all.
	Depth int `query:"depth"`
}

// ProjectQuery names one project, or none for the tenant default.
type ProjectQuery struct {
	// Project is a project key.
	Project string `query:"project"`
}

// ActorQuery names whose credentials to list.
type ActorQuery struct {
	// ActorID is the actor; empty means the caller.
	ActorID string `query:"actor_id"`
}

// InboxQuery is the filter an inbox listing reads.
type InboxQuery struct {
	PageQuery
	// Unread keeps unread items only.
	Unread bool `query:"unread"`
}

// ProjectListQuery is the filter a project listing reads.
type ProjectListQuery struct {
	PageQuery
	// Key is a project key; repeat it for several.
	Key []string `query:"key"`
	// IncludeArchived keeps archived projects.
	IncludeArchived bool `query:"include_archived"`
}

// ProjectDeleteQuery is how a project deletion is asked for.
type ProjectDeleteQuery struct {
	// Archive archives the project instead of deleting it, answering 204.
	Archive bool `query:"archive"`
	// MoveTasksTo is the project key the tasks move to.
	MoveTasksTo string `query:"move_tasks_to"`
	// DeleteTasks deletes the tasks with the project.
	DeleteTasks bool `query:"delete_tasks"`
}

// DeletionPlanQuery is the deletion a plan is drawn up for.
type DeletionPlanQuery struct {
	// MoveTasksTo is the project key the tasks would move to.
	MoveTasksTo string `query:"move_tasks_to"`
	// DeleteTasks would delete the tasks with the project.
	DeleteTasks bool `query:"delete_tasks"`
}

// StatsQuery is the window a statistics report covers.
type StatsQuery struct {
	// Project is a project key; empty means every project.
	Project string `query:"project"`
	// Since is the RFC 3339 instant the window starts at.
	Since time.Time `query:"since"`
	// Window is a duration such as 7d or 168h.
	Window string `query:"window"`
	// Top is how many actors the report ranks.
	Top int `query:"top"`
	// Oldest is how many of the oldest open tasks the report lists.
	Oldest int `query:"oldest"`
}

// TemplateListQuery is the filter a template listing reads.
type TemplateListQuery struct {
	PageQuery
	// Project is a project key.
	Project string `query:"project"`
}

// BundleImportQuery is how a bundle import is asked for.
type BundleImportQuery struct {
	// OnCollision is the collision policy.
	OnCollision string `query:"on_collision"`
	// Preview reports what would change and writes nothing.
	Preview bool `query:"preview"`
	// ProjectRef is the project the bundle's components land in.
	ProjectRef string `query:"project_ref"`
}

// ImportQuery is how a snapshot import is asked for.
type ImportQuery struct {
	// Mode is the import mode.
	Mode string `query:"mode"`
	// DryRun reports what would change and writes nothing.
	DryRun bool `query:"dry_run"`
}

// GroupQuery names one group of a view.
type GroupQuery struct {
	// Group is the group to reset; empty resets every group.
	Group string `query:"group"`
}

// DeliveryListQuery is the filter a webhook delivery listing reads.
type DeliveryListQuery struct {
	PageQuery
	// EndpointID is the webhook the deliveries belong to.
	EndpointID string `query:"endpoint_id"`
	// Status is a delivery status; repeat it for several.
	Status []string `query:"status"`
}
