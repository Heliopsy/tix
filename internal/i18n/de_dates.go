// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"For example:":                      "Zum Beispiel:",
		"tomorrow, +3d or %s":               "tomorrow, +3d oder %s",
		"Pick the due date from a calendar": "Fälligkeitsdatum aus einem Kalender wählen",
		"A word, an offset from today or a date in your date format. Leave it empty for no due date.": "Ein Wort, ein Abstand ab heute oder ein Datum in Ihrem Datumsformat. Leer lassen für kein Fälligkeitsdatum.",
	})
}
