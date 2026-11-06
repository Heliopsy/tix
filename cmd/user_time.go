// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"github.com/heliopsy/tix/internal/core"
	"github.com/spf13/cobra"
)

// newUserTimeCmd shows and changes the caller's own date format and timezone.
func newUserTimeCmd(g *globals) *cobra.Command {
	var format, zone string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "time",
		Short: "Show or set your date format and timezone",
		Long: "Show the date format and timezone your dates are shown in, and where each comes from: " +
			"the deployment (output.time_format, output.timezone), this tenant's default, or your own " +
			"choice. Your own choice follows you to every browser, terminal and tenant.\n\n" +
			"--format takes iso, rfc3339, short, long, dotted, dayfirst, us or relative; --timezone takes " +
			"an IANA name such as Europe/Sofia or UTC. Either as \"\" goes back to the tenant's default.\n\n" +
			"A format or timezone set in this machine's configuration (a flag, TIX_OUTPUT_* or the " +
			"configuration file) still wins on this machine. JSON and YAML output always carry RFC 3339.\n\n" +
			"Exit codes: 2 an unknown format or timezone, or the caller is not a person.",
		Example: "  tix user time\n" +
			"  tix user time --format dotted --timezone Europe/Sofia\n" +
			"  tix user time --timezone \"\"",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var in core.TimePrefsInput
			if cmd.Flags().Changed("format") {
				in.TimeFormat = &format
			}
			if cmd.Flags().Changed("timezone") {
				in.Timezone = &zone
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if in.TimeFormat == nil && in.Timezone == nil {
				out, err := conn.Service.GetTimeSettings(ctx)
				if err != nil {
					return err
				}
				return g.render(cmd, out)
			}
			if dryRun {
				core.NormalizeTimePrefs(in.TimeFormat, in.Timezone)
				if err := in.Validate(); err != nil {
					return err
				}
				g.diag(cmd, "dry run: nothing was changed")
				return g.render(cmd, in)
			}
			out, err := conn.Service.SetTimePrefs(ctx, in)
			if err != nil {
				return err
			}
			return g.render(cmd, out)
		},
	}
	cmd.Flags().StringVar(&format, "format", "", "your date format, or \"\" for the tenant's default")
	cmd.Flags().StringVar(&zone, "timezone", "", "your IANA timezone, or \"\" for the tenant's default")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "check the values without saving them")
	_ = cmd.RegisterFlagCompletionFunc("format", fixedCompletion(core.TimeFormatNames))
	return cmd
}
