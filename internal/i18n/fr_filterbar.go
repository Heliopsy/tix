// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Filter, for example tag:ops due:week @raj": "Filtrer, par exemple tag:ops due:week @raj",
		"Filters":                "Filtres",
		"a handle, or me":        "un identifiant, ou me",
		"add a tag":              "ajouter un tag",
		"any status":             "tout statut",
		"as typed":               "tel que saisi",
		"any priority":           "toute priorité",
		"Include finished tasks": "Inclure les tâches terminées",
		"Sort by":                "Trier par",
		"Order":                  "Ordre",
		"ascending":              "croissant",
		"descending":             "décroissant",
		"More":                   "Plus",
		"%d per page (default)":  "%d par page (par défaut)",
		"%d per page":            "%d par page",
		"Rename, share, reorder or delete them in the sidebar, under Tasks.": "Renommez-les, partagez-les, réordonnez-les ou supprimez-les dans la barre latérale, sous Tâches.",
		"Filter the list, then save it here.":                                "Filtrez la liste, puis enregistrez-la ici.",
		"%d filter in force":                                                 "%d filtre actif",
		"%d filters in force":                                                "%d filtres actifs",
		"Filters in force":                                                   "Filtres actifs",
		"including finished":                                                 "terminées comprises",
		"Apply page size":                                                    "Appliquer la taille de page",
		"Filter suggestions":                                                 "Suggestions de filtre",
	})
}
