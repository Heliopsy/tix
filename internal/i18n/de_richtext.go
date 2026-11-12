// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Editor":         "Editor",
		"Rich text":      "Rich Text",
		"Markdown":       "Markdown",
		"Formatting":     "Formatierung",
		"Bold":           "Fett",
		"Italic":         "Kursiv",
		"Strikethrough":  "Durchgestrichen",
		"Heading":        "Überschrift",
		"Bulleted list":  "Aufzählung",
		"Numbered list":  "Nummerierte Liste",
		"Quote":          "Zitat",
		"Link":           "Link",
		"Table":          "Tabelle",
		"Add row":        "Zeile hinzufügen",
		"Add column":     "Spalte hinzufügen",
		"Delete row":     "Zeile löschen",
		"Delete column":  "Spalte löschen",
		"Delete table":   "Tabelle löschen",
		"Link address":   "Linkadresse",
		"Add link":       "Link hinzufügen",
		"Checklist item": "Checklisteneintrag",
		"The rich text editor could not be loaded. Your text is unchanged.":                          "Der Rich-Text-Editor konnte nicht geladen werden. Ihr Text ist unverändert.",
		"This text uses Markdown the rich text editor cannot keep exactly, so it stays in Markdown.": "Dieser Text verwendet Markdown, das der Rich-Text-Editor nicht exakt erhalten kann, daher bleibt er in Markdown.",
	})
}
