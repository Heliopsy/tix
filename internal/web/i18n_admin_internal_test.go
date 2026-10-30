// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "github.com/heliopsy/tix/internal/core"

// The admin screens translate the token expiry choices, the token states, the
// delivery statuses and the rows of the tenant diagram, all fixed English
// labels Go hands the templates.
func init() {
	labelSets = append(labelSets,
		func() []string {
			var out []string
			for _, c := range tokenExpiryChoices {
				out = append(out, c.Label)
			}
			return out
		},
		func() []string { return []string{"active", "revoked", "expired"} },
		func() []string {
			return []string{string(core.DeliveryPending), string(core.DeliveryDelivered), string(core.DeliveryFailed)}
		},
		func() []string {
			return []string{
				"Tenant", "Members", "Domains", "API tokens", "Workflows", "Projects", "Tasks", "Field definitions",
				"everything below belongs to it and is invisible from any other",
				"people, each with a role here",
				"hostnames that resolve to this tenant",
				"what an agent authenticates with",
				"the states a task moves between",
				"each one picks a workflow",
				"the work, and its comments, artifacts and dependencies",
				"the custom fields a task in that project carries",
				countFailed, countShort, "on the task list", "per project",
			}
		},
	)
}
