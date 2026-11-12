// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Group lanes by":        "Bahnen gruppieren nach",
		"Unassigned":            "Nicht zugewiesen",
		"No tag":                "Kein Tag",
		"Show only these tasks": "Nur diese Aufgaben anzeigen",
		"A task with several tags is drawn once, in the first lane among its tags. Its card lists them all.":                                                                                                                                              "Eine Aufgabe mit mehreren Tags wird einmal gezeigt, in der ersten Bahn unter ihren Tags. Ihre Karte listet alle auf.",
		"Each lane is a project, a person, a priority, a tag or a deadline. A task with several tags is drawn once, in the first lane among its tags. The choice is in the address, so a link keeps it; the task list also remembers it in this browser.": "Jede Bahn ist ein Projekt, eine Person, eine Priorität, ein Tag oder eine Frist. Eine Aufgabe mit mehreren Tags wird einmal gezeigt, in der ersten Bahn unter ihren Tags. Die Auswahl steht in der Adresse, ein Link behält sie also; die Aufgabenliste merkt sie sich außerdem in diesem Browser.",
	})
}
