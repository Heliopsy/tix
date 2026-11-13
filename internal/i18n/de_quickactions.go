// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// The quick actions on a task: Assign to me and the due date field in
		// the task, row and lane card menus, and the toasts they answer with.
		"Assign to me":       "Mir zuweisen",
		"Clear the due date": "Fälligkeitsdatum entfernen",
		"assigned to you":    "Ihnen zugewiesen",
		"assignee changed":   "Zuständigkeit geändert",
		"assignee cleared":   "Zuständigkeit entfernt",
		"due date set":       "Fälligkeitsdatum gesetzt",
		"due date cleared":   "Fälligkeitsdatum entfernt",
	})
}
