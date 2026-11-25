// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Actions for %s":              "Aktionen für %s",
		"Arranging columns and lanes": "Spalten und Bahnen anordnen",
		"Collapse":                    "Einklappen",
		"Expand":                      "Ausklappen",
		"Hide":                        "Ausblenden",
		"Hidden: %d":                  "Ausgeblendet: %d",
		"Hidden columns":              "Ausgeblendete Spalten",
		"Hidden lanes":                "Ausgeblendete Bahnen",
		"None hidden.":                "Nichts ausgeblendet.",
		"Show %s":                     "%s anzeigen",
		"Move left":                   "Nach links",
		"Move right":                  "Nach rechts",
		"Move up":                     "Nach oben",
		"Move down":                   "Nach unten",
		"Star, to show it first":      "Als Favorit markieren, um es zuerst zu zeigen",
		"Open the project":            "Projekt öffnen",
		"Show only this project":      "Nur dieses Projekt anzeigen",
		"Show only this status":       "Nur diesen Status anzeigen",
		"New task in %s":              "Neue Aufgabe in %s",
		"Reset the arrangement":       "Anordnung zurücksetzen",
		"Group the list":              "Liste gruppieren",
		"How to group the list":       "Wie die Liste gruppiert wird",
		"Grouping":                    "Gruppierung",
		"List groups":                 "Listengruppen",
		"No groups":                   "Keine Gruppen",
		"By project":                  "Nach Projekt",
		"By status":                   "Nach Status",
		"How the list on <a href=\"/tasks\">Tasks</a> groups its rows.": "Wie die Liste unter <a href=\"/tasks\">Aufgaben</a> ihre Zeilen gruppiert.",
		"Draws a heading per project or per status over the rows of this page. Each heading has the same menu as a lane or a board column, and shares its arrangement.":                                                "Zeigt eine Überschrift pro Projekt oder pro Status über den Zeilen dieser Seite. Jede Überschrift hat dasselbe Menü wie eine Bahn oder eine Board-Spalte und teilt deren Anordnung.",
		"The three dots on a column, lane or group heading hide, star, collapse or move it. The arrangement is yours, kept with your account, so it follows you to any browser and changes nothing anybody else sees.": "Die drei Punkte an einer Spalte, Bahn oder Gruppenüberschrift blenden sie aus, markieren sie als Favorit, klappen sie ein oder verschieben sie. Die Anordnung gehört Ihnen und wird mit Ihrem Konto gespeichert, sie folgt Ihnen also in jeden Browser und ändert nichts an dem, was andere sehen.",
		"Whether the list draws a heading per project or per status over its rows. Each heading's menu hides, stars, collapses or moves the group, kept with your account.":                                            "Ob die Liste eine Überschrift pro Projekt oder pro Status über ihren Zeilen zeigt. Das Menü jeder Überschrift blendet die Gruppe aus, markiert sie als Favorit, klappt sie ein oder verschiebt sie, gespeichert mit Ihrem Konto.",
	})
}
