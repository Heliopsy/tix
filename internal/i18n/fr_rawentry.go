// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The dialog a history row's Raw control opens: one audit entry as recorded.
func init() {
	register(French, map[string]string{
		"Raw entry":                        "Entrée brute",
		"Entry":                            "Entrée",
		"Step %d of %d":                    "Étape %d sur %d",
		"Actor":                            "Acteur",
		"Subject":                          "Sujet",
		"Time":                             "Heure",
		"Sequence":                         "Séquence",
		"system":                           "système",
		"Changed fields":                   "Champs modifiés",
		"Field":                            "Champ",
		"Before":                           "Avant",
		"After":                            "Après",
		"added":                            "ajouté",
		"removed":                          "retiré",
		"(none)":                           "(aucun)",
		"This entry recorded no snapshot.": "Cette entrée n’a enregistré aucun instantané.",
		"Copy all as JSON":                 "Tout copier en JSON",
	})
}
