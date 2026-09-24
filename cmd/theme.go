// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"github.com/spf13/cobra"
)

// newThemeCmd builds the theme command group.
//
// It reads configuration and never dials. A theme is a deployment setting
// rather than a record, so the answer to "what can I name here" comes from the
// built-ins plus this machine's `themes:` block; opening a database to find out
// would be asking the wrong thing, and would make the command fail on a host
// that cannot reach one.
func newThemeCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "theme",
		Short:   "Inspect the palettes a tenant may use",
		GroupID: "admin",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(themeLsCmd(g))
	return cmd
}

func themeLsCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List every resolvable theme",
		Long: "List every theme a tenant may name: the built-in palettes and anything\n" +
			"this deployment defines under `themes:` in its configuration.\n\n" +
			"A configured theme may reuse a built-in name, and then it wins, so the\n" +
			"source column is what tells you which palette you are actually getting.\n\n" +
			"Set one with `tix tenant edit REF --theme NAME`.\n\n" +
			"Exit codes: 2 a theme in configuration has a colour that is not #rrggbb.",
		Example: "  tix theme ls\n  tix theme ls -o json",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolved, err := g.resolve()
			if err != nil {
				return err
			}
			themes, err := resolved.Config.ThemeRegistry()
			if err != nil {
				return err
			}
			return g.render(cmd, themes.List())
		},
	}
}

func init() { builders = append(builders, newThemeCmd) }
