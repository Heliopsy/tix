// SPDX-License-Identifier: AGPL-3.0-or-later

package web

func init() {
	labelSets = append(labelSets, timePrefKeys)
}

// timePrefKeys are the words the date preference controls are built from in
// Go: the inherit choice, each level's name, and where a value came from.
func timePrefKeys() []string {
	out := []string{"Inherit", zoneLabel("")}
	for _, label := range timeSourceLabels {
		out = append(out, label, "from "+label)
	}
	return out
}
