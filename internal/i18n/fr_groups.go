// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Actions for %s":              "Actions pour %s",
		"Arranging columns and lanes": "Disposition des colonnes et des couloirs",
		"Collapse":                    "Réduire",
		"Expand":                      "Développer",
		"Hide":                        "Masquer",
		"Hidden: %d":                  "Masquées : %d",
		"Hidden columns":              "Colonnes masquées",
		"Hidden lanes":                "Couloirs masqués",
		"None hidden.":                "Rien de masqué.",
		"Show %s":                     "Afficher %s",
		"Move left":                   "Déplacer à gauche",
		"Move right":                  "Déplacer à droite",
		"Move up":                     "Monter",
		"Move down":                   "Descendre",
		"Star, to show it first":      "Ajouter aux favoris, pour l’afficher en premier",
		"Open the project":            "Ouvrir le projet",
		"Show only this project":      "Afficher seulement ce projet",
		"Show only this status":       "Afficher seulement ce statut",
		"New task in %s":              "Nouvelle tâche dans %s",
		"Reset the arrangement":       "Réinitialiser la disposition",
		"Group the list":              "Regrouper la liste",
		"How to group the list":       "Comment regrouper la liste",
		"Grouping":                    "Regroupement",
		"List groups":                 "Groupes de la liste",
		"No groups":                   "Sans regroupement",
		"By project":                  "Par projet",
		"By status":                   "Par statut",
		"How the list on <a href=\"/tasks\">Tasks</a> groups its rows.": "Comment la liste de <a href=\"/tasks\">Tâches</a> regroupe ses lignes.",
		"Draws a heading per project or per status over the rows of this page. Each heading has the same menu as a lane or a board column, and shares its arrangement.":                                                "Affiche un en-tête par projet ou par statut au-dessus des lignes de cette page. Chaque en-tête a le même menu qu’un couloir ou une colonne du tableau, et partage sa disposition.",
		"The three dots on a column, lane or group heading hide, star, collapse or move it. The arrangement is yours, kept with your account, so it follows you to any browser and changes nothing anybody else sees.": "Les trois points d’un en-tête de colonne, de couloir ou de groupe le masquent, l’ajoutent aux favoris, le réduisent ou le déplacent. La disposition vous appartient, elle est conservée avec votre compte : elle vous suit dans tout navigateur et ne change rien à ce que voient les autres.",
		"Whether the list draws a heading per project or per status over its rows. Each heading's menu hides, stars, collapses or moves the group, kept with your account.":                                            "Si la liste affiche un en-tête par projet ou par statut au-dessus de ses lignes. Le menu de chaque en-tête masque, ajoute aux favoris, réduit ou déplace le groupe, ce qui est conservé avec votre compte.",
	})
}
