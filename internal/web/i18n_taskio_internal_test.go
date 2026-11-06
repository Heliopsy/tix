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

func init() {
	labelSets = append(labelSets, func() []string {
		var out []string
		for _, s := range importSteps {
			out = append(out, s.Label)
		}
		for _, l := range importTargetLabels {
			out = append(out, l)
		}
		for _, l := range importModeLabels {
			out = append(out, l)
		}
		for _, l := range importPolicyLabels {
			out = append(out, l)
		}
		return append(out, "No mapping", "Status", "Priority", "Assignee", "create", "update", "skip", "error")
	})
}
