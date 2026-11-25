// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Page %d of %s": "Page %d sur %s",
		"First":         "Première",
		"Last":          "Dernière",
		"Go to page":    "Aller à la page",
		"Jump":          "Aller",
	})
}
