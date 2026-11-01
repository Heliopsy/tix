// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The command palette: its dialog, the opener, the group names and the
// actions it offers.
func init() {
	register(Bulgarian, map[string]string{
		"Open the command palette": "Отвори командната палитра",
		"Command palette":          "Командна палитра",
		"Search":                   "Търсене",
		"Search tasks, projects, screens and actions": "Търси задачи, проекти, екрани и действия",
		"Results": "Резултати",
		"<kbd>↑</kbd> <kbd>↓</kbd> to move, <kbd>Enter</kbd> to open, <kbd>Esc</kbd> to close": "<kbd>↑</kbd> <kbd>↓</kbd> за движение, <kbd>Enter</kbd> за отваряне, <kbd>Esc</kbd> за затваряне",
		"New task in %s":                  "Нова задача в %s",
		"Recent":                          "Скорошни",
		"Actions":                         "Действия",
		"Screens":                         "Екрани",
		"Searching tasks":                 "Търсене на задачи",
		"The task search did not answer.": "Търсенето на задачи не отговори.",
		"No matches":                      "Няма съвпадения",
		"Open the filter":                 "Отвори филтъра",
		"Show tasks as a list":            "Покажи задачите като списък",
		"Show tasks as a board":           "Покажи задачите като табло",
		"Show tasks in lanes":             "Покажи задачите в ленти",
		"Switch the colour scheme":        "Смени цветовата схема",
	})
}
