// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// How a project is named in a picker.
		"Project names in pickers":                  "Projektnamen in Auswahllisten",
		"How a project reads where you choose one.": "So erscheint ein Projekt dort, wo Sie es auswählen.",
		"How a project reads in every list you choose one from: the add box, Add with details, Copy to, New from template, a heading's New task in, the move target when a project is deleted, and the statistics filter. The name with its key, the key first, the name alone, or the key alone. A long name is shortened; the key never is.": "So erscheint ein Projekt in jeder Liste, aus der Sie es auswählen: im Eingabefeld, unter Mit Details hinzufügen, Kopieren nach, Neu aus Vorlage, Neue Aufgabe in einer Überschrift, als Ziel beim Löschen eines Projekts und im Statistikfilter. Der Name mit Schlüssel, zuerst der Schlüssel, nur der Name oder nur der Schlüssel. Ein langer Name wird gekürzt, der Schlüssel nie.",
		"Apply project names": "Projektnamen übernehmen",
		"Name (KEY)":          "Name (SCHLÜSSEL)",
		"KEY · Name":          "SCHLÜSSEL · Name",
		"Name":                "Name",
		"KEY":                 "SCHLÜSSEL",
	})
}
