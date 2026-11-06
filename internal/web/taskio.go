// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"slices"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/wire"
)

// exportDialogID is the id of a project's Export dialog.
const exportDialogID = "export"

// taskIORoutes are a project's task export and import.
func (h *handler) taskIORoutes() []route {
	return []route{
		post(RouteProjectTaskExport, h.exportProjectTasks, "ExportTasks"),
	}
}

// exportColumn is one column the Export dialog offers.
type exportColumn struct {
	Key     string
	Label   string
	Checked bool
}

// exportForm is what a project's Export dialog draws.
type exportForm struct {
	Formats    []core.TaskFormat
	Columns    []exportColumn
	Delimiters []string
}

// exportDialog is the "export-dialog" partial's argument: the page it is
// drawn on and the form.
type exportDialog struct {
	Page any
	Form *exportForm
}

// newExportDialog builds the "export-dialog" partial's argument.
func newExportDialog(page any, form *exportForm) exportDialog {
	return exportDialog{Page: page, Form: form}
}

// formatLabels name the export formats in the dialog.
var formatLabels = map[core.TaskFormat]string{
	core.TaskFormatCSV: "CSV, for spreadsheets", core.TaskFormatJSON: "JSON",
	core.TaskFormatNDJSON: "NDJSON, one task per line", core.TaskFormatMarkdown: "Markdown table",
}

// exportFormatLabel names an export format.
func exportFormatLabel(f core.TaskFormat) string { return formatLabels[f] }

// delimiterLabels name the CSV separators in the dialog.
var delimiterLabels = map[string]string{",": "Comma", ";": "Semicolon", "tab": "Tab", "|": "Vertical bar"}

// delimiterLabel names a CSV separator.
func delimiterLabel(d string) string { return delimiterLabels[d] }

// exportColumnLabels are the words the dialog names a built-in column with.
var exportColumnLabels = map[string]string{
	core.TaskColumnRef: "Reference", core.TaskColumnID: "Identifier", core.TaskColumnProject: "Project",
	core.TaskColumnTitle: "Title", core.TaskColumnBody: "Description", core.TaskColumnStatus: "Status",
	core.TaskColumnPriority: "Priority", core.TaskColumnAssignee: "Assignee", core.TaskColumnCreator: "Creator",
	core.TaskColumnTags: "Tags", core.TaskColumnDue: "Due", core.TaskColumnParent: "Parent",
	core.TaskColumnDependsOn: "Depends on", core.TaskColumnRecurrence: "Repeat",
	core.TaskColumnCreatedAt: "Created", core.TaskColumnUpdatedAt: "Updated",
	core.TaskColumnStartedAt: "Started", core.TaskColumnCompletedAt: "Completed",
	core.TaskColumnDeletedAt: "Deleted", core.TaskColumnComments: "Comments", core.TaskColumnHistory: "History",
}

// newExportForm lists every column the project's tasks can be written with,
// the defaults ticked, custom fields after the built-in ones.
func newExportForm(defs []core.FieldDef) *exportForm {
	form := &exportForm{Formats: core.TaskFormats, Delimiters: []string{",", ";", "tab", "|"}}
	for _, c := range core.TaskColumns {
		form.Columns = append(form.Columns, exportColumn{Key: c, Label: exportColumnLabels[c],
			Checked: slices.Contains(core.DefaultTaskColumns, c)})
	}
	for _, d := range defs {
		form.Columns = append(form.Columns, exportColumn{Key: core.CustomColumn(d.Key), Label: d.Label, Checked: true})
	}
	return form
}

// exportProjectTasks streams the project's tasks as a download. Nothing is
// sent until the first byte of the file, so a refusal found before then, a
// filter that does not parse or a column that does not exist, draws the
// project again with the dialog open and the reason beside it.
func (h *handler) exportProjectTasks(w http.ResponseWriter, r *http.Request) error {
	key := r.PathValue("key")
	in := core.TaskExportInput{
		ProjectRef:      key,
		Format:          core.TaskFormat(field(r, "format")),
		Query:           field(r, "q"),
		Columns:         r.PostForm["column"],
		ExcludeFinished: !checked(r, "finished"),
		IncludeTrashed:  checked(r, "trashed"),
		ExcludeSubtasks: !checked(r, "subtasks"),
		Dates:           field(r, "dates"),
		Delimiter:       field(r, "delimiter"),
	}.Normalize()
	if in.Dates == core.TaskDatesDisplay {
		in.TimeFormat, in.Timezone = h.exportStyle(r)
	}
	out := &lazyDownload{w: w, contentType: wire.TaskFormatContent(string(in.Format)),
		filename: wire.TaskExportFilename(key, in.Format.Extension())}
	err := h.svc.ExportTasks(r.Context(), in, out)
	if err == nil || out.started {
		return nil
	}
	return h.refuseIn(w, r, exportDialogID, RouteProjects+"/"+key, err, h.showBoard)
}

// exportStyle is the reader's own time format and zone, the way their pages
// draw instants. A relative time cannot be read back, so it is written in
// the ISO style instead.
func (h *handler) exportStyle(r *http.Request) (string, string) {
	style := h.readerStyle(r)
	format := style.Name()
	if format == output.TimeRelative {
		format = output.TimeISO
	}
	return format, style.Location().String()
}

// lazyDownload sends a download's headers with its first byte, so an error
// found before then can still be answered with a page.
type lazyDownload struct {
	w           http.ResponseWriter
	contentType string
	filename    string
	started     bool
}

func (d *lazyDownload) Write(p []byte) (int, error) {
	if !d.started {
		d.started = true
		d.w.Header().Set("Content-Type", d.contentType+"; charset=utf-8")
		d.w.Header().Set("Content-Disposition", `attachment; filename="`+d.filename+`"`)
		d.w.WriteHeader(http.StatusOK)
	}
	return d.w.Write(p)
}
