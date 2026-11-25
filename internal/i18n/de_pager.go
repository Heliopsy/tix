// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Page %d of %s": "Seite %d von %s",
		"First":         "Erste",
		"Last":          "Letzte",
		"Go to page":    "Zu Seite",
		"Jump":          "Springen",
	})
}
