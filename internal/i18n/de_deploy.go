// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The words a tab says when the server it talks to was upgraded under it.
func init() {
	register(German, map[string]string{
		"Updated to %s":               "Aktualisiert auf %s",
		"A new version is available.": "Eine neue Version ist verfügbar.",
		"Reload":                      "Neu laden",
	})
}
