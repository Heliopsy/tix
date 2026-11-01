// SPDX-License-Identifier: AGPL-3.0-or-later

package output

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// FormatCSV renders a task listing as comma separated values. It is not in
// Formats: only a task listing has a column set, so only `tix task ls`
// accepts it.
const FormatCSV = "csv"

// TaskColumn is one column of a CSV task listing: its header and how a task
// fills its cell. Names labels actors where the listing resolved them.
type TaskColumn struct {
	Header string
	Cell   func(t core.Task, names map[string]string) string
}

// Task columns, named once so the CLI, the API and the browser export spell a
// column the same way.
var (
	ColumnRef      = TaskColumn{"ref", func(t core.Task, _ map[string]string) string { return t.Ref }}
	ColumnProject  = TaskColumn{"project", func(t core.Task, _ map[string]string) string { return ProjectKeyOf(t.Ref) }}
	ColumnTitle    = TaskColumn{"title", func(t core.Task, _ map[string]string) string { return t.Title }}
	ColumnStatus   = TaskColumn{"status", func(t core.Task, _ map[string]string) string { return t.Status }}
	ColumnPriority = TaskColumn{"priority", func(t core.Task, _ map[string]string) string { return priorityLabel(t.Priority) }}
	ColumnAssignee = TaskColumn{"assignee", func(t core.Task, names map[string]string) string {
		if name := names[t.AssigneeActorID]; name != "" {
			return name
		}
		return t.AssigneeActorID
	}}
	ColumnTags    = TaskColumn{"tags", func(t core.Task, _ map[string]string) string { return strings.Join(t.Tags, " ") }}
	ColumnDue     = TaskColumn{"due", func(t core.Task, _ map[string]string) string { return FormatTimestampPtr(t.DueAt) }}
	ColumnCreated = TaskColumn{"created", func(t core.Task, _ map[string]string) string { return FormatTimestamp(t.CreatedAt) }}
	ColumnUpdated = TaskColumn{"updated", func(t core.Task, _ map[string]string) string { return FormatTimestamp(t.UpdatedAt) }}
	ColumnStarred = TaskColumn{"starred", func(t core.Task, _ map[string]string) string { return yesNo(t.Starred) }}
	ColumnRepeat  = TaskColumn{"repeat", func(t core.Task, _ map[string]string) string { return t.Recurrence }}
)

// TaskCSVColumns is the fixed column set of the command line and the API.
var TaskCSVColumns = []TaskColumn{
	ColumnRef, ColumnProject, ColumnTitle, ColumnStatus, ColumnPriority,
	ColumnAssignee, ColumnTags, ColumnDue, ColumnCreated, ColumnUpdated,
}

// ProjectKeyOf is the project key a task reference starts with.
func ProjectKeyOf(ref string) string {
	if i := strings.LastIndex(ref, "-"); i > 0 {
		return ref[:i]
	}
	return ""
}

// GuardCell defuses a cell a spreadsheet would read as a formula. A leading
// =, +, - or @ starts one, and a tab or carriage return can hide one; a quote
// in front makes the cell text without changing what a reader sees.
func GuardCell(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}

// TaskCSV writes tasks as CSV under the given columns, header first.
type TaskCSV struct {
	w       *csv.Writer
	columns []TaskColumn
	names   map[string]string
	started bool
}

// NewTaskCSV returns a writer of task rows.
func NewTaskCSV(w io.Writer, columns []TaskColumn, names map[string]string) *TaskCSV {
	return &TaskCSV{w: csv.NewWriter(w), columns: columns, names: names}
}

// Header writes the header row, once.
func (c *TaskCSV) Header() error {
	if c.started {
		return nil
	}
	c.started = true
	row := make([]string, len(c.columns))
	for i, col := range c.columns {
		row[i] = col.Header
	}
	return c.w.Write(row)
}

// Write writes one task, after the header.
func (c *TaskCSV) Write(t core.Task) error {
	if err := c.Header(); err != nil {
		return err
	}
	row := make([]string, len(c.columns))
	for i, col := range c.columns {
		row[i] = GuardCell(col.Cell(t, c.names))
	}
	return c.w.Write(row)
}

// Close writes the header if no row did and flushes.
func (c *TaskCSV) Close() error {
	if err := c.Header(); err != nil {
		return err
	}
	c.w.Flush()
	if err := c.w.Error(); err != nil {
		return fmt.Errorf("writing csv: %w", err)
	}
	return nil
}

// csvFormatter renders a buffered task listing.
type csvFormatter struct {
	names map[string]string
}

func (f *csvFormatter) Format(w io.Writer, data any) error {
	var tasks []core.Task
	switch v := data.(type) {
	case []core.Task:
		tasks = v
	case core.TaskPage:
		tasks = v.Tasks
	case core.Task:
		tasks = []core.Task{v}
	default:
		return core.Invalid("csv output is only available for task listings")
	}
	out := NewTaskCSV(w, TaskCSVColumns, f.names)
	for _, t := range tasks {
		if err := out.Write(t); err != nil {
			return err
		}
	}
	return out.Close()
}
