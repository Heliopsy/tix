// SPDX-License-Identifier: AGPL-3.0-or-later

package web

func init() {
	labelSets = append(labelSets, func() []string {
		var out []string
		for _, p := range snoozePresets {
			out = append(out, p.Label)
		}
		return out
	})
}
