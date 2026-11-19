// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Snooze":           "Reporter",
		"Unsnooze":         "Ne plus reporter",
		"Snoozed until %s": "Reportée jusqu’à %s",
		"Snoozed by %s":    "Reportée par %s",
		"snoozed by %s":    "reportée par %s",
		"until %s":         "jusqu’à %s",
		"Later today":      "Plus tard aujourd’hui",
		"Tomorrow morning": "Demain matin",
		"Next week":        "La semaine prochaine",
		"Pick a date":      "Choisir une date",
	})
}
