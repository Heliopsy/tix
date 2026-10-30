// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "github.com/heliopsy/tix/internal/core"

func init() {
	labelSets = append(labelSets,
		func() []string {
			var out []string
			for _, c := range priorityChoices {
				out = append(out, c.Label)
			}
			return out
		},
		func() []string {
			var out []string
			for _, k := range core.ArtifactKinds {
				out = append(out, string(k))
			}
			return out
		},
	)
}
