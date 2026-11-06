// SPDX-License-Identifier: AGPL-3.0-or-later

package web

func init() {
	labelSets = append(labelSets, func() []string {
		out := append([]string(nil), viaDetailLabels...)
		for _, pair := range viaLabels {
			out = append(out, pair[0], pair[1])
		}
		for _, l := range viaChoiceLabels {
			out = append(out, l)
		}
		return out
	})
}
