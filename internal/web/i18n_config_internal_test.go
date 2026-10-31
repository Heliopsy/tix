// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "github.com/heliopsy/tix/internal/core"

func init() {
	labelSets = append(labelSets,
		func() []string {
			var out []string
			for _, k := range core.ComponentKinds {
				out = append(out, componentKindLabel(k))
			}
			return out
		},
		func() []string {
			var out []string
			for _, s := range syncSystems {
				out = append(out, s.Note)
				for _, setting := range syncSettingsFor(s.Value) {
					out = append(out, setting.Purpose)
				}
			}
			return out
		},
	)
}
