// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"slices"
	"strings"
	"time"
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

// TaskFormatLines is the import format beyond the export ones: a list of
// quick-add lines, one task per line, which needs no mapping at all.
const TaskFormatLines TaskFormat = "lines"

// TaskImportFormats lists the formats an import reads. Empty detects CSV, JSON
// or NDJSON from the file itself.
var TaskImportFormats = []TaskFormat{TaskFormatCSV, TaskFormatJSON, TaskFormatNDJSON, TaskFormatLines}

// TaskImportMode says what an import does with a row that matches a task the
// project already has, by external id or reference.
type TaskImportMode string

// Task import modes.
const (
	// TaskImportCreate creates every row as a new task, matched or not.
	TaskImportCreate TaskImportMode = "create"
	// TaskImportUpsert updates a matched task and creates the rest.
	TaskImportUpsert TaskImportMode = "upsert"
	// TaskImportSkip leaves a matched task alone and creates the rest.
	TaskImportSkip TaskImportMode = "skip"
)

// TaskImportModes lists the modes in the order a chooser offers them.
var TaskImportModes = []TaskImportMode{TaskImportUpsert, TaskImportCreate, TaskImportSkip}

// Valid reports whether m is a known mode.
func (m TaskImportMode) Valid() bool { return slices.Contains(TaskImportModes, m) }

// MaxImportPreview bounds the rows a dry run shows as tix would create them.
const MaxImportPreview = 200

// TaskImportInput says how to read a file of tasks into a project.
type TaskImportInput struct {
	// ProjectRef is the key or identifier of the project to import into.
	// Required.
	ProjectRef string `json:"project_ref" yaml:"project_ref"`
	// Format is csv, json, ndjson or lines; empty detects it from the file.
	// lines reads one task per line in quick-add syntax, such as
	// "Fix login due:fri #bug !high", and needs no mapping.
	Format TaskFormat `json:"format,omitempty" yaml:"format,omitempty"`
	// Filename is the uploaded file's name, which helps tell its format.
	Filename string `json:"filename,omitempty" yaml:"filename,omitempty"`
	// Content is the file itself, for a caller that sends it inside a JSON
	// body rather than as the upload; the upload wins when both are given.
	Content string `json:"content,omitempty" yaml:"content,omitempty"`
	// Mapping is a mapping file, YAML or JSON: which column fills which
	// field, how values translate, and what to do with unknown ones. Empty
	// uses the suggested mapping, which reads a tix export and the common
	// headers of Jira, GitHub, Trello and Linear as they are.
	Mapping string `json:"mapping,omitempty" yaml:"mapping,omitempty"`
	// Mode overrides the mapping's mode: upsert (the default), create or
	// skip.
	Mode TaskImportMode `json:"mode,omitempty" yaml:"mode,omitempty"`
	// DryRun reads and checks the whole file and reports what would happen,
	// writing nothing.
	DryRun bool `json:"dry_run,omitempty" yaml:"dry_run,omitempty"`
	// Preview is how many rows a dry run shows as tix would create them,
	// 20 when zero, at most 200.
	Preview int `json:"preview,omitempty" yaml:"preview,omitempty"`
}

// Validate checks the input.
func (in TaskImportInput) Validate() error {
	if strings.TrimSpace(in.ProjectRef) == "" {
		return Invalid("a project to import into is required").WithDetail(DetailField, "project_ref")
	}
	if in.Format != "" && !slices.Contains(TaskImportFormats, in.Format) {
		return Invalid("import format %q is not one of csv, json, ndjson, lines", in.Format).
			WithDetail(DetailField, "format")
	}
	if in.Mode != "" && !in.Mode.Valid() {
		return Invalid("import mode %q is not one of upsert, create, skip", in.Mode).WithDetail(DetailField, "mode")
	}
	if in.Preview < 0 || in.Preview > MaxImportPreview {
		return Invalid("preview must be between 0 and %d rows", MaxImportPreview).WithDetail(DetailField, "preview")
	}
	return nil
}

// Task import row actions.
const (
	ImportRowCreate = "create"
	ImportRowUpdate = "update"
	ImportRowSkip   = "skip"
	ImportRowError  = "error"
)

// TaskImportColumn is one column of the file and the field it fills.
type TaskImportColumn struct {
	// Header is the column as the file names it.
	Header string `json:"header" yaml:"header"`
	// Target is the field it fills: title, body, status, priority,
	// assignee, tags, due, parent, depends_on, recurrence, external_id,
	// alias, ref, custom:KEY, append_body or ignore.
	Target string `json:"target" yaml:"target"`
	// Suggested is true when the mapping did not name the column and the
	// target is tix's own guess.
	Suggested bool `json:"suggested" yaml:"suggested"`
	// Sample is the column's first non-empty values, up to three.
	Sample []string `json:"sample,omitempty" yaml:"sample,omitempty"`
}

