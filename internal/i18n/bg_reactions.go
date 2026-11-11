// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Reactions":        "Реакции",
		"Add reaction":     "Добави реакция",
		"React with %s":    "Реагирай с %s",
		"Any emoji":        "Друг емоджи",
		"React":            "Реагирай",
		"reaction added":   "реакцията е добавена",
		"reaction removed": "реакцията е премахната",
	})
}
