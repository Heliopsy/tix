// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"slices"
	"strings"
)

// TaskFormat is a file format a project's tasks are exported in.
type TaskFormat string

// Task export formats.
const (
	TaskFormatCSV      TaskFormat = "csv"
	TaskFormatJSON     TaskFormat = "json"
	TaskFormatNDJSON   TaskFormat = "ndjson"
	TaskFormatMarkdown TaskFormat = "markdown"
)

// TaskFormats lists the export formats in the order a chooser offers them.
var TaskFormats = []TaskFormat{TaskFormatCSV, TaskFormatJSON, TaskFormatNDJSON, TaskFormatMarkdown}

// Valid reports whether f is a known export format.
func (f TaskFormat) Valid() bool { return slices.Contains(TaskFormats, f) }

// Extension is the file name extension a download in this format carries.
func (f TaskFormat) Extension() string {
	if f == TaskFormatMarkdown {
		return "md"
	}
	return string(f)
}

// Task export columns. A custom field is the column "custom:" and its key.
const (
	TaskColumnRef         = "ref"
	TaskColumnID          = "id"
	TaskColumnProject     = "project"
	TaskColumnTitle       = "title"
	TaskColumnBody        = "body"
	TaskColumnStatus      = "status"
	TaskColumnPriority    = "priority"
	TaskColumnAssignee    = "assignee"
	TaskColumnCreator     = "creator"
	TaskColumnTags        = "tags"
	TaskColumnDue         = "due"
	TaskColumnParent      = "parent"
	TaskColumnDependsOn   = "depends_on"
	TaskColumnRecurrence  = "recurrence"
	TaskColumnCreatedAt   = "created_at"
	TaskColumnUpdatedAt   = "updated_at"
	TaskColumnStartedAt   = "started_at"
	TaskColumnCompletedAt = "completed_at"
	TaskColumnDeletedAt   = "deleted_at"
	TaskColumnComments    = "comments"
	TaskColumnHistory     = "history"

	// TaskColumnCustomPrefix starts the column of a custom field.
	TaskColumnCustomPrefix = "custom:"
)

// TaskColumns is every built-in export column, in the default order.
var TaskColumns = []string{
	TaskColumnRef, TaskColumnTitle, TaskColumnStatus, TaskColumnPriority, TaskColumnAssignee,
	TaskColumnTags, TaskColumnDue, TaskColumnParent, TaskColumnDependsOn, TaskColumnRecurrence,
	TaskColumnBody, TaskColumnCreatedAt, TaskColumnUpdatedAt, TaskColumnStartedAt,
	TaskColumnCompletedAt, TaskColumnDeletedAt, TaskColumnCreator, TaskColumnProject, TaskColumnID,
	TaskColumnComments, TaskColumnHistory,
}

// DefaultTaskColumns is what an export carries when it names no columns,
// before the project's custom fields are added after them.
var DefaultTaskColumns = []string{
	TaskColumnRef, TaskColumnTitle, TaskColumnStatus, TaskColumnPriority, TaskColumnAssignee,
	TaskColumnTags, TaskColumnDue, TaskColumnParent, TaskColumnDependsOn, TaskColumnRecurrence,
	TaskColumnBody, TaskColumnCreatedAt, TaskColumnUpdatedAt, TaskColumnCompletedAt,
}

// CustomColumn is the export column of a custom field.
func CustomColumn(key string) string { return TaskColumnCustomPrefix + key }

// CustomColumnKey is the custom field a column names, if it names one.
func CustomColumnKey(column string) (string, bool) {
	key, ok := strings.CutPrefix(column, TaskColumnCustomPrefix)
	return key, ok && key != ""
}

// Task export date styles.
const (
	// TaskDatesISO writes RFC 3339 in UTC, which every importer reads.
	TaskDatesISO = "iso"
	// TaskDatesDate writes the calendar day alone, YYYY-MM-DD in UTC.
	TaskDatesDate = "date"
	// TaskDatesDisplay writes the reader's own time format and zone, for a
	// spreadsheet a person reads rather than a program.
	TaskDatesDisplay = "display"
)

// TaskDateStyles lists the date styles an export accepts.
var TaskDateStyles = []string{TaskDatesISO, TaskDatesDate, TaskDatesDisplay}

