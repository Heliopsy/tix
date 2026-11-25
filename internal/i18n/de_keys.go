// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The keyboard shortcuts the web shares with the app: the help overlay's
// groups and the actions added with the shared key table.
func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Navigate": "Navigation",
		"Lists":    "Listen",
		"Task":     "Aufgabe",
		"General":  "Allgemein",

		"Go to Inbox":      "Zum Posteingang",
		"Go to My work":    "Zu Meine Arbeit",
		"Go to Tasks":      "Zu Aufgaben",
		"Go to Projects":   "Zu Projekte",
		"Go to Activity":   "Zu Aktivität",
		"Go to Settings":   "Zu Einstellungen",
		"Go to Statistics": "Zu Statistik",

		"Page down":   "Seite nach unten",
		"Page up":     "Seite nach oben",
		"Refresh":     "Aktualisieren",
		"Switch view": "Ansicht wechseln",

		"Mark done":              "Als erledigt markieren",
		"Edit the description":   "Beschreibung bearbeiten",
		"Move to another status": "In einen anderen Status verschieben",
		"Star or unstar":         "Zu Favoriten hinzufügen oder entfernen",
		"Set the priority":       "Priorität festlegen",
		"Set the due date":       "Fälligkeitsdatum festlegen",
		"Snooze or unsnooze":     "Zurückstellen oder nicht mehr zurückstellen",
		"Copy the reference":     "Referenz kopieren",
		"Copied %s":              "%s kopiert",
	})
}
