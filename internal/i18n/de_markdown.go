// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Preview":             "Vorschau",
		"Write":               "Schreiben",
		"Loading preview…":    "Vorschau wird geladen…",
		"Nothing to preview.": "Nichts für die Vorschau.",
		"The preview could not be drawn. Your text is unchanged.": "Die Vorschau konnte nicht erstellt werden. Ihr Text ist unverändert.",
		"Checklist":                     "Checkliste",
		"%d of %d checklist items done": "%d von %d Checklisteneinträgen erledigt",
		"item ticked":                   "Eintrag abgehakt",
		"item cleared":                  "Haken entfernt",
		"Written in Markdown: **bold**, lists, tables, `code`, links, and - [ ] for a checklist the task screen can tick. A reference such as INFRA-8 and an @handle become links. HTML is shown as typed, and images are linked rather than loaded.": "In Markdown geschrieben: **fett**, Listen, Tabellen, `Code`, Links und - [ ] für eine Checkliste, die auf der Aufgabenseite abgehakt werden kann. Eine Referenz wie INFRA-8 und ein @handle werden zu Links. HTML wird wie eingegeben angezeigt, und Bilder werden verlinkt statt geladen.",
	})
}
