// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The words acting in place adds: the toast's Undo and what the page says
// when an action it already drew is refused or cannot be sent.
func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Undo": "Annuler",
		"That did not work. Nothing was changed.":          "Cela n’a pas fonctionné. Rien n’a été modifié.",
		"Could not reach the server. Nothing was changed.": "Impossible de joindre le serveur. Rien n’a été modifié.",
	})
}
