// SPDX-License-Identifier: AGPL-3.0-or-later

package web

func init() {
	labelSets = append(labelSets, func() []string {
		var out []string
		for _, l := range exportColumnLabels {
			out = append(out, l)
		}
		for _, l := range formatLabels {
			out = append(out, l)
		}
		for _, l := range delimiterLabels {
			out = append(out, l)
		}
		return out
	})
}
