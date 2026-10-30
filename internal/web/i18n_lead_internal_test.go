// SPDX-License-Identifier: AGPL-3.0-or-later

package web

// The labels the shared partials translate: column choices, the view switch
// and the colour scheme switch.
func init() {
	labelSets = append(labelSets, func() []string {
		var out []string
		for _, set := range columnSets {
			for _, c := range set {
				out = append(out, c.Label)
			}
		}
		for _, v := range taskViewLabels {
			out = append(out, v)
		}
		for _, v := range themeLabels {
			out = append(out, v)
		}
		return out
	})
}
