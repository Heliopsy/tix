// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// The open task count beside each project on the projects screen.
		"%d open task":  "%d offene Aufgabe",
		"%d open tasks": "%d offene Aufgaben",
		"no open tasks": "keine offenen Aufgaben",
	})
}
