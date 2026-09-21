package cmd

import (
	"fmt"
	"os/signal"
	"slices"
	"syscall"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/spf13/cobra"
)

// newWatchCmd builds the event stream command.
func newWatchCmd(g *globals) *cobra.Command {
	var projects, types, actors []string
	var since int64
	var limit int
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Follow the event stream",
		Long: "Stream domain events as they are committed, until the command is interrupted.\n" +
			"With no --since the stream starts at the next event; --since resumes after a sequence number.\n" +
			"The default format prints one readable line per event as it arrives: when, who, what happened, " +
			"to which task. Pass -o ndjson for a machine to parse instead.\n\n" +
			fmt.Sprintf("Exit codes: %d invalid filter, %d permission denied.",
				core.KindInvalid.ExitCode(), core.KindForbidden.ExitCode()),
		Example: "  tix watch\n" +
			"  tix watch --actor agent-pax --type task.*\n" +
			"  tix watch --type task.* --since 42 -o ndjson\n" +
			"  tix watch --limit 1 -o json",
		GroupID: "work",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if since < 0 {
				return usagef(cmd, "--since must not be negative")
			}
			if limit < 0 {
				return usagef(cmd, "--limit must not be negative")
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			events, err := conn.Service.Subscribe(ctx, core.EventFilter{
				ProjectIDs: projects, Types: toEventTypes(types), SinceSeq: since,
			})
			if err != nil {
				return err
			}
			return g.follow(cmd, filterByActor(events, actors), limit)
		},
	}
	f := cmd.Flags()
	f.StringSliceVar(&projects, "project", nil, "restrict to project identifiers, repeatable")
	f.StringSliceVar(&types, "type", nil, "restrict to event types, repeatable; a trailing * matches a prefix")
	f.StringSliceVar(&actors, "actor", nil, "restrict to actor identifiers, repeatable")
	f.Int64Var(&since, "since", 0, "resume after this event sequence number, zero to start from the next event")
	f.IntVar(&limit, "limit", 0, "stop after this many events, zero to follow until interrupted")
	return cmd
}

// filterByActor keeps only events whose actor is one of actors, forwarding
// everything unchanged when none were requested. core.EventFilter has no
// actor field, so this filters the stream after the service has already
// applied the project and type filters, rather than asking the server to.
func filterByActor(events <-chan core.Event, actors []string) <-chan core.Event {
	if len(actors) == 0 {
		return events
	}
	out := make(chan core.Event)
	go func() {
		defer close(out)
		for event := range events {
			if slices.Contains(actors, event.ActorID) {
				out <- event
			}
		}
	}()
	return out
}

// follow renders events until the stream ends or limit events were rendered.
// The table format, tix's default, cannot size its columns until the stream
// ends, which a live tail never does, so it renders one readable line per
// event instead; every other format keeps streaming through newList as before.
func (g *globals) follow(cmd *cobra.Command, events <-chan core.Event, limit int) error {
	if g.formatName() == output.FormatTable {
		return g.followHuman(cmd, events, limit)
	}
	out := newList[core.Event](g, cmd)
	seen := 0
	for event := range events {
		if err := out.Write(event); err != nil {
			return out.fail(err)
		}
		seen++
		if limit > 0 && seen >= limit {
			break
		}
	}
	return out.Close()
}

// followHuman writes one line per event as it arrives: a short local time,
// the actor, what happened and the task it happened to, coloured the same
// way every other table cell is.
func (g *globals) followHuman(cmd *cobra.Command, events <-chan core.Event, limit int) error {
	w := cmd.OutOrStdout()
	p := output.NewPainterWithStyle(g.colorMode(), w, g.timeStyle())
	seen := 0
	for event := range events {
		if _, err := fmt.Fprintln(w, output.FormatEventLine(p, event)); err != nil {
			return core.Internal("writing event line").Wrap(err)
		}
		seen++
		if limit > 0 && seen >= limit {
			break
		}
	}
	return nil
}

// toEventTypes converts flag strings to event types.
func toEventTypes(values []string) []core.EventType {
	if len(values) == 0 {
		return nil
	}
	out := make([]core.EventType, 0, len(values))
	for _, v := range values {
		out = append(out, core.EventType(v))
	}
	return out
}
