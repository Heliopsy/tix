// SPDX-License-Identifier: AGPL-3.0-or-later

package web

func init() {
	labelSets = append(labelSets, func() []string {
		var out []string
		for _, c := range inboxKindChoices(nil) {
			out = append(out, c.Label, c.Help)
		}
		return out
	})
}