// TaskImportValue is one distinct source value of a mapped field and the
// tix value it becomes.
type TaskImportValue struct {
	// Value is the value as the file spells it.
	Value string `json:"value" yaml:"value"`
	// Count is how many rows carry it.
	Count int `json:"count" yaml:"count"`
	// Mapped is the tix value it becomes, empty when it has none and the
	// unknown-value policy decides.
	Mapped string `json:"mapped,omitempty" yaml:"mapped,omitempty"`
	// Suggested is true when Mapped is tix's own guess rather than the
	// mapping's.
	Suggested bool `json:"suggested,omitempty" yaml:"suggested,omitempty"`
}

// TaskImportRow is one row as the import would write it.
type TaskImportRow struct {
	// Line is the row's line or record number in the file.
	Line int `json:"line" yaml:"line"`
	// Action is create, update, skip or error.
	Action string `json:"action" yaml:"action"`
	// Ref is the task's reference: the one it has when updated, the one it
	// would get or got when created.
	Ref string `json:"ref,omitempty" yaml:"ref,omitempty"`
	// ExternalID is the row's identifier in the file.
	ExternalID string   `json:"external_id,omitempty" yaml:"external_id,omitempty"`
	Title      string   `json:"title,omitempty" yaml:"title,omitempty"`
	Status     string   `json:"status,omitempty" yaml:"status,omitempty"`
	Priority   Priority `json:"priority,omitempty" yaml:"priority,omitempty"`
	Assignee   string   `json:"assignee,omitempty" yaml:"assignee,omitempty"`
	Tags       []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	// Due is the due date the row sets.
	Due    *time.Time `json:"due,omitempty" yaml:"due,omitempty"`
	Parent string     `json:"parent,omitempty" yaml:"parent,omitempty"`
	// Reason says why a row is skipped.
	Reason string `json:"reason,omitempty" yaml:"reason,omitempty"`
	// Errors says why a row cannot be imported.
	Errors []string `json:"errors,omitempty" yaml:"errors,omitempty"`
}

// TaskImportError is one row's refusal, the shape of the error report.
type TaskImportError struct {
	Line    int    `json:"line" yaml:"line"`
	Message string `json:"message" yaml:"message"`
}

// MaxImportErrors bounds the refusals one result lists.
const MaxImportErrors = 1000

// TaskImportResult reports an import, or what a dry run would have done.
type TaskImportResult struct {
	// DryRun is true when nothing was written.
	DryRun bool `json:"dry_run" yaml:"dry_run"`
	// Committed is true when the tasks were written. An import with any row
	// in error writes nothing.
	Committed bool `json:"committed" yaml:"committed"`
	// Format is the format the file was read as.
	Format TaskFormat `json:"format" yaml:"format"`
	// Source is the tool the file looks like it came from: jira, github,
	// trello, linear, tix, or empty.
	Source string `json:"source,omitempty" yaml:"source,omitempty"`
	// Mode is the mode the import ran in.
	Mode TaskImportMode `json:"mode" yaml:"mode"`
	// Rows is how many rows the file holds.
	Rows    int `json:"rows" yaml:"rows"`
	Created int `json:"created" yaml:"created"`
	Updated int `json:"updated" yaml:"updated"`
	Skipped int `json:"skipped" yaml:"skipped"`
	Failed  int `json:"failed" yaml:"failed"`
	// Columns is every column of the file and the field it fills.
	Columns []TaskImportColumn `json:"columns,omitempty" yaml:"columns,omitempty"`
	// Confident is true when every column was named by the mapping or
	// recognised outright, so the suggested mapping can be used as it is.
	Confident bool `json:"confident" yaml:"confident"`
	// Values lists, for status, priority and assignee, the distinct values
	// of the file and what each becomes.
	Values map[string][]TaskImportValue `json:"values,omitempty" yaml:"values,omitempty"`
	// Preview is the first rows as tix would write them.
	Preview []TaskImportRow `json:"preview,omitempty" yaml:"preview,omitempty"`
	// Errors lists every row that cannot be imported, up to 1000.
	Errors []TaskImportError `json:"errors,omitempty" yaml:"errors,omitempty"`
	// Mapping is the mapping in force, as a mapping file, to save and pass
	// back next time.
	Mapping string `json:"mapping" yaml:"mapping"`
}
