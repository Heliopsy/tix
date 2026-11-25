// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// The open task count beside each project on the projects screen.
		"%d open task":  "%d tâche ouverte",
		"%d open tasks": "%d tâches ouvertes",
		"no open tasks": "aucune tâche ouverte",
	})
}
