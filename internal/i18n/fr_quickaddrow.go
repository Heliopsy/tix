// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Quick add suggestions":        "Suggestions d’ajout rapide",
		"Quick add from the clipboard": "Ajout rapide depuis le presse-papiers",
		"The clipboard holds no text.": "Le presse-papiers ne contient pas de texte.",
		"Stay open for the next line":  "Rester ouvert pour la ligne suivante",
		"Add another":                  "En ajouter une autre",
	})
}
