// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// Task templates.
		"Templates":                    "Шаблони",
		"Template":                     "Шаблон",
		"Template name":                "Име на шаблона",
		"Task templates":               "Шаблони на задачи",
		"Save as template":             "Запази като шаблон",
		"Save template":                "Запази шаблона",
		"Delete template":              "Изтрий шаблона",
		"New from template":            "Нова от шаблон",
		"From template":                "От шаблон",
		"All templates…":               "Всички шаблони…",
		"All projects":                 "Всички проекти",
		"This project":                 "Този проект",
		"Use in":                       "За",
		"Also keep":                    "Запази също",
		"Due in":                       "Срок след",
		"Use":                          "Използвай",
		"Uses":                         "Използвания",
		"Used %d times":                "Използван %d пъти",
		"Actions":                      "Действия",
		"Task title":                   "Заглавие на задачата",
		"The template's project":       "Проектът на шаблона",
		"The template's title":         "Заглавието на шаблона",
		"3d":                           "3d",
		"template saved":               "шаблонът е запазен",
		"template deleted":             "шаблонът е изтрит",
		"saved as a template":          "запазена като шаблон",
		"task created from a template": "задачата е създадена от шаблон",
		"No templates yet. Open a task and pick Save as template.": "Още няма шаблони. Отворете задача и изберете Запази като шаблон.",
		"Tasks already made from this template stay as they are.":  "Задачите, вече създадени от този шаблон, остават каквито са.",
		"Titles may carry placeholders, filled in each time a template is used: {DATE} is today's date, {N} the use number and {PROJECT} the project's key.":                                                                                                                                                                               "Заглавията може да съдържат заместители, попълвани при всяко използване на шаблона: {DATE} е днешната дата, {N} е поредният номер на използването, а {PROJECT} е ключът на проекта.",
		"A template is a named shape of a task: title, description, priority, tags, custom fields, repeat rule and, when saved with them, subtasks. It is not a task, so it never shows in a listing, a claim or the statistics. Save one from a task's Save as template, and create tasks from it with New from template in the add box.": "Шаблонът е именувана форма на задача: заглавие, описание, приоритет, етикети, потребителски полета, правило за повторение и, ако са запазени с него, подзадачи. Той не е задача, затова никога не се показва в списък, в заявка или в статистиката. Запазете шаблон от „Запази като шаблон“ на задача и създавайте задачи от него с „Нова от шаблон“ в полето за добавяне.",
		"Changes apply to tasks made from now on; tasks already made from the template keep what they were made with. Subtasks change only by saving a task as a template again.":                                                                                                                                                          "Промените важат за задачите, създадени оттук нататък; вече създадените от шаблона задачи запазват това, с което са създадени. Подзадачите се променят само като се запази отново задача като шаблон.",
		"Creates a task, and a subtask for each one the template holds. It starts in the project's first state and keeps the custom fields that project defines.":                                                                                                                                                                          "Създава задача и по една подзадача за всяка, която шаблонът съдържа. Тя започва в първото състояние на проекта и запазва потребителските полета, които проектът определя.",
		"The template keeps this task's title, description, priority, tags, custom fields and repeat rule, and its subtasks when ticked. Status, assignee, due date, comments and history stay with the task. Templates never show in a listing; find them under Templates.":                                                               "Шаблонът запазва заглавието, описанието, приоритета, етикетите, потребителските полета и правилото за повторение на задачата, а и подзадачите ѝ, ако е отметнато. Състоянието, изпълнителят, срокът, коментарите и историята остават при задачата. Шаблоните никога не се показват в списък; ще ги намерите в Шаблони.",
		"Each task made from the template falls due this long after it is made, such as 3d or 12h. Leave it empty for no due date.":                                                                                                                                                                                                        "Всяка задача от шаблона има срок толкова време след създаването си, например 3d или 12h. Оставете празно за задача без срок.",
		"How long from now the task falls due, such as 3d or 12h. Leave it empty for the template's own offset; 0 sets no due date.":                                                                                                                                                                                                       "След колко време от сега е срокът на задачата, например 3d или 12h. Оставете празно за срока на шаблона; 0 означава без срок.",
		"Leave it empty for the template's own title. Placeholders work here too: {DATE} is today's date, {N} the use number and {PROJECT} the project's key.":                                                                                                                                                                             "Оставете празно за заглавието на шаблона. Заместителите работят и тук: {DATE} е днешната дата, {N} е поредният номер на използването, а {PROJECT} е ключът на проекта.",
		"One key=value per line. A line removed removes the field. The project a task lands in keeps the fields it defines and drops the rest.":                                                                                                                                                                                            "По едно ключ=стойност на ред. Премахнат ред премахва полето. Проектът, в който попада задачата, запазва полетата, които определя, и пропуска останалите.",
		"Placeholders are filled in each time the template is used, in any case: {DATE} is today's date as YYYY-MM-DD, {N} the use number and {PROJECT} the key of the project the task lands in.":                                                                                                                                         "Заместителите се попълват при всяко използване на шаблона, независимо от регистъра: {DATE} е днешната дата във формат ГГГГ-ММ-ДД, {N} е поредният номер на използването, а {PROJECT} е ключът на проекта, в който попада задачата.",
	})
}
