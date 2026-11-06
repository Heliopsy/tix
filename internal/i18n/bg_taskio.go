// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Export":                            "Експорт",
		"Export tasks of %s":                "Експорт на задачите на %s",
		"Download":                          "Изтегли",
		"Dates":                             "Дати",
		"ISO 8601, for programs":            "ISO 8601, за програми",
		"Day only, YYYY-MM-DD":              "Само ден, ГГГГ-ММ-ДД",
		"As I read them, for a spreadsheet": "Както ги чета, за електронна таблица",
		"CSV separator":                     "Разделител в CSV",
		"Include":                           "Включи",
		"Finished tasks":                    "Завършени задачи",
		"Tasks in the trash":                "Задачи в кошчето",
		"Move %s up":                        "Премести %s нагоре",
		"Move %s down":                      "Премести %s надолу",
		"tag:infra priority:high":           "tag:infra priority:high",
		"The same filter the task list takes. Empty exports every task of the project.":                                         "Същият филтър като в списъка със задачи. Празен изнася всички задачи на проекта.",
		"Ticked columns are written in this order. Move a column up or down with its arrows.":                                   "Отметнатите колони се записват в този ред. Преместете колона нагоре или надолу със стрелките ѝ.",
		"A CSV cell that starts with = + - or @ gets a leading quote, so a spreadsheet shows it as text instead of running it.": "Клетка в CSV, която започва с = + - или @, получава апостроф отпред, за да я покаже електронната таблица като текст, вместо да я изпълни.",
		"Identifier":                "Идентификатор",
		"Creator":                   "Създател",
		"Parent":                    "Родител",
		"Repeat":                    "Повторение",
		"Completed":                 "Завършена",
		"CSV, for spreadsheets":     "CSV, за електронни таблици",
		"JSON":                      "JSON",
		"NDJSON, one task per line": "NDJSON, по една задача на ред",
		"Markdown table":            "Таблица в Markdown",
		"Comma":                     "Запетая",
		"Semicolon":                 "Точка и запетая",
		"Tab":                       "Табулация",
		"Looking at one task?":      "Търсите една задача?",
		"Open it, or run tix task show REF -o json. No export needed.": "Отворете я или изпълнете tix task show REF -o json. Не е нужен експорт.",
		"Vertical bar": "Вертикална черта",
	})
}
