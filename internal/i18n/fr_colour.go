// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// The shared colour control.
		"Custom":                    "Personnalisée",
		"Custom colour, as #rrggbb": "Couleur personnalisée, au format #rrggbb",
		"Pick a custom colour":      "Choisir une couleur personnalisée",
		"Preview":                   "Aperçu",
		"Pick one of the predefined colours, or choose Custom and set any colour with the picker or as #rrggbb. Each colour scheme draws a custom colour as close to it as stays readable.":            "Choisissez l’une des couleurs prédéfinies, ou choisissez Personnalisée et définissez n’importe quelle couleur avec le sélecteur ou au format #rrggbb. Chaque jeu de couleurs affiche une couleur personnalisée aussi proche que possible tout en restant lisible.",
		"The accent this tenant is drawn in. Pick a predefined theme, or choose Custom and set any accent with the picker or as #rrggbb; its tint and its shade for dark schemes are derived from it.": "La couleur d’accent de cet espace. Choisissez un thème prédéfini, ou choisissez Personnalisée et définissez n’importe quel accent avec le sélecteur ou au format #rrggbb ; sa teinte claire et sa nuance pour les jeux sombres en sont dérivées.",
	})
}
