// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/spf13/cobra"
)

// newStatusCmd builds the command that reports the whole installation.
func newStatusCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Report what the installation holds and what is running",
		Long: "Report the installation, the servers registered against it and the work it holds.\n\n" +
			"A server registers itself when it starts and refreshes that registration while it " +
			"runs, so this answers how many servers are up rather than what one of them can see. " +
			"A server that stopped answering is listed and marked, not hidden: a graceful " +
			"shutdown removes its own row, so a row nobody is refreshing is a process that died.\n\n" +
			"This is not `tix doctor`. Doctor asks whether the installation is sound and fails " +
			"when a check does; this reports what exists, and a server on another machine being " +
			"down does not make it fail.\n\n" +
			"Exit codes: 5 permission denied.",
		Example: "  tix status\n  tix status -o json",
		GroupID: "admin",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			report, err := conn.Service.Status(ctx)
			if err != nil {
				return err
			}
			// The store is a property of the target this reader resolved, not
			// of the installation, so it is filled here rather than by a
			// service that has no idea how it was reached. `tix doctor`
			// reports the same string from the same place.
			report.Installation.Store = conn.Info.Target.Describe()
			if report.Installation.Engine == "" {
				report.Installation.Engine = string(conn.Info.Target.Engine)
			}
			if g.formatName() != output.FormatTable {
				return g.render(cmd, report)
			}
			return writeStatusTable(cmd.OutOrStdout(), report, g.timeStyle())
		},
	}
}

// writeStatusTable renders the report for a person.
//
// It lives here rather than in internal/output because the server block cannot
// be shown without the sentence saying which servers are attached and what an
// unknown connection count means, and a generic renderer has nowhere to put
// that.
func writeStatusTable(w io.Writer, r *core.StatusReport, style output.TimeStyle) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	inst := r.Installation

	if _, err := fmt.Fprintf(tw, "INSTALLATION\n  version\t%s\tstore\t%s\n  schema\t%d\tengine\t%s\n\n",
		inst.Version, dashIfEmpty(inst.Store), inst.SchemaVersion, dashIfEmpty(inst.Engine)); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(tw, "SERVERS (%s)\n", serverSummary(r)); err != nil {
		return err
	}
	for _, s := range r.Servers {
		if err := writeServerRows(tw, s, style); err != nil {
			return err
		}
	}
	if len(r.Servers) == 0 {
		if _, err := fmt.Fprint(tw,
			"  none registered; nothing is serving this store right now\n"); err != nil {
			return err
		}
	}

	work := r.Work
	if _, err := fmt.Fprintf(tw,
		"\nWORK\n  %s\t%s\t%s\n  %d claimed\t%d leases expired and unswept\n"+
			"  webhooks: %d pending, %d failed\n",
		plural(work.Tenants, "tenant"), plural(work.Projects, "project"), plural(work.Tasks, "task"),
		work.Claimed, work.LeasesExpiredUnswept,
		work.WebhooksPending, work.WebhooksFailed); err != nil {
		return err
	}
	return tw.Flush()
}

// writeServerRows renders one server over two lines: what it is, then what it
// serves.
func writeServerRows(w io.Writer, s core.ServerStatus, style output.TimeStyle) error {
	state := "up " + s.Uptime.String()
	seen := "last seen " + style.Format(s.LastSeenAt)
	if !s.Attached {
		state = "NOT HEARTBEATING"
	}
	if _, err := fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n",
		s.ID, dashIfEmpty(s.Address), dashIfEmpty(s.Version), state, seen); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "  \t%s\t\t%s\t\n",
		surfaceList(s.Surfaces), connectionDetail(s.Connections))
	return err
}

// serverSummary says how many servers are attached, and names the rest.
func serverSummary(r *core.StatusReport) string {
	attached := r.AttachedCount()
	stale := len(r.Servers) - attached
	out := fmt.Sprintf("%d attached", attached)
	if stale > 0 {
		out += fmt.Sprintf(", %d not heartbeating", stale)
	}
	return out
}

// surfaceList renders the surfaces a server serves.
func surfaceList(surfaces []core.ServerSurface) string {
	parts := make([]string, 0, len(surfaces))
	for _, s := range surfaces {
		parts = append(parts, string(s))
	}
	return dashIfEmpty(strings.Join(parts, " "))
}

// connectionDetail renders a connection count, saying nothing rather than zero
// for a server whose count this reader cannot know. A connection lives in one
// process's memory, so only that process can report one, and printing zero
// here would be a claim about somebody else's sockets.
func connectionDetail(n *int) string {
	if n == nil {
		return "- connections (only the server answering knows its own)"
	}
	return fmt.Sprintf("%d connections", *n)
}

// plural counts a thing in words a person would use. "1 tenants" is the sort
// of line that makes a reader wonder whether the number is real.
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// dashIfEmpty renders an absent value as a dash rather than as a blank column.
func dashIfEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func init() { builders = append(builders, newStatusCmd) }
