// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Snooze":                                 "Zurückstellen",
		"Unsnooze":                               "Nicht mehr zurückstellen",
		"Snooze %s":                              "%s zurückstellen",
		"Snoozed until %s":                       "Zurückgestellt bis %s",
		"Snoozed by %s":                          "Zurückgestellt von %s",
		"snoozed by %s":                          "zurückgestellt von %s",
		"until %s":                               "bis %s",
		"Later today":                            "Später heute",
		"Tomorrow morning":                       "Morgen früh",
		"Next week":                              "Nächste Woche",
		"Pick a date":                            "Datum wählen",
		"Hide it from every listing until later": "Bis später aus allen Listen ausblenden",
		"A snoozed task leaves every listing, for everybody, until the time you pick, then comes back by itself. Agents cannot claim it meanwhile. is:snoozed finds it.": "Eine zurückgestellte Aufgabe verschwindet für alle aus jeder Liste, bis zur gewählten Zeit, und kommt dann von selbst zurück. Agenten können sie in der Zwischenzeit nicht übernehmen. is:snoozed findet sie.",
	})
}
