// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"My work":                           "Mon travail",
		"Include tasks I hold":              "Inclure les tâches que je détiens",
		"Hidden groups":                     "Groupes masqués",
		"Overdue":                           "En retard",
		"Today":                             "Aujourd’hui",
		"This week":                         "Cette semaine",
		"Later":                             "Plus tard",
		"No date":                           "Sans date",
		"the first %d by deadline":          "les %d premières par échéance",
		"see them all":                      "les voir toutes",
		"Save this filter":                  "Enregistrer ce filtre",
		"For example, my overdue work":      "Par exemple, mon travail en retard",
		"Share with everyone in the tenant": "Partager avec tout l’espace",
		"Saved filters":                     "Filtres enregistrés",
		"shared":                            "partagé",
		"Stop sharing":                      "Ne plus partager",
		"Recently viewed":                   "Consultés récemment",
		"filter saved":                      "filtre enregistré",
		"filter removed":                    "filtre supprimé",
		"Tasks assigned to you that are still open, under a heading per deadline: overdue, today, this week, later and no date. Include tasks I hold adds the ones you have claimed. The same list is tix task ls --mine, or assignee:me in any filter.": "Les tâches qui vous sont assignées et encore ouvertes, sous un en-tête par échéance : en retard, aujourd’hui, cette semaine, plus tard et sans date. Inclure les tâches que je détiens ajoute celles que vous avez prises. La même liste s’obtient avec tix task ls --mine, ou assignee:me dans n’importe quel filtre.",
		"A saved filter keeps this filter, its sort and the view under a name in the sidebar, under Tasks. It is yours; a shared one is listed for everybody in the tenant, and only you or an administrator can change it.":                             "Un filtre enregistré conserve ce filtre, son tri et la vue sous un nom dans la barre latérale, sous Tâches. Il vous appartient ; un filtre partagé est listé pour tout le monde dans l’espace, et seuls vous ou un administrateur pouvez le modifier.",
	})
}
