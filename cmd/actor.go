// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/heliopsy/tix/internal/core"
)

func actorLsCmd(g *globals) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List the actors of this tenant, agents included",
		Long:    "List this tenant's directory: the people and the agents work can be assigned to.\n\nExit codes: 5 permission denied.",
		Example: "  tix actor ls -o json",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			actors, next, err := conn.Service.ListActors(ctx, core.Page{Limit: limit})
			if err != nil {
				return err
			}
			if next != "" {
				g.diag(cmd, "more results available; next cursor %s", next)
			}
			return g.render(cmd, actors)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", core.DefaultPageLimit, "maximum records to return")
	return cmd
}
