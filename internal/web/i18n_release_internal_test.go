// SPDX-License-Identifier: AGPL-3.0-or-later

package web

func init() {
	labelSets = append(labelSets, func() []string {
		var out []string
		for _, m := range upgradeMethods("0.0.0") {
			out = append(out, m.Name, m.Help)
		}
		return out
	})
}
