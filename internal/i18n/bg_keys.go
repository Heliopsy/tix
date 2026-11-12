// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The keyboard shortcuts the web shares with the app: the help overlay's
// groups and the actions added with the shared key table.
func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Navigate": "Навигация",
		"Lists":    "Списъци",
		"Task":     "Задача",
		"General":  "Общи",

		"Go to Inbox":      "Към Входящи",
		"Go to My work":    "Към Моята работа",
		"Go to Tasks":      "Към Задачи",
		"Go to Projects":   "Към Проекти",
		"Go to Activity":   "Към Активност",
		"Go to Settings":   "Към Настройки",
		"Go to Statistics": "Към Статистика",

		"Page down":   "Страница надолу",
		"Page up":     "Страница нагоре",
		"Refresh":     "Обнови",
		"Switch view": "Смени изгледа",

		"Mark done":              "Отбележи като готова",
		"Edit the description":   "Редактирай описанието",
		"Move to another status": "Премести в друг статус",
		"Star or unstar":         "Добави или махни звезда",
		"Set the priority":       "Задай приоритет",
		"Set the due date":       "Задай краен срок",
		"Snooze or unsnooze":     "Отложи или върни сега",
		"Copy the reference":     "Копирай референцията",
		"Copied %s":              "Копирано: %s",
	})
}
