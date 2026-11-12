// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// Task templates.
		"Templates":                    "Vorlagen",
		"Template":                     "Vorlage",
		"Template name":                "Name der Vorlage",
		"Task templates":               "Aufgabenvorlagen",
		"Save as template":             "Als Vorlage speichern",
		"Save template":                "Vorlage speichern",
		"Delete template":              "Vorlage löschen",
		"New from template":            "Neu aus Vorlage",
		"From template":                "Aus Vorlage",
		"All templates…":               "Alle Vorlagen…",
		"All projects":                 "Alle Projekte",
		"This project":                 "Dieses Projekt",
		"Use in":                       "Verwenden in",
		"Also keep":                    "Auch behalten",
		"Due in":                       "Fällig in",
		"Use":                          "Verwenden",
		"Uses":                         "Verwendungen",
		"Used %d times":                "%d-mal verwendet",
		"Actions":                      "Aktionen",
		"Task title":                   "Aufgabentitel",
		"The template's project":       "Das Projekt der Vorlage",
		"The template's title":         "Der Titel der Vorlage",
		"3d":                           "3d",
		"template saved":               "Vorlage gespeichert",
		"template deleted":             "Vorlage gelöscht",
		"saved as a template":          "als Vorlage gespeichert",
		"task created from a template": "Aufgabe aus Vorlage erstellt",
		"No templates yet. Open a task and pick Save as template.": "Noch keine Vorlagen. Öffnen Sie eine Aufgabe und wählen Sie Als Vorlage speichern.",
		"Tasks already made from this template stay as they are.":  "Bereits aus dieser Vorlage erstellte Aufgaben bleiben, wie sie sind.",
		"Titles may carry placeholders, filled in each time a template is used: {DATE} is today's date, {N} the use number and {PROJECT} the project's key.":                                                                                                                                                                               "Titel können Platzhalter enthalten, die bei jeder Verwendung einer Vorlage ausgefüllt werden: {DATE} ist das heutige Datum, {N} die Nummer der Verwendung und {PROJECT} der Schlüssel des Projekts.",
		"A template is a named shape of a task: title, description, priority, tags, custom fields, repeat rule and, when saved with them, subtasks. It is not a task, so it never shows in a listing, a claim or the statistics. Save one from a task's Save as template, and create tasks from it with New from template in the add box.": "Eine Vorlage ist die benannte Form einer Aufgabe: Titel, Beschreibung, Priorität, Tags, benutzerdefinierte Felder, Wiederholungsregel und, wenn mit ihnen gespeichert, Unteraufgaben. Sie ist keine Aufgabe und erscheint daher nie in einer Liste, einer Übernahme oder der Statistik. Speichern Sie eine über Als Vorlage speichern einer Aufgabe und erstellen Sie Aufgaben daraus mit Neu aus Vorlage im Eingabefeld.",
		"Changes apply to tasks made from now on; tasks already made from the template keep what they were made with. Subtasks change only by saving a task as a template again.":                                                                                                                                                          "Änderungen gelten für ab jetzt erstellte Aufgaben; bereits aus der Vorlage erstellte Aufgaben behalten, womit sie erstellt wurden. Unteraufgaben ändern sich nur, wenn eine Aufgabe erneut als Vorlage gespeichert wird.",
		"Creates a task, and a subtask for each one the template holds. It starts in the project's first state and keeps the custom fields that project defines.":                                                                                                                                                                          "Erstellt eine Aufgabe und für jede Unteraufgabe der Vorlage eine Unteraufgabe. Sie beginnt im ersten Zustand des Projekts und behält die benutzerdefinierten Felder, die dieses Projekt definiert.",
		"The template keeps this task's title, description, priority, tags, custom fields and repeat rule, and its subtasks when ticked. Status, assignee, due date, comments and history stay with the task. Templates never show in a listing; find them under Templates.":                                                               "Die Vorlage übernimmt Titel, Beschreibung, Priorität, Tags, benutzerdefinierte Felder und Wiederholungsregel dieser Aufgabe sowie, wenn angekreuzt, ihre Unteraufgaben. Status, Zuständigkeit, Fälligkeitsdatum, Kommentare und Verlauf bleiben bei der Aufgabe. Vorlagen erscheinen nie in einer Liste; Sie finden sie unter Vorlagen.",
		"Each task made from the template falls due this long after it is made, such as 3d or 12h. Leave it empty for no due date.":                                                                                                                                                                                                        "Jede aus der Vorlage erstellte Aufgabe wird so lange nach ihrer Erstellung fällig, etwa 3d oder 12h. Leer lassen für kein Fälligkeitsdatum.",
		"How long from now the task falls due, such as 3d or 12h. Leave it empty for the template's own offset; 0 sets no due date.":                                                                                                                                                                                                       "Wie lange ab jetzt bis zur Fälligkeit der Aufgabe, etwa 3d oder 12h. Leer lassen für den eigenen Versatz der Vorlage; 0 setzt kein Fälligkeitsdatum.",
		"Leave it empty for the template's own title. Placeholders work here too: {DATE} is today's date, {N} the use number and {PROJECT} the project's key.":                                                                                                                                                                             "Leer lassen für den eigenen Titel der Vorlage. Platzhalter funktionieren auch hier: {DATE} ist das heutige Datum, {N} die Nummer der Verwendung und {PROJECT} der Schlüssel des Projekts.",
		"One key=value per line. A line removed removes the field. The project a task lands in keeps the fields it defines and drops the rest.":                                                                                                                                                                                            "Ein key=value pro Zeile. Eine entfernte Zeile entfernt das Feld. Das Projekt, in dem eine Aufgabe landet, behält die Felder, die es definiert, und verwirft den Rest.",
		"Placeholders are filled in each time the template is used, in any case: {DATE} is today's date as YYYY-MM-DD, {N} the use number and {PROJECT} the key of the project the task lands in.":                                                                                                                                         "Platzhalter werden bei jeder Verwendung der Vorlage ausgefüllt, in beliebiger Schreibweise: {DATE} ist das heutige Datum als YYYY-MM-DD, {N} die Nummer der Verwendung und {PROJECT} der Schlüssel des Projekts, in dem die Aufgabe landet.",
	})
}
