// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/spf13/cobra"
)

// StdinMarker is the argument that means "read this from standard input".
const StdinMarker = "-"

// formatName returns the effective output format for this invocation.
func (g *globals) formatName() string {
	if g.format != "" {
		return g.format
	}
	if g.resolved != nil {
		if f := g.resolved.Config.Output.Format; f != "" {
			return f
		}
	}
	return output.FormatTable
}

// timeStyle resolves how an instant is shown to a person. Configuration is
// validated at load, so a bad value cannot reach here; a zero style is still
// the fallback rather than a panic, because rendering a timestamp in the wrong
// layout is a far smaller failure than refusing to print a task list.
func (g *globals) timeStyle() output.TimeStyle {
	if g.resolved == nil {
		return output.TimeStyle{}
	}
	style, err := output.NewTimeStyle(g.resolved.Config.Output.TimeFormat, g.resolved.Config.Output.Timezone)
	if err != nil {
		return output.TimeStyle{}
	}
	return style
}

// render writes one result to standard output in the selected format.
func (g *globals) render(cmd *cobra.Command, data any) error {
	return output.NewWithStyle(g.formatName(), g.colorMode(), g.timeStyle()).Format(cmd.OutOrStdout(), data)
}

// resolveActorNames looks up each distinct actor a list names. An actor that
// no longer resolves is left out, and the column falls back to the identifier
// rather than the row vanishing or claiming a name it cannot support.
func resolveActorNames(ctx context.Context, svc core.Service, tasks []core.Task) map[string]string {
	names := map[string]string{}
	for _, t := range tasks {
		for _, id := range []string{t.AssigneeActorID, t.ClaimedByActorID} {
			if id == "" {
				continue
			}
			if _, done := names[id]; done {
				continue
			}
			names[id] = ""
			actor, err := svc.GetActor(ctx, id)
			if err != nil || actor.Handle == "" {
				continue
			}
			names[id] = actor.Handle
		}
	}
	return names
}

// stream returns a writer that emits records as they are produced.
func (g *globals) stream(cmd *cobra.Command) output.Stream {
	return output.NewStreamWithStyle(g.formatName(), cmd.OutOrStdout(), g.colorMode(), g.timeStyle())
}

// diag writes a diagnostic to standard error unless quiet was requested.
func (g *globals) diag(cmd *cobra.Command, format string, args ...any) {
	if g.quiet {
		return
	}
	errw := cmd.ErrOrStderr()
	line := output.NewPainter(g.colorMode(), errw).Muted(fmt.Sprintf(format, args...))
	_, _ = fmt.Fprintln(errw, line)
}

// exactArgs accepts precisely n positional arguments.
func exactArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != n {
			return usagef(cmd, "%s takes %d argument(s), got %d", cmd.CommandPath(), n, len(args))
		}
		return nil
	}
}

// minArgs accepts n or more positional arguments.
func minArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < n {
			return usagef(cmd, "%s takes at least %d argument(s), got %d", cmd.CommandPath(), n, len(args))
		}
		return nil
	}
}

// rangeArgs accepts between min and max positional arguments.
func rangeArgs(minN, maxN int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < minN || len(args) > maxN {
			return usagef(cmd, "%s takes between %d and %d arguments, got %d",
				cmd.CommandPath(), minN, maxN, len(args))
		}
		return nil
	}
}

// noArgs rejects any positional argument.
func noArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return usagef(cmd, "%s takes no arguments", cmd.CommandPath())
	}
	return nil
}

// readAll reads every byte of r.
func readAll(r io.Reader) (string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return "", core.Internal("reading standard input").Wrap(err)
	}
	return string(b), nil
}

// body returns value, or the contents of standard input when value is "-".
func body(cmd *cobra.Command, value string) (string, error) {
	if value != StdinMarker {
		return value, nil
	}
	return readAll(cmd.InOrStdin())
}

