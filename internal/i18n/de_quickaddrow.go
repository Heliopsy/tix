// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Quick add suggestions":        "Vorschläge für Schnell hinzufügen",
		"Quick add from the clipboard": "Schnell hinzufügen aus der Zwischenablage",
		"The clipboard holds no text.": "Die Zwischenablage enthält keinen Text.",
		"Stay open for the next line":  "Für die nächste Zeile geöffnet bleiben",
		"Add another":                  "Weitere hinzufügen",
	})
}