// TaskDelimiters names the CSV separators an export accepts.
var TaskDelimiters = map[string]rune{",": ',', ";": ';', "tab": '\t', "|": '|'}

// TaskExportInput selects a project's tasks and the shape of the file they
// are written as.
type TaskExportInput struct {
	// ProjectRef is the key or identifier of the project to export. Required.
	ProjectRef string `json:"project_ref" yaml:"project_ref"`
	// Format is csv (the default), json, ndjson or markdown.
	Format TaskFormat `json:"format,omitempty" yaml:"format,omitempty"`
	// Query is a filter in the task list's query language, such as
	// "tag:infra priority:high"; empty selects every task of the project.
	Query string `json:"query,omitempty" yaml:"query,omitempty"`
	// Columns names the columns and their order: ref, id, project, title,
	// body, status, priority, assignee, creator, tags, due, parent,
	// depends_on, recurrence, created_at, updated_at, started_at,
	// completed_at, deleted_at, comments, history, or custom:KEY. Empty
	// writes the default columns and every custom field.
	Columns []string `json:"columns,omitempty" yaml:"columns,omitempty"`
	// ExcludeFinished leaves out tasks in a terminal state.
	ExcludeFinished bool `json:"exclude_finished,omitempty" yaml:"exclude_finished,omitempty"`
	// IncludeTrashed adds the tasks in the trash.
	IncludeTrashed bool `json:"include_trashed,omitempty" yaml:"include_trashed,omitempty"`
	// ExcludeSubtasks keeps top-level tasks alone.
	ExcludeSubtasks bool `json:"exclude_subtasks,omitempty" yaml:"exclude_subtasks,omitempty"`
	// Dates is iso (RFC 3339 in UTC, the default), date (YYYY-MM-DD) or
	// display (TimeFormat in Timezone, for a spreadsheet).
	Dates string `json:"dates,omitempty" yaml:"dates,omitempty"`
	// TimeFormat is the named time format display dates use: iso, rfc3339,
	// short or us.
	TimeFormat string `json:"time_format,omitempty" yaml:"time_format,omitempty"`
	// Timezone is the IANA zone display dates are written in; empty is UTC.
	Timezone string `json:"timezone,omitempty" yaml:"timezone,omitempty"`
	// Delimiter separates CSV cells: "," (the default), ";", "tab" or "|".
	Delimiter string `json:"delimiter,omitempty" yaml:"delimiter,omitempty"`
}

// Normalize fills the defaults an empty input leaves to the export.
func (in TaskExportInput) Normalize() TaskExportInput {
	if in.Format == "" {
		in.Format = TaskFormatCSV
	}
	if in.Dates == "" {
		in.Dates = TaskDatesISO
	}
	if in.Delimiter == "" {
		in.Delimiter = ","
	}
	return in
}

// Validate checks the input.
func (in TaskExportInput) Validate() error {
	in = in.Normalize()
	if strings.TrimSpace(in.ProjectRef) == "" {
		return Invalid("a project to export is required").WithDetail(DetailField, "project_ref")
	}
	if !in.Format.Valid() {
		return Invalid("export format %q is not one of %s", in.Format, joinFormats()).WithDetail(DetailField, "format")
	}
	if !slices.Contains(TaskDateStyles, in.Dates) {
		return Invalid("date style %q is not one of %s", in.Dates, strings.Join(TaskDateStyles, ", ")).
			WithDetail(DetailField, "dates")
	}
	if _, ok := TaskDelimiters[in.Delimiter]; !ok {
		return Invalid("delimiter %q is not one of , ; tab |", in.Delimiter).WithDetail(DetailField, "delimiter")
	}
	seen := map[string]bool{}
	for _, c := range in.Columns {
		if seen[c] {
			return Invalid("column %q is named twice", c).WithDetail(DetailField, "columns")
		}
		seen[c] = true
		if _, custom := CustomColumnKey(c); custom || slices.Contains(TaskColumns, c) {
			continue
		}
		return Invalid("column %q is not one of %s, or custom:KEY", c, strings.Join(TaskColumns, ", ")).
			WithDetail(DetailField, "columns")
	}
	return nil
}

func joinFormats() string {
	names := make([]string, len(TaskFormats))
	for i, f := range TaskFormats {
		names[i] = string(f)
	}
	return strings.Join(names, ", ")
}
