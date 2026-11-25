// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Filter, for example tag:ops due:week @raj": "Filtern, z. B. tag:ops due:week @raj",
		"Filters":                "Filter",
		"a handle, or me":        "ein Handle oder me",
		"add a tag":              "Tag hinzufügen",
		"any status":             "beliebiger Status",
		"as typed":               "wie eingegeben",
		"any priority":           "beliebige Priorität",
		"Include finished tasks": "Abgeschlossene Aufgaben einschließen",
		"Sort by":                "Sortieren nach",
		"Order":                  "Reihenfolge",
		"ascending":              "aufsteigend",
		"descending":             "absteigend",
		"More":                   "Mehr",
		"%d per page (default)":  "%d pro Seite (Standard)",
		"%d per page":            "%d pro Seite",
		"Rename, share, reorder or delete them in the sidebar, under Tasks.": "Umbenennen, teilen, umsortieren oder löschen Sie sie in der Seitenleiste unter „Aufgaben“.",
		"Filter the list, then save it here.":                                "Filtern Sie die Liste und speichern Sie den Filter dann hier.",
		"%d filter in force":                                                 "%d Filter aktiv",
		"%d filters in force":                                                "%d Filter aktiv",
		"Filters in force":                                                   "Aktive Filter",
		"including finished":                                                 "einschließlich abgeschlossener",
		"Apply page size":                                                    "Seitengröße übernehmen",
		"Filter suggestions":                                                 "Filtervorschläge",
	})
}
