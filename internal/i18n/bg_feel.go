// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The words acting in place adds: the toast's Undo and what the page says
// when an action it already drew is refused or cannot be sent.
func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Undo": "Отмени",
		"That did not work. Nothing was changed.":          "Не се получи. Нищо не е променено.",
		"Could not reach the server. Nothing was changed.": "Няма връзка със сървъра. Нищо не е променено.",
	})
}
