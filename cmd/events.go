// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"fmt"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/spf13/cobra"
)

func init() { builders = append(builders, newEventsCmd) }

// newEventsCmd builds the one-shot poll of the event log.
func newEventsCmd(g *globals) *cobra.Command {
	var projects, types []string
	var since int64
	var limit int
	var wait string
	cmd := &cobra.Command{
		Use:   "events",
		Short: "Read the events after a cursor once, over plain HTTP",
		Long: "Ask once for the events committed after sequence --since, in order, and the cursor to pass " +
			"back next time. It is the long-poll alternative to tix watch for networks that break WebSockets, " +
			"with the same filters, scopes and cursor, so a client can switch between the two without a gap.\n\n" +
			"--since is exclusive; without it the poll starts at the newest event and only reports the " +
			"cursor. --wait holds the request until an event arrives or the wait runs out, at most 25s.\n\n" +
			"-o json and -o yaml print {events, next_seq}. The other formats print the events and report " +
			"next_seq on standard error. next_seq can be past the last event shown when the filter or " +
			"the credential left later events out; resume from it, not from the last seq you saw.\n\n" +
			fmt.Sprintf("Exit codes: %d invalid filter, wait or unavailable cursor, %d permission denied.",
				core.KindInvalid.ExitCode(), core.KindForbidden.ExitCode()),
		Example: "  tix events --since 41 -o json\n" +
			"  tix events --since 41 --type task.* --wait 25s -o ndjson",
		GroupID: "work",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			d, err := parseDuration(wait)
			if err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			page, err := conn.Service.PollEvents(ctx, core.EventPollInput{
				SinceSeq: since, Types: toEventTypes(types), ProjectIDs: projects, Limit: limit, Wait: d,
			})
			if err != nil {
				return err
			}
			switch g.formatName() {
			case output.FormatJSON, output.FormatYAML:
				return g.render(cmd, page)
			}
			if err := g.writeEvents(cmd, page.Events); err != nil {
				return err
			}
			g.diag(cmd, "next_seq %d", page.NextSeq)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringSliceVar(&projects, "project", nil, "restrict to project identifiers, repeatable")
	f.StringSliceVar(&types, "type", nil, "restrict to event types, repeatable; a trailing * matches a prefix")
	f.Int64Var(&since, "since", 0, "return events after this sequence number, zero to start at the newest")
	f.IntVar(&limit, "limit", 0, "return at most this many events, default 200, at most 1000")
	f.StringVar(&wait, "wait", "", "wait this long for an event when none is there yet, such as 25s")
	return cmd
}

// writeEvents prints a finished batch of events the way tix watch prints a stream.
func (g *globals) writeEvents(cmd *cobra.Command, events []core.Event) error {
	ch := make(chan core.Event, len(events))
	for _, e := range events {
		ch <- e
	}
	close(ch)
	return g.follow(cmd, ch, 0)
}
