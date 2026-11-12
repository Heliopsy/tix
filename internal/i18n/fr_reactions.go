// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Reactions":        "Réactions",
		"Add reaction":     "Ajouter une réaction",
		"React with %s":    "Réagir avec %s",
		"Any emoji":        "N’importe quel emoji",
		"React":            "Réagir",
		"reaction added":   "réaction ajoutée",
		"reaction removed": "réaction retirée",
	})
}
