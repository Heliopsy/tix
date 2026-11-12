// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// The trash: the task list filtered by is:trashed.
		"Trash":            "Papierkorb",
		"%d deleted task":  "%d gelöschte Aufgabe",
		"%d deleted tasks": "%d gelöschte Aufgaben",
		"Restore one to put it back where it was.":              "Stellen Sie eine wieder her, um sie an ihren alten Platz zurückzubringen.",
		"The trash is empty.":                                   "Der Papierkorb ist leer.",
		"The trash is empty":                                    "Der Papierkorb ist leer",
		"A deleted task waits here until somebody restores it.": "Eine gelöschte Aufgabe wartet hier, bis jemand sie wiederherstellt.",
		"Deleted %s":             "Gelöscht %s",
		"deleted":                "gelöscht",
		"Restore %s":             "%s wiederherstellen",
		"Restore":                "Wiederherstellen",
		"Delete %s permanently":  "%s endgültig löschen",
		"Delete %s permanently?": "%s endgültig löschen?",
		"%s and its subtasks in the trash are deleted for good, with their comments, links and attachments. It cannot be undone; the audit log keeps the record.": "%s und die zugehörigen Unteraufgaben im Papierkorb werden endgültig gelöscht, mit ihren Kommentaren, Links und Anhängen. Das lässt sich nicht rückgängig machen; das Audit-Log behält den Eintrag.",

		// A tenant whose trash is turned off: the confirmation that stands in
		// for the undo, and the setting on the Tenant screen.
		"task deleted permanently":    "Aufgabe endgültig gelöscht",
		"Delete %d task permanently":  "%d Aufgabe endgültig löschen",
		"Delete %d tasks permanently": "%d Aufgaben endgültig löschen",
		"Trash is off":                "Papierkorb ist aus",
		"This tenant's trash is turned off, so a deleted task is removed for good: it cannot be restored and there is no undo. A tenant admin can turn the trash back on from the Tenant screen.": "Der Papierkorb dieses Mandanten ist ausgeschaltet, daher wird eine gelöschte Aufgabe endgültig entfernt: Sie kann nicht wiederhergestellt werden, und es gibt kein Rückgängig. Ein Mandanten-Admin kann den Papierkorb im Bildschirm Mandant wieder einschalten.",
		"Delete %s?": "%s löschen?",
		"The trash is turned off for this tenant, so this cannot be undone.": "Der Papierkorb ist für diesen Mandanten ausgeschaltet, daher lässt sich dies nicht rückgängig machen.",
		"Their subtasks are deleted with them.":                              "Ihre Unteraufgaben werden mit ihnen gelöscht.",
		"On: deleted tasks can be restored":                                  "An: gelöschte Aufgaben können wiederhergestellt werden",
		"Off: deleting is permanent":                                         "Aus: Löschen ist endgültig",
		"On, a deleted task goes to the Trash, where anyone who could delete it can restore it, and every delete offers Undo. Off, deleting a task removes it for good: each delete asks first, nothing can be restored, and the Trash leaves the sidebar. Tasks already in the trash when it is turned off are kept, out of reach, and come back when it is turned on again.": "An: Eine gelöschte Aufgabe kommt in den Papierkorb, wo jeder, der sie löschen konnte, sie wiederherstellen kann, und jedes Löschen bietet Rückgängig an. Aus: Löschen entfernt eine Aufgabe endgültig, jedes Löschen fragt vorher nach, nichts kann wiederhergestellt werden, und der Papierkorb verschwindet aus der Seitenleiste. Aufgaben, die beim Ausschalten bereits im Papierkorb liegen, bleiben unerreichbar erhalten und kommen beim Wiedereinschalten zurück.",
		"%d task deleted while the trash was on is kept out of reach until it is turned back on.":                 "%d Aufgabe, die bei eingeschaltetem Papierkorb gelöscht wurde, bleibt unerreichbar, bis er wieder eingeschaltet wird.",
		"%d tasks deleted while the trash was on are kept out of reach until it is turned back on.":               "%d Aufgaben, die bei eingeschaltetem Papierkorb gelöscht wurden, bleiben unerreichbar, bis er wieder eingeschaltet wird.",
		"%d task is in the trash. Turning the trash off keeps it, out of reach, until the trash is on again.":     "%d Aufgabe liegt im Papierkorb. Beim Ausschalten des Papierkorbs bleibt sie unerreichbar erhalten, bis er wieder eingeschaltet wird.",
		"%d tasks are in the trash. Turning the trash off keeps them, out of reach, until the trash is on again.": "%d Aufgaben liegen im Papierkorb. Beim Ausschalten des Papierkorbs bleiben sie unerreichbar erhalten, bis er wieder eingeschaltet wird.",
	})
}
