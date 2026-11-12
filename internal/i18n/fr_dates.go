// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"For example:":                      "Par exemple :",
		"tomorrow, +3d or %s":               "tomorrow, +3d ou %s",
		"Pick the due date from a calendar": "Choisir l’échéance dans un calendrier",
		"A word, an offset from today or a date in your date format. Leave it empty for no due date.": "Un mot, un décalage depuis aujourd’hui ou une date dans votre format de date. Laissez vide pour aucune échéance.",
	})
}
