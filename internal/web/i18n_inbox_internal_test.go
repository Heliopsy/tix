// SPDX-License-Identifier: AGPL-3.0-or-later

package web

func init() {
	labelSets = append(labelSets, func() []string {
		var out []string
		for _, c := range inboxKindChoices(nil) {
			out = append(out, c.Label, c.Help)
		}
		for _, c := range channelFormats() {
			out = append(out, c.Label)
		}
		return append(out, "Add a channel", "Add channel", "Settings", "Save channel", "test sent",
			"test not delivered yet", "deliveries")
	})
}
