// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/spf13/cobra"
)

// taskExportDelimiters are the separators --delimiter accepts, in help order.
var taskExportDelimiters = []string{",", ";", "tab", "|"}

// newExportTasksCmd builds the project task export command.
func newExportTasksCmd(g *globals) *cobra.Command {
	var (
		in                          core.TaskExportInput
		format, out                 string
		finished, subtasks, trashed bool
	)
	cmd := &cobra.Command{
		Use:   "tasks",
		Short: "Write one project's tasks as CSV, JSON, NDJSON or a Markdown table",
		Long: "Write one project's tasks to standard output, or to --out, as they are read, " +
			"so a project of any size streams in constant memory.\n\n" +
			"--query takes the task list's filter language. --columns names the columns and their order: " +
			strings.Join(core.TaskColumns, ", ") + ", or custom:KEY for a custom field. " +
			"Without it the file carries " + strings.Join(core.DefaultTaskColumns, ", ") +
			" and every custom field.\n\n" +
			"--dates iso writes RFC 3339 in UTC, date writes YYYY-MM-DD, and display writes the time " +
			"format and zone of your configuration, for a spreadsheet a person reads. " +
			"A CSV cell starting with =, +, -, @, a tab or a carriage return gets a leading quote, " +
			"so a spreadsheet shows it as text; tix import tasks takes the quote off again.\n\n" +
			"To look at one task, tix task show REF -o json is quicker.\n\n" +
			fmt.Sprintf("Exit codes: %d invalid filter or column, %d unknown project, %d permission denied.",
				core.KindInvalid.ExitCode(), core.KindNotFound.ExitCode(), core.KindForbidden.ExitCode()),
		Example: "  tix export tasks -p infra > infra.csv\n" +
			"  tix export tasks -p infra --format json --query 'tag:ops !is:done' --out ops.json\n" +
			"  tix export tasks -p infra --columns ref,title,status,due --dates display --delimiter ';'",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if in.ProjectRef == "" {
				fallback, err := g.projectFallback(cmd)
				if err != nil {
					return err
				}
				in.ProjectRef = fallback
			}
			in.Format = core.TaskFormat(format)
			in.ExcludeFinished = !finished
			in.ExcludeSubtasks = !subtasks
			in.IncludeTrashed = trashed
			in = in.Normalize()
			if err := in.Validate(); err != nil {
				return err
			}
			if err := localFile(cmd, "--out", out); err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if in.Dates == core.TaskDatesDisplay {
				in.TimeFormat, in.Timezone = g.displayStyle()
			}
			if out == "" {
				return conn.Service.ExportTasks(ctx, in, cmd.OutOrStdout())
			}
			return exportToFile(cmd, out, func(w io.Writer) error { return conn.Service.ExportTasks(ctx, in, w) })
		},
	}
	f := cmd.Flags()
	f.StringVarP(&in.ProjectRef, "project", "p", "", "project to export; the configured default project otherwise")
	f.StringVar(&format, "format", string(core.TaskFormatCSV), "file format: "+strings.Join(flagValues(core.TaskFormats), "|"))
	f.StringVar(&in.Query, "query", "", "filter in the task list's query language")
	f.StringSliceVar(&in.Columns, "columns", nil, "columns in order, comma separated")
	f.BoolVar(&finished, "finished", true, "include tasks in a terminal state")
	f.BoolVar(&subtasks, "subtasks", true, "include subtasks")
	f.BoolVar(&trashed, "trashed", false, "include tasks in the trash")
	f.StringVar(&in.Dates, "dates", core.TaskDatesISO, "date style: "+strings.Join(core.TaskDateStyles, "|"))
	f.StringVar(&in.Delimiter, "delimiter", ",", "csv separator: "+strings.Join(taskExportDelimiters, "|"))
	f.StringVar(&out, "out", "", "write to this file instead of standard output")
	_ = cmd.RegisterFlagCompletionFunc("format", fixedCompletion(flagValues(core.TaskFormats)))
	_ = cmd.RegisterFlagCompletionFunc("dates", fixedCompletion(core.TaskDateStyles))
	_ = cmd.RegisterFlagCompletionFunc("delimiter", fixedCompletion(taskExportDelimiters))
	_ = cmd.RegisterFlagCompletionFunc("columns", fixedCompletion(core.TaskColumns))
	registerCompletions(g, cmd, "project")
	return cmd
}

// exportToFile writes an export to path, removing the file again when the
// export fails, so a refused export never leaves half a file behind.
func exportToFile(cmd *cobra.Command, path string, write func(io.Writer) error) error {
	if err := localFile(cmd, "--out", path); err != nil {
		return err
	}
	// #nosec G304 -- the path is the operator's own --out argument.
	file, err := os.Create(path)
	if err != nil {
		return core.Invalid("creating %q: %v", path, err)
	}
	if err := write(file); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Close(); err != nil {
		return core.Internal("writing %q: %v", path, err)
	}
	return nil
}

// displayStyle is the configured time format and zone an export's display
// dates are written in. A relative time cannot be read back, so it is
// written in the ISO style instead.
func (g *globals) displayStyle() (string, string) {
	format := output.TimeISO
	if g.resolved != nil {
		if f := g.resolved.Config.Output.TimeFormat; f != "" && f != output.TimeRelative {
			format = f
		}
	}
	return format, g.timeStyle().Location().String()
}
