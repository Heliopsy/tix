// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// Housekeeping: the cleanup rules on the Tenant screen and in a
		// project's settings.
		"Housekeeping":           "Aufräumen",
		"Save housekeeping":      "Aufräumregeln speichern",
		"Clean up now":           "Jetzt aufräumen",
		"Done tasks":             "Erledigte Aufgaben",
		"Cancelled tasks":        "Abgebrochene Aufgaben",
		"After a number of days": "Nach einer Anzahl von Tagen",
		"days":                   "Tage",
		"Inherit: off":           "Erben: aus",
		"Inherit: %d day":        "Erben: %d Tag",
		"Inherit: %d days":       "Erben: %d Tage",
		"%d task would be moved to the trash now.":  "%d Aufgabe würde jetzt in den Papierkorb verschoben.",
		"%d tasks would be moved to the trash now.": "%d Aufgaben würden jetzt in den Papierkorb verschoben.",
		"%d task would be deleted for good now.":    "%d Aufgabe würde jetzt endgültig gelöscht.",
		"%d tasks would be deleted for good now.":   "%d Aufgaben würden jetzt endgültig gelöscht.",
		"cleanup saved": "Aufräumregeln gespeichert",
		"cleanup ran":   "Aufräumen ausgeführt",
		"cleanup ran: %d moved to the trash, %d deleted for good":                  "Aufräumen ausgeführt: %d in den Papierkorb verschoben, %d endgültig gelöscht",
		"Cleanup acts on this project only: %d to the trash, %d deleted for good.": "Das Aufräumen betrifft nur dieses Projekt: %d in den Papierkorb, %d endgültig gelöscht.",
		"Clean up this project":                    "Dieses Projekt aufräumen",
		"Paused while the trash is off.":           "Pausiert, solange der Papierkorb aus ist.",
		"Nothing to clean up now.":                 "Gerade nichts aufzuräumen.",
		"cleanup is paused while the trash is off": "Aufräumen ist pausiert, solange der Papierkorb aus ist",
		"The trash is off, so cleanup is paused: nothing is moved or deleted until the trash is on again.":                                                                                                                     "Der Papierkorb ist aus, daher ist das Aufräumen pausiert: Nichts wird verschoben oder gelöscht, bis der Papierkorb wieder an ist.",
		"Overrides the tenant's cleanup defaults for this project. Inherit follows the Tenant screen.":                                                                                                                         "Überschreibt die Aufräum-Standards des Mandanten für dieses Projekt. Erben folgt der Seite „Mandant“.",
		"Cleans up finished tasks and the trash on its own, on the same schedule as retention. These are the defaults for every project; a project can override each one in its settings. Everything is off until you set it.": "Räumt abgeschlossene Aufgaben und den Papierkorb selbstständig auf, im selben Rhythmus wie die Aufbewahrung. Das sind die Standards für jedes Projekt; ein Projekt kann jeden davon in seinen Einstellungen überschreiben. Alles ist aus, bis Sie es setzen.",
		"Tasks that have sat in a done state this many days go to the trash, where they can be restored. A done task with open subtasks stays until they are finished too. Cannot be turned on while the trash is off.":        "Aufgaben, die so viele Tage in einem erledigten Zustand standen, kommen in den Papierkorb, wo sie wiederhergestellt werden können. Eine erledigte Aufgabe mit offenen Unteraufgaben bleibt, bis auch diese abgeschlossen sind. Kann nicht eingeschaltet werden, solange der Papierkorb aus ist.",
		"Tasks that have sat in a cancelled state this many days go to the trash, where they can be restored. Set apart from done tasks, because abandoned work is usually worth less to keep.":                                "Aufgaben, die so viele Tage in einem abgebrochenen Zustand standen, kommen in den Papierkorb, wo sie wiederhergestellt werden können. Getrennt von erledigten Aufgaben, weil aufgegebene Arbeit meist weniger aufbewahrenswert ist.",
		"Tasks deleted more than this many days ago are deleted for good, subtasks first. This cannot be undone.":                                                                                                              "Aufgaben, die vor mehr als so vielen Tagen gelöscht wurden, werden endgültig gelöscht, Unteraufgaben zuerst. Das kann nicht rückgängig gemacht werden.",
	})
}
