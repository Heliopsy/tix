// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"My work":                           "Моята работа",
		"Include tasks I hold":              "Включи и задачите, които държа",
		"Hidden groups":                     "Скрити групи",
		"Overdue":                           "Просрочени",
		"Today":                             "Днес",
		"This week":                         "Тази седмица",
		"Later":                             "По-късно",
		"No date":                           "Без срок",
		"the first %d by deadline":          "първите %d по срок",
		"see them all":                      "виж всички",
		"Save this filter":                  "Запази този филтър",
		"For example, my overdue work":      "Например, моите просрочени",
		"Share with everyone in the tenant": "Сподели с всички в тенанта",
		"Saved filters":                     "Запазени филтри",
		"shared":                            "споделен",
		"Stop sharing":                      "Спри споделянето",
		"Recently viewed":                   "Наскоро отваряни",
		"filter saved":                      "филтърът е запазен",
		"filter removed":                    "филтърът е премахнат",
		"Tasks assigned to you that are still open, under a heading per deadline: overdue, today, this week, later and no date. Include tasks I hold adds the ones you have claimed. The same list is tix task ls --mine, or assignee:me in any filter.": "Отворените задачи, възложени на вас, под заглавие за всеки срок: просрочени, днес, тази седмица, по-късно и без срок. „Включи и задачите, които държа“ добавя и поетите от вас. Същият списък е tix task ls --mine или assignee:me във всеки филтър.",
		"A saved filter keeps this filter, its sort and the view under a name in the sidebar, under Tasks. It is yours; a shared one is listed for everybody in the tenant, and only you or an administrator can change it.":                             "Запазеният филтър пази този филтър, подредбата и изгледа под име в страничната лента, под Задачи. Той е ваш; споделеният се показва на всички в тенанта и само вие или администратор можете да го променяте.",
	})
}
