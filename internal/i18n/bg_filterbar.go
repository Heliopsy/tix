// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Filter, for example tag:ops due:week @raj": "Филтър, например tag:ops due:week @raj",
		"Filters":                "Филтри",
		"a handle, or me":        "потребител или me",
		"add a tag":              "добавете етикет",
		"any status":             "всяко състояние",
		"as typed":               "както е написано",
		"any priority":           "всеки приоритет",
		"Include finished tasks": "Включи и завършените задачи",
		"Sort by":                "Подреди по",
		"Order":                  "Посока",
		"ascending":              "възходящо",
		"descending":             "низходящо",
		"More":                   "Още",
		"%d per page (default)":  "%d на страница (по подразбиране)",
		"%d per page":            "%d на страница",
		"Rename, share, reorder or delete them in the sidebar, under Tasks.": "Преименувайте, споделяйте, пренареждайте или изтривайте ги в страничната лента, под Задачи.",
		"Filter the list, then save it here.":                                "Филтрирайте списъка, после го запазете тук.",
		"%d filter in force":                                                 "%d активен филтър",
		"%d filters in force":                                                "%d активни филтъра",
		"Filters in force":                                                   "Активни филтри",
		"including finished":                                                 "с завършените",
		"Apply page size":                                                    "Приложи размера на страницата",
		"Filter suggestions":                                                 "Предложения за филтъра",
	})
}
