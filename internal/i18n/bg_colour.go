// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// The shared colour control.
		"Custom":                    "Собствен",
		"Custom colour, as #rrggbb": "Собствен цвят, като #rrggbb",
		"Pick a custom colour":      "Изберете собствен цвят",
		"Preview":                   "Преглед",
		"Pick one of the predefined colours, or choose Custom and set any colour with the picker or as #rrggbb. Each colour scheme draws a custom colour as close to it as stays readable.":            "Изберете един от готовите цветове или „Собствен“ и задайте произволен цвят с инструмента за избор или като #rrggbb. Всяка цветова схема рисува собствения цвят възможно най-близо до него, докато остава четим.",
		"The accent this tenant is drawn in. Pick a predefined theme, or choose Custom and set any accent with the picker or as #rrggbb; its tint and its shade for dark schemes are derived from it.": "Акцентният цвят, с който се показва този тенант. Изберете готова тема или „Собствен“ и задайте произволен акцент с инструмента за избор или като #rrggbb; оттенъкът му и вариантът за тъмните схеми се извеждат от него.",
	})
}
