// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"My work":                           "Meine Arbeit",
		"Include tasks I hold":              "Von mir gehaltene Aufgaben einbeziehen",
		"Hidden groups":                     "Ausgeblendete Gruppen",
		"Overdue":                           "Überfällig",
		"Today":                             "Heute",
		"This week":                         "Diese Woche",
		"Later":                             "Später",
		"No date":                           "Kein Datum",
		"the first %d by deadline":          "die ersten %d nach Frist",
		"see them all":                      "alle anzeigen",
		"Save this filter":                  "Diesen Filter speichern",
		"For example, my overdue work":      "Zum Beispiel meine überfälligen Aufgaben",
		"Share with everyone in the tenant": "Mit allen im Mandanten teilen",
		"Saved filters":                     "Gespeicherte Filter",
		"shared":                            "geteilt",
		"Stop sharing":                      "Nicht mehr teilen",
		"Recently viewed":                   "Zuletzt angesehen",
		"filter saved":                      "Filter gespeichert",
		"filter removed":                    "Filter entfernt",
		"Tasks assigned to you that are still open, under a heading per deadline: overdue, today, this week, later and no date. Include tasks I hold adds the ones you have claimed. The same list is tix task ls --mine, or assignee:me in any filter.": "Offene Aufgaben, die Ihnen zugewiesen sind, unter einer Überschrift pro Frist: überfällig, heute, diese Woche, später und ohne Datum. Von mir gehaltene Aufgaben einbeziehen fügt die von Ihnen übernommenen hinzu. Dieselbe Liste liefert tix task ls --mine oder assignee:me in jedem Filter.",
		"A saved filter keeps this filter, its sort and the view under a name in the sidebar, under Tasks. It is yours; a shared one is listed for everybody in the tenant, and only you or an administrator can change it.":                             "Ein gespeicherter Filter bewahrt diesen Filter, seine Sortierung und die Ansicht unter einem Namen in der Seitenleiste unter Aufgaben auf. Er gehört Ihnen; ein geteilter wird allen im Mandanten angezeigt, und nur Sie oder ein Administrator können ihn ändern.",
	})
}
