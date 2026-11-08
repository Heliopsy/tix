// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// Settings, Profile.
		"Profile": "Профил",
		"Who you are signed in as here, and with what authority.": "Като кого сте влезли тук и с какви права.",
		"Open your actor page": "Отворете страницата си",
		"The short name other people type to mention you or assign you work, unique within this tenant.": "Краткото име, с което другите ви споменават или ви възлагат работа. Уникално е в този наемател.",
		"Actor id":           "Идентификатор",
		"Copy your actor id": "Копирайте идентификатора си",
		"The identifier the command line, the API and the audit log use for you. It never changes, while a handle can.": "Идентификаторът, с който командният ред, API и одитният дневник ви назовават. Той не се променя, а краткото име може.",
		"Your role in this tenant. Its scopes add to any granted to you directly.":                                      "Ролята ви в този наемател. Правата ѝ се добавят към дадените ви пряко.",
		"Whether you may administer the whole installation, every tenant included, right now.":                          "Дали в момента можете да администрирате цялата инсталация, всички наематели включително.",
		"Signed in with": "Удостоверяване",
		"What this request authenticated with: the session this browser holds, or an API token.": "С какво е удостоверена тази заявка: сесията на този браузър или API токен.",
		"Browser session": "Сесия на браузъра",
		"Everything you may do here, from your role and from any scope granted to you directly.":                           "Всичко, което можете да правите тук, от ролята ви и от правата, дадени ви пряко.",
		"The address you sign in with. It belongs to your account, which is the same in every tenant you are a member of.": "Адресът, с който влизате. Принадлежи на акаунта ви, който е един и същ във всеки наемател, в който членувате.",
		"Every tenant your account is a member of, with your handle and role there.":                                       "Всеки наемател, в който членува акаунтът ви, с краткото ви име и ролята ви там.",
		"this tenant": "този наемател",
		"Not set":     "Не е зададено",
		"None":        "Няма",
		"Yes":         "Да",
		"No":          "Не",

		// The actor screen.
		"A person or an agent of this tenant, by the name others use for it. What it may do is not shown here.": "Човек или агент от този наемател, с името, с което другите го наричат. Какво може да прави не се показва тук.",
		"About":                "Сведения",
		"Copy the actor id":    "Копирайте идентификатора",
		"Tasks assigned to %s": "Задачи, възложени на %s",
		"Your profile":         "Вашият профил",
		"Recent activity":      "Скорошна дейност",
		"All activity":         "Цялата дейност",
		"Recent activity needs the audit:read scope, which you do not hold.": "Скорошната дейност изисква правото audit:read, което нямате.",
		"Open %s's page": "Отворете страницата на %s",

		// The credential filter on the token and SSH key screens.
		"Whose credentials":                    "Чии идентификационни данни",
		"Handle or id, empty for yours":        "Кратко име или идентификатор, празно за вашите",
		"Show":                                 "Покажи",
		"Show yours":                           "Покажи вашите",
		"No actor of this tenant is named %s.": "В този наемател няма участник с име %s.",
		"Listing what %s holds.":               "Показва се какво притежава %s.",
		"Keys enrolled against the actor named below. Enrolling a key here always enrols it against your own actor.": "Ключове, регистрирани за участника по-долу. Ключ, регистриран оттук, винаги се регистрира за вас.",

		// The subtask tree.
		"Direct subtasks":  "Преки подзадачи",
		"Subtasks of %s":   "Подзадачи на %s",
		"Show %d subtask":  "Покажи %d подзадача",
		"Show %d subtasks": "Покажи %d подзадачи",
		"This tree is too large to draw whole, so only direct subtasks are shown.":                                                                                         "Дървото е твърде голямо, за да се покаже цялото, затова се показват само преките подзадачи.",
		"The whole tree under this task. Three levels show at once, and a deeper level loads when you ask for it. The arrow beside a subtask folds its own subtasks away.": "Цялото дърво под тази задача. Наведнъж се показват три нива, а по-дълбоко ниво се зарежда, когато го поискате. Стрелката до подзадача скрива нейните подзадачи.",
	})
}
