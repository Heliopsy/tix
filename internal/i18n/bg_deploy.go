// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The words a tab says when the server it talks to was upgraded under it.
func init() {
	register(Bulgarian, map[string]string{
		"Updated to %s":               "Обновено до %s",
		"A new version is available.": "Има нова версия.",
		"Reload":                      "Презареди",
	})
}
