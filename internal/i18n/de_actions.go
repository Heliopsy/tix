// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Revoke %s?":       "%s widerrufen?",
		"Remove %s?":       "%s entfernen?",
		"Archive %s?":      "%s archivieren?",
		"Clean up %s now?": "%s jetzt aufräumen?",
		"Settings for %s":  "Einstellungen für %s",
		"Nothing is subscribed to this tenant's events.":                                     "Nichts ist für die Ereignisse dieses Mandanten abonniert.",
		"Which key each scheme uses for each action.":                                        "Welche Taste jedes Schema für jede Aktion verwendet.",
		"Archiving hides %s from active lists and keeps its tasks.":                          "Archivieren blendet %s in aktiven Listen aus und behält die Aufgaben.",
		"Deleting removes the project for good. You choose what happens to its tasks first.": "Löschen entfernt das Projekt endgültig. Zuerst wählen Sie, was mit seinen Aufgaben geschieht.",
		"Deleting removes the account and its membership. It cannot be undone.":              "Löschen entfernt das Konto und seine Mitgliedschaft. Das kann nicht rückgängig gemacht werden.",
		"What this project is called and how it looks, how long a claim on its tasks holds, how it cleans up its finished tasks and the trash, and archiving or deleting it.":                                                                                                                                                                                                                                      "Wie dieses Projekt heißt und aussieht, wie lange eine Übernahme seiner Aufgaben gilt, wie es erledigte Aufgaben und den Papierkorb aufräumt, sowie Archivieren oder Löschen.",
		"A row appears when an import <strong>finishes</strong>, read back from the audit entry it wrote. A <strong>dry run</strong> writes nothing, so it leaves no row, and a <strong>failed</strong> run stops without writing one: it shows as its source's status in the second table. A row cannot show where the cursor stood after its run; the second table shows where each cursor stands <em>now</em>.": "Eine Zeile erscheint, wenn ein Import <strong>abgeschlossen</strong> ist, gelesen aus dem Audit-Eintrag, den er geschrieben hat. Ein <strong>Probelauf</strong> schreibt nichts und hinterlässt daher keine Zeile, und ein <strong>fehlgeschlagener</strong> Lauf bricht ab, ohne eine zu schreiben: Er erscheint als Status seiner Quelle in der zweiten Tabelle. Eine Zeile kann nicht zeigen, wo der Cursor nach ihrem Lauf stand; die zweite Tabelle zeigt, wo jeder Cursor <em>jetzt</em> steht.",
		"The tasks an import created or updated are audited one by one, in <a href=\"/activity?source=system\">activity</a>, attributed to the System actor.":                                                                                                                                                                                                                                                      "Die Aufgaben, die ein Import erstellt oder aktualisiert hat, werden einzeln protokolliert, unter <a href=\"/activity?source=system\">Aktivität</a>, dem Akteur System zugeordnet.",
	})
}
