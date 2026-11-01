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

// The settings screen's labels: shortcut actions and schemes, colouring.
func init() {
	labelSets = append(labelSets, func() []string {
		var out []string
		for _, a := range shortcutActions {
			out = append(out, a.Label)
		}
		for _, s := range KeySchemes() {
			out = append(out, keySchemeLabel(s))
		}
		for _, v := range tintLabels {
			out = append(out, v)
		}
		for _, v := range projectLabelNames {
			out = append(out, v)
		}
		for _, p := range columnPages {
			out = append(out, p.Label)
		}
		return out
	})
}
