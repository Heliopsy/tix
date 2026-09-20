package cmd

import (
	"github.com/spf13/cobra"
	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/tui"
)

// newTUICmd builds the command that runs the terminal interface.
func newTUICmd(g *globals) *cobra.Command {
	var project, filter string
	cmd := &cobra.Command{
		Use:     "tui",
		Short:   "Browse and work on tasks in a terminal interface",
		Long:    "Open the terminal interface against the configured target.\n\nExit codes: 1 fatal error, 130 interrupted.",
		Example: "  tix tui\n  tix tui -p infra --filter \"status:todo is:unclaimed\"",
		GroupID: "work",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fallback, err := g.projectFallback(cmd)
			if err != nil {
				return err
			}
			if fallback != "" {
				project = fallback
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			code := tui.Run(tui.Options{
				Service: conn.Service,
				Actor:   conn.Actor,
				Context: ctx,
				Project: project,
				Filter:  filter,
				In:      cmd.InOrStdin(),
				Out:     cmd.OutOrStdout(),
				Err:     cmd.ErrOrStderr(),
				Environ: g.environ,
			})
			if code == core.ExitOK {
				return nil
			}
			return &exitError{code: code}
		},
	}
	f := cmd.Flags()
	f.StringVarP(&project, "project", "p", "", "project key to open the board for")
	f.StringVar(&filter, "filter", "", "filter expression to start with")
	return cmd
}

func init() { builders = append(builders, newTUICmd) }
