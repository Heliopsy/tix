// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Actions for %s":              "Действия за %s",
		"Arranging columns and lanes": "Подредба на колони и ленти",
		"Collapse":                    "Свий",
		"Expand":                      "Разгъни",
		"Hide":                        "Скрий",
		"Hidden: %d":                  "Скрити: %d",
		"Hidden columns":              "Скрити колони",
		"Hidden lanes":                "Скрити ленти",
		"None hidden.":                "Няма скрити.",
		"Show %s":                     "Покажи %s",
		"Move left":                   "Премести наляво",
		"Move right":                  "Премести надясно",
		"Move up":                     "Премести нагоре",
		"Move down":                   "Премести надолу",
		"Star, to show it first":      "Със звезда, за да е първа",
		"Open the project":            "Отвори проекта",
		"Show only this project":      "Покажи само този проект",
		"Show only this status":       "Покажи само това състояние",
		"New task in %s":              "Нова задача в %s",
		"Reset the arrangement":       "Върни подредбата по подразбиране",
		"Group the list":              "Групирай списъка",
		"How to group the list":       "Как да се групира списъкът",
		"Grouping":                    "Групиране",
		"List groups":                 "Групи в списъка",
		"No groups":                   "Без групи",
		"By project":                  "По проект",
		"By status":                   "По състояние",
		"How the list on <a href=\"/tasks\">Tasks</a> groups its rows.": "Как списъкът в <a href=\"/tasks\">Задачи</a> групира редовете си.",
		"Draws a heading per project or per status over the rows of this page. Each heading has the same menu as a lane or a board column, and shares its arrangement.":                                                "Показва заглавие за всеки проект или състояние над редовете на тази страница. Всяко заглавие има същото меню като лента или колона на таблото и споделя нейната подредба.",
		"The three dots on a column, lane or group heading hide, star, collapse or move it. The arrangement is yours, kept with your account, so it follows you to any browser and changes nothing anybody else sees.": "Трите точки на колона, лента или заглавие на група я скриват, отбелязват със звезда, свиват или местят. Подредбата е ваша и се пази към профила ви, така че ви следва във всеки браузър и не променя нищо за другите.",
		"Whether the list draws a heading per project or per status over its rows. Each heading's menu hides, stars, collapses or moves the group, kept with your account.":                                            "Дали списъкът показва заглавие за всеки проект или състояние над редовете си. Менюто на всяко заглавие скрива, отбелязва със звезда, свива или мести групата и се пази към профила ви.",
	})
}
