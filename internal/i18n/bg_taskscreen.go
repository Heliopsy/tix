// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Properties":                      "Свойства",
		"Claim":                           "Поемане",
		"Created by":                      "Създадена от",
		"Snoozed":                         "Отложена",
		"Snooze…":                         "Отложи…",
		"Reclaim the lease…":              "Отнеми резервацията…",
		"Does not repeat":                 "Не се повтаря",
		"No subtasks.":                    "Няма подзадачи.",
		"Waits for nothing.":              "Не чака нищо.",
		"Depends on, for example infra-2": "Зависи от, например infra-2",
		"Remove the dependency on %s":     "Премахни зависимостта от %s",
		"in":                              "в",
	})
}
