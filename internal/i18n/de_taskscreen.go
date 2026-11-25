// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Properties":                      "Eigenschaften",
		"Claim":                           "Übernehmen",
		"Created by":                      "Erstellt von",
		"Snoozed":                         "Zurückgestellt",
		"Snooze…":                         "Zurückstellen…",
		"Reclaim the lease…":              "Sperre zurücknehmen…",
		"Does not repeat":                 "Keine Wiederholung",
		"No subtasks.":                    "Keine Unteraufgaben.",
		"Waits for nothing.":              "Wartet auf nichts.",
		"Depends on, for example infra-2": "Hängt ab von, z. B. infra-2",
		"Remove the dependency on %s":     "Abhängigkeit von %s entfernen",
		"in":                              "in",
	})
}
