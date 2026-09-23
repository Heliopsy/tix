// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/tui"
	"github.com/spf13/cobra"
)

// newTUICmd builds the command that runs the terminal interface.
// schemeNames lists the keybinding schemes the terminal interface ships.
func schemeNames() []string {
	out := make([]string, 0, len(tui.Schemes()))
	for _, s := range tui.Schemes() {
		out = append(out, string(s))
	}
	return out
}

func newTUICmd(g *globals) *cobra.Command {
	var project, filter, scheme string
	var overrides map[string]string
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
			// The flag wins, the configured scheme is the fallback, so a
			// choice made in the settings view survives a restart without
			// taking the flag away from a one-off run.
			resolved, err := g.resolve()
			if err != nil {
				return err
			}
			if scheme == "" {
				scheme = resolved.Config.TUI.Keymap
			}
			overrides = mergeOverrides(resolved.Config.TUI.Keys, overrides)
			// Refuse an unknown scheme before opening a terminal, so a typo in
			// tui.keymap reports the typo rather than a TTY error.
			if _, err := tui.ParseScheme(scheme); err != nil {
				return err
			}
			code := tui.Run(tui.Options{
				TimeStyle: g.timeStyle(),
				Service:   conn.Service,
				Actor:     conn.Actor,
				Context:   ctx,
				Project:   project,
				Filter:    filter,
				Scheme:    scheme,
				Overrides: overrides,
				In:        cmd.InOrStdin(),
				Out:       cmd.OutOrStdout(),
				Err:       cmd.ErrOrStderr(),
				Environ:   g.environ,
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
	f.StringVar(&scheme, "keys", "", "keybinding scheme: "+strings.Join(schemeNames(), "|"))
	f.StringToStringVar(&overrides, "key", nil, "rebind one action, such as --key New=o")
	_ = cmd.RegisterFlagCompletionFunc("keys", fixedCompletion(schemeNames()))
	return cmd
}

func init() { builders = append(builders, newTUICmd) }

// mergeOverrides layers per-invocation rebindings over the configured ones.
// A flag naming an action the configuration also rebinds wins for that action
// alone, rather than discarding the rest of the configured set.
func mergeOverrides(configured, flags map[string]string) map[string]string {
	if len(configured) == 0 {
		return flags
	}
	out := make(map[string]string, len(configured)+len(flags))
	for action, key := range configured {
		out[action] = key
	}
	for action, key := range flags {
		out[action] = key
	}
	return out
}
