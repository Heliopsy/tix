// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "github.com/heliopsy/tix/internal/core"

func init() {
	labelSets = append(labelSets, func() []string {
		var out []string
		for _, c := range davChoices(core.AllDAVCollections) {
			out = append(out, c.Label, c.Help)
		}
		// The reason GetDAVInfo gives while caldav.enabled is false, which
		// the section translates. TestDisabledCalDAVShowsTheReason reads it
		// from the service, so a change there fails that test first.
		return append(out, "CalDAV is turned off on this server (caldav.enabled)")
	})
}
