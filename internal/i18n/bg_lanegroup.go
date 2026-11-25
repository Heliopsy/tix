// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Group lanes by":        "Ленти по",
		"Unassigned":            "Без изпълнител",
		"No tag":                "Без етикет",
		"Show only these tasks": "Покажи само тези задачи",
		"A task with several tags is drawn once, in the first lane among its tags. Its card lists them all.":                                                                                                                                              "Задача с няколко етикета се показва веднъж, в първата лента сред нейните етикети. Картата ѝ ги изброява всички.",
		"Each lane is a project, a person, a priority, a tag or a deadline. A task with several tags is drawn once, in the first lane among its tags. The choice is in the address, so a link keeps it; the task list also remembers it in this browser.": "Всяка лента е проект, човек, приоритет, етикет или краен срок. Задача с няколко етикета се показва веднъж, в първата лента сред нейните етикети. Изборът е в адреса, така че връзката го пази; списъкът със задачи го помни и в този браузър.",
	})
}
