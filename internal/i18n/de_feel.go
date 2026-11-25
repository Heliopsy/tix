// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The words acting in place adds: the toast's Undo and what the page says
// when an action it already drew is refused or cannot be sent.
func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Undo": "Rückgängig",
		"That did not work. Nothing was changed.":          "Das hat nicht funktioniert. Nichts wurde geändert.",
		"Could not reach the server. Nothing was changed.": "Der Server war nicht erreichbar. Nichts wurde geändert.",
	})
}
