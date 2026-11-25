// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The dialog a history row's Raw control opens: one audit entry as recorded.
func init() {
	register(German, map[string]string{
		"Raw entry":                        "Rohdaten des Eintrags",
		"Entry":                            "Eintrag",
		"Step %d of %d":                    "Schritt %d von %d",
		"Actor":                            "Akteur",
		"Subject":                          "Gegenstand",
		"Time":                             "Zeit",
		"Sequence":                         "Sequenz",
		"system":                           "System",
		"Changed fields":                   "Geänderte Felder",
		"Field":                            "Feld",
		"Before":                           "Vorher",
		"After":                            "Nachher",
		"added":                            "hinzugefügt",
		"removed":                          "entfernt",
		"(none)":                           "(keine)",
		"This entry recorded no snapshot.": "Dieser Eintrag enthält keinen Snapshot.",
		"Copy all as JSON":                 "Alles als JSON kopieren",
	})
}
