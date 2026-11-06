// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/spf13/cobra"
)

// newImportTasksCmd builds the project task import command.
func newImportTasksCmd(g *globals) *cobra.Command {
	var (
		in                         core.TaskImportInput
		format, mode, mappingPath  string
		linesPath, saveMappingPath string
	)
	cmd := &cobra.Command{
		Use:   "tasks [FILE]",
		Short: "Read a file of tasks into one project",
		Long: "Read a CSV, JSON or NDJSON file of tasks into one project, all or nothing. FILE is a path, " +
			"or - or nothing for standard input.\n\n" +
			"For a handful of tasks, tix task add once per task is simpler, and --lines reads a plain list: " +
			"one task per line in quick-add syntax, such as 'Fix login due:fri #bug !high', with no mapping.\n\n" +
			"No mapping file is needed for a tix export or for the usual headers of Jira, GitHub, Trello and " +
			"Linear: tix suggests which column fills which field and how values such as statuses translate. " +
			"Run with --dry-run first to see the suggestion, the first rows as they would be written and every " +
			"row in error; --save-mapping writes the mapping in force to a file to edit and pass back with " +
			"--mapping.\n\n" +
			"A row is matched to a task imported before by its external id, or by a tix reference. --mode upsert " +
			"(the default) updates it, skip leaves it alone and create makes a new task anyway. Parent and " +
			"dependency columns may name another row of the file or an existing task.\n\n" +
			"If any row is in error nothing is written, the rows in error are listed, and the exit code is " +
			fmt.Sprintf("%d. ", core.KindInvalid.ExitCode()) +
			fmt.Sprintf("Exit codes: %d invalid file, mapping or row, %d unknown project, %d permission denied.",
				core.KindInvalid.ExitCode(), core.KindNotFound.ExitCode(), core.KindForbidden.ExitCode()),
		Example: "  tix import tasks -p infra jira.csv --dry-run\n" +
			"  tix import tasks -p infra jira.csv --dry-run --save-mapping jira.yaml\n" +
			"  tix import tasks -p infra jira.csv --mapping jira.yaml\n" +
			"  tix export tasks -p infra | tix import tasks -p ops\n" +
			"  printf 'Fix login due:fri #bug\\nWrite release notes !high\\n' | tix import tasks -p infra --lines -",
		Args: rangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if in.ProjectRef == "" {
				fallback, err := g.projectFallback(cmd)
				if err != nil {
					return err
				}
				in.ProjectRef = fallback
			}
			in.Format, in.Mode = core.TaskFormat(format), core.TaskImportMode(mode)
			path := StdinMarker
			if len(args) == 1 {
				path = args[0]
			}
			if cmd.Flags().Changed("lines") {
				if len(args) == 1 {
					return usagef(cmd, "--lines names the list; pass no FILE beside it")
				}
				in.Format, path = core.TaskFormatLines, linesPath
			}
			if mappingPath != "" {
				// #nosec G304 -- the path is the operator's own --mapping argument.
				raw, err := os.ReadFile(mappingPath)
				if err != nil {
					return core.Invalid("reading mapping %q: %v", mappingPath, err)
				}
				in.Mapping = string(raw)
			}
			if err := in.Validate(); err != nil {
				return err
			}
			r, closeFn, err := openImportSource(cmd, path)
			if err != nil {
				return err
			}
			defer closeFn()
			if path != StdinMarker {
				in.Filename = filepath.Base(path)
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			result, err := conn.Service.ImportTasks(ctx, r, in)
			if err != nil {
				return err
			}
			if saveMappingPath != "" && result.Mapping != "" {
				if err := os.WriteFile(saveMappingPath, []byte(result.Mapping), 0o600); err != nil {
					return core.Internal("writing %q: %v", saveMappingPath, err)
				}
			}
			if err := g.renderImport(cmd, result); err != nil {
				return err
			}
			if result.Failed > 0 {
				return core.Invalid("%d row(s) in error; nothing was imported", result.Failed)
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVarP(&in.ProjectRef, "project", "p", "", "project to import into; the configured default project otherwise")
	f.StringVar(&format, "format", "", "file format: csv|json|ndjson; detected when empty")
	f.StringVar(&mappingPath, "mapping", "", "mapping file, YAML or JSON; the suggestion when empty")
	f.StringVar(&mode, "mode", "", "a row imported before: upsert (default) updates it, skip leaves it, create adds it again")
	f.BoolVar(&in.DryRun, "dry-run", false, "report what would happen and write nothing")
	f.IntVar(&in.Preview, "preview", 0, "rows a dry run shows, 20 by default, at most 200")
	f.StringVar(&linesPath, "lines", "", "read a list instead, one task per line in quick-add syntax; - for standard input")
	f.StringVar(&saveMappingPath, "save-mapping", "", "write the mapping in force to this file")
	_ = cmd.RegisterFlagCompletionFunc("format", fixedCompletion([]string{"csv", "json", "ndjson"}))
	_ = cmd.RegisterFlagCompletionFunc("mode", fixedCompletion(flagValues(core.TaskImportModes)))
	registerCompletions(g, cmd, "project")
	return cmd
}

// openImportSource opens the file an import reads, or standard input.
func openImportSource(cmd *cobra.Command, path string) (io.Reader, func(), error) {
	if path == "" || path == StdinMarker {
		return cmd.InOrStdin(), func() {}, nil
	}
	// #nosec G304 -- the path is the operator's own argument.
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, core.Invalid("opening %q: %v", path, err)
	}
	return f, func() { _ = f.Close() }, nil
}

// renderImport writes an import's report: the whole result in a machine
// format, and in a table the counts, the column mapping, the preview rows
// and the rows in error.
func (g *globals) renderImport(cmd *cobra.Command, r *core.TaskImportResult) error {
	if g.formatName() != output.FormatTable {
		return g.render(cmd, r)
	}
	w := cmd.OutOrStdout()
	verb := "imported"
	switch {
	case r.DryRun:
		verb = "dry run"
	case !r.Committed:
		verb = "nothing imported"
	}
	source := ""
	if r.Source != "" {
		source = ", looks like " + r.Source
	}
	_, _ = fmt.Fprintf(w, "%s: %d rows read as %s%s; %d create, %d update, %d skip, %d in error\n",
		verb, r.Rows, r.Format, source, r.Created, r.Updated, r.Skipped, r.Failed)
	if len(r.Columns) > 0 {
		cols := make([]importColumnLine, 0, len(r.Columns))
		for _, c := range r.Columns {
			note := ""
			if c.Suggested {
				note = "suggested"
			}
			cols = append(cols, importColumnLine{Column: c.Header, Target: c.Target, Note: note})
		}
		_, _ = fmt.Fprintln(w)
		if err := g.render(cmd, cols); err != nil {
			return err
		}
	}
	if r.DryRun && len(r.Preview) > 0 {
		rows := make([]importPreviewLine, 0, len(r.Preview))
		for _, p := range r.Preview {
			rows = append(rows, importPreviewLine{Line: p.Line, Action: p.Action, Ref: p.Ref, Title: p.Title,
				Status: p.Status, Note: strings.TrimSpace(strings.Join(append([]string{p.Reason}, p.Errors...), " "))})
		}
		_, _ = fmt.Fprintln(w)
		if err := g.render(cmd, rows); err != nil {
			return err
		}
	}
	if len(r.Errors) > 0 {
		_, _ = fmt.Fprintln(w)
		for _, e := range r.Errors {
			_, _ = fmt.Fprintf(w, "line %d: %s\n", e.Line, e.Message)
		}
	}
	return nil
}

// importColumnLine is one column of an import's table report.
type importColumnLine struct {
	Column string `json:"column" yaml:"column"`
	Target string `json:"target" yaml:"target"`
	Note   string `json:"note,omitempty" yaml:"note,omitempty"`
}

// importPreviewLine is one preview row of an import's table report.
type importPreviewLine struct {
	Line   int    `json:"line" yaml:"line"`
	Action string `json:"action" yaml:"action"`
	Ref    string `json:"ref,omitempty" yaml:"ref,omitempty" show:"ref"`
	Title  string `json:"title" yaml:"title"`
	Status string `json:"status,omitempty" yaml:"status,omitempty"`
	Note   string `json:"note,omitempty" yaml:"note,omitempty"`
}
