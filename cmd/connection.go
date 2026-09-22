package cmd

import (
	"github.com/spf13/cobra"
)

// newConnectionCmd builds the live connection command group.
func newConnectionCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "connection",
		Aliases: []string{"conn"},
		Short:   "See and end the connections a server holds",
		Long: "Inspect the live connections one server is holding, on the event stream and over SSH.\n\n" +
			"The view is one process's. A connection lives in exactly one server, so a deployment " +
			"running several against one database gets a partial view from each, and every answer " +
			"names the server it came from.",
		GroupID: "admin",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(connectionLsCmd(g), connectionKillCmd(g))
	return cmd
}

func connectionLsCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List the live connections of your tenant",
		Long: "List what this server is holding for your tenant, with the counts that go with it.\n\n" +
			"The per-surface counts are your tenant's. The process total covers every connection the " +
			"server holds and says nothing about who holds them.\n\nExit codes: 5 permission denied.",
		Example: "  tix connection ls\n  tix connection ls -o json",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			list, err := conn.Service.ListConnections(ctx)
			if err != nil {
				return err
			}
			g.diag(cmd, "answered by %s; this view covers this server only", list.ServerID)
			return g.render(cmd, list)
		},
	}
}

func connectionKillCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "kill ID",
		Aliases: []string{"end"},
		Short:   "End one live connection",
		Long: "Close one live connection of your tenant by identifier.\n\n" +
			"This is not revocation: the holder may reconnect at once with credentials that are still " +
			"valid. Revoke the token or the key to stop that. An identifier this server does not hold " +
			"is reported as not found.\n\nExit codes: 3 no such live connection, 5 permission denied.",
		Example: "  tix connection kill 01JB2K3M4N5P6Q7R8S9T",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if err := conn.Service.EndConnection(ctx, args[0]); err != nil {
				return err
			}
			g.diag(cmd, "ending is not revocation; the holder may reconnect")
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
	}
}
