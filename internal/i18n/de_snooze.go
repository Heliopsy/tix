// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Snooze":           "Zurückstellen",
		"Unsnooze":         "Nicht mehr zurückstellen",
		"Snoozed until %s": "Zurückgestellt bis %s",
		"Snoozed by %s":    "Zurückgestellt von %s",
		"snoozed by %s":    "zurückgestellt von %s",
		"until %s":         "bis %s",
		"Later today":      "Später heute",
		"Tomorrow morning": "Morgen früh",
		"Next week":        "Nächste Woche",
		"Pick a date":      "Datum wählen",
	})
}
