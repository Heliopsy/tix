// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Page %d of %s": "Страница %d от %s",
		"First":         "Първа",
		"Last":          "Последна",
		"Go to page":    "Към страница",
		"Jump":          "Отиди",
	})
}
