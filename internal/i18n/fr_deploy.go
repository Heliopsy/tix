// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The words a tab says when the server it talks to was upgraded under it.
func init() {
	register(French, map[string]string{
		"Updated to %s":               "Mis à jour vers %s",
		"A new version is available.": "Une nouvelle version est disponible.",
		"Reload":                      "Recharger",
	})
}
