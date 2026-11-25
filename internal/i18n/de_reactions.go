// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Reactions":        "Reaktionen",
		"Add reaction":     "Reaktion hinzufügen",
		"React with %s":    "Mit %s reagieren",
		"Any emoji":        "Beliebiges Emoji",
		"React":            "Reagieren",
		"reaction added":   "Reaktion hinzugefügt",
		"reaction removed": "Reaktion entfernt",
	})
}
