// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The command palette: its dialog, the opener, the group names and the
// actions it offers.
func init() {
	register(German, map[string]string{
		"Open the command palette": "Befehlspalette öffnen",
		"Command palette":          "Befehlspalette",
		"Search":                   "Suchen",
		"Search tasks, projects, screens and actions": "Aufgaben, Projekte, Seiten und Aktionen suchen",
		"Results": "Ergebnisse",
		"<kbd>↑</kbd> <kbd>↓</kbd> to move, <kbd>Enter</kbd> to open, <kbd>Esc</kbd> to close": "<kbd>↑</kbd> <kbd>↓</kbd> zum Bewegen, <kbd>Enter</kbd> zum Öffnen, <kbd>Esc</kbd> zum Schließen",
		"New task in %s":                   "Neue Aufgabe in %s",
		"Recent":                           "Zuletzt verwendet",
		"Actions":                          "Aktionen",
		"Screens":                          "Seiten",
		"Searching tasks":                  "Aufgaben werden gesucht",
		"The task search did not respond.": "Die Aufgabensuche hat nicht geantwortet.",
		"No matches":                       "Keine Treffer",
		"Open the filter":                  "Filter öffnen",
		"Show tasks as a list":             "Aufgaben als Liste anzeigen",
		"Show tasks as a board":            "Aufgaben als Board anzeigen",
		"Show tasks in lanes":              "Aufgaben in Bahnen anzeigen",
		"Switch the colour scheme":         "Farbschema wechseln",
	})
}