// readRefs parses positional task references, expanding "-" from standard input.
func readRefs(cmd *cobra.Command, args []string) ([]core.TaskRef, error) {
	raw := make([]string, 0, len(args))
	for _, arg := range args {
		if arg != StdinMarker {
			raw = append(raw, arg)
			continue
		}
		lines, err := scanRefs(cmd.InOrStdin())
		if err != nil {
			return nil, err
		}
		raw = append(raw, lines...)
	}
	if len(raw) == 0 {
		return nil, core.Invalid("no task references were supplied")
	}
	refs := make([]core.TaskRef, 0, len(raw))
	for _, token := range raw {
		ref, err := core.ParseTaskRef(token)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

// scanRefs reads whitespace-separated references from r.
func scanRefs(r io.Reader) ([]string, error) {
	var out []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		out = append(out, strings.Fields(scanner.Text())...)
	}
	if err := scanner.Err(); err != nil {
		return nil, core.Internal("reading references from standard input").Wrap(err)
	}
	return out, nil
}

// outcome is one reference's result in a bulk operation.
type outcome struct {
	Ref    string `json:"ref" yaml:"ref"`
	Status string `json:"status" yaml:"status"`
	Code   string `json:"code,omitempty" yaml:"code,omitempty"`
	Error  string `json:"error,omitempty" yaml:"error,omitempty"`
}

// Bulk outcome statuses.
const (
	statusOK      = "ok"
	statusFailed  = "failed"
	statusPlanned = "planned"
)

// bulk applies fn to every reference, reports each outcome and fails if any did.
func (g *globals) bulk(cmd *cobra.Command, refs []core.TaskRef, fn func(core.TaskRef) error) error {
	results := make([]outcome, 0, len(refs))
	var failure error
	for _, ref := range refs {
		err := fn(ref)
		if err == nil {
			results = append(results, outcome{Ref: ref.String(), Status: statusOK})
			continue
		}
		results = append(results, outcome{
			Ref:    ref.String(),
			Status: statusFailed,
			Code:   string(core.KindOf(err)),
			Error:  err.Error(),
		})
		if failure == nil {
			failure = err
		}
	}
	if err := g.render(cmd, results); err != nil {
		return err
	}
	return failure
}

// plan describes what a dry run would have done.
type plan struct {
	Action string         `json:"action" yaml:"action"`
	Target string         `json:"target,omitempty" yaml:"target,omitempty"`
	Status string         `json:"status" yaml:"status"`
	Code   string         `json:"code,omitempty" yaml:"code,omitempty"`
	Error  string         `json:"error,omitempty" yaml:"error,omitempty"`
	Detail map[string]any `json:"detail,omitempty" yaml:"detail,omitempty"`
	DryRun bool           `json:"dry_run" yaml:"dry_run"`
}

// newPlan builds a dry-run report entry.
func newPlan(action, target string, detail map[string]any) plan {
	return plan{Action: action, Target: target, Status: statusPlanned, Detail: detail, DryRun: true}
}

// dryBulk checks every reference resolves and reports the change it would make.
func (g *globals) dryBulk(cmd *cobra.Command, conn *connect.Conn, ctx context.Context,
	refs []core.TaskRef, action string, detail map[string]any,
) error {
	plans := make([]plan, 0, len(refs))
	var failure error
	for _, ref := range refs {
		entry := newPlan(action, ref.String(), detail)
		if _, err := conn.Service.GetTask(ctx, ref); err != nil {
			entry.Status, entry.Code, entry.Error = statusFailed, string(core.KindOf(err)), err.Error()
			if failure == nil {
				failure = err
			}
		}
		plans = append(plans, entry)
	}
	if err := g.render(cmd, plans); err != nil {
		return err
	}
	return failure
}

// listWriter emits records as they are produced. Only the table format is
// buffered, because a table cannot size its columns until it has every row.
type listWriter[T any] struct {
	g      *globals
	cmd    *cobra.Command
	stream output.Stream
	rows   []T

	// names labels the actors the buffered rows refer to. It is resolved at
	// close, when every row is in hand, so one lookup covers a whole listing
	// however many rows mention the same person.
	names func([]T) map[string]string
}

// newList returns a writer for a listing of T.
func newList[T any](g *globals, cmd *cobra.Command) *listWriter[T] {
	l := &listWriter[T]{g: g, cmd: cmd}
	if g.formatName() != output.FormatTable {
		l.stream = g.stream(cmd)
	}
	return l
}

// Write emits one record.
func (l *listWriter[T]) Write(record T) error {
	if l.stream != nil {
		return l.stream.Write(record)
	}
	l.rows = append(l.rows, record)
	return nil
}

// Close finishes the listing.
func (l *listWriter[T]) Close() error {
	if l.stream != nil {
		return l.stream.Close()
	}
	if l.names != nil {
		return output.NewWithNames(l.g.formatName(), l.g.colorMode(), l.g.timeStyle(), l.names(l.rows)).
			Format(l.cmd.OutOrStdout(), l.rows)
	}
	return l.g.render(l.cmd, l.rows)
}

// fail closes the listing and returns the original error.
func (l *listWriter[T]) fail(err error) error {
	_ = l.Close()
	return err
}
