// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"fmt"

	"github.com/heliopsy/tix/internal/core"
	"github.com/spf13/cobra"
)

// newRetentionCmd builds the retention policy command group.
func newRetentionCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "retention",
		Short:   "Inspect and change the history retention policy",
		GroupID: "admin",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(retentionShowCmd(g), retentionSetCmd(g))
	return cmd
}

func retentionShowCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "show",
		Aliases: []string{"get"},
		Short:   "Show the retention policy tix prune enforces",
		Long: "Show the windows events, audit entries and webhook deliveries are kept for.\n\n" +
			fmt.Sprintf("Exit codes: %d permission denied.", core.KindForbidden.ExitCode()),
		Example: "  tix retention show\n  tix retention show -o json",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			policy, err := conn.Service.GetRetention(ctx)
			if err != nil {
				return err
			}
			return g.render(cmd, policy)
		},
	}
}

func retentionSetCmd(g *globals) *cobra.Command {
	var events, audit, deliveries string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Change the retention policy",
		Long: "Change one or more retention windows. A window left unset keeps its current value.\n\n" +
			fmt.Sprintf("Exit codes: %d invalid window, %d permission denied.",
				core.KindInvalid.ExitCode(), core.KindForbidden.ExitCode()),
		Example: "  tix retention set --events 30d\n" +
			"  tix retention set --audit 365d --webhook-deliveries 7d",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			policy, err := retentionPolicy(cmd, events, audit, deliveries)
			if err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("retention.put", "", map[string]any{
					"events":             policy.Events.String(),
					"audit_entries":      policy.AuditEntries.String(),
					"webhook_deliveries": policy.WebhookDeliveries.String(),
				}))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			updated, err := conn.Service.PutRetention(ctx, policy)
			if err != nil {
				return err
			}
			return g.render(cmd, updated)
		},
	}
	f := cmd.Flags()
	f.StringVar(&events, "events", "", "how long events are kept, as a duration such as 30d or 720h")
	f.StringVar(&audit, "audit", "", "how long audit entries are kept")
	f.StringVar(&deliveries, "webhook-deliveries", "", "how long webhook deliveries are kept")
	f.BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

// retentionPolicy builds the policy from the windows the operator named. A
// window the service receives as zero keeps the value already stored, so a
// flag given explicitly must carry a positive duration.
func retentionPolicy(cmd *cobra.Command, events, audit, deliveries string) (core.RetentionPolicy, error) {
	var policy core.RetentionPolicy
	windows := []struct {
		flag  string
		value string
		into  *core.Duration
	}{
		{"events", events, &policy.Events},
		{"audit", audit, &policy.AuditEntries},
		{"webhook-deliveries", deliveries, &policy.WebhookDeliveries},
	}
	changed := false
	for _, w := range windows {
		if !cmd.Flags().Changed(w.flag) {
			continue
		}
		changed = true
		d, err := parseDuration(w.value)
		if err != nil {
			return policy, err
		}
		if d <= 0 {
			return policy, core.Invalid("retention window for %s must be positive", w.flag)
		}
		*w.into = d
	}
	if !changed {
		return policy, usagef(cmd, "at least one of --events, --audit and --webhook-deliveries is required")
	}
	return policy, nil
}
