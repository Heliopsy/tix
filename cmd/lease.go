// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// leaseFlag reads a duration flag that "" clears back to the inherited value.
// It returns nil when the flag was not given, so the field is left alone.
func leaseFlag(cmd *cobra.Command, name, raw string) (*core.Duration, error) {
	if !cmd.Flags().Changed(name) {
		return nil, nil
	}
	var d core.Duration
	if raw != "" {
		parsed, err := core.ParseDuration(raw)
		if err != nil {
			return nil, core.Invalid("--%s: %v", name, err)
		}
		d = parsed
	}
	return &d, nil
}

// leasePhrase is the short form a claim table shows, such as "4h (project)".
func leasePhrase(g *core.LeaseGrant) string {
	if g == nil {
		return ""
	}
	if g.Clamped {
		return fmt.Sprintf("%s (%s maximum)", g.TTL.Human(), g.MaxSource)
	}
	return fmt.Sprintf("%s (%s)", g.TTL.Human(), g.Source)
}

// noteLease prints under a shown task how long a claim on it would hold. A
// caller allowed to read the task but not this is told nothing rather than
// failing the show.
func noteLease(ctx context.Context, g *globals, cmd *cobra.Command, svc core.Service, task *core.Task) {
	if g.formatName() != output.FormatTable {
		return
	}
	grant, err := svc.TaskLease(ctx, core.TaskRef{ID: task.ID})
	if err != nil {
		return
	}
	g.diag(cmd, "%s", grant.Sentence())
}
