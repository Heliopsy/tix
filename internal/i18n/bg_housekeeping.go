// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// Housekeeping: the cleanup rules on the Tenant screen and in a
		// project's settings.
		"Housekeeping":           "Почистване",
		"Save housekeeping":      "Запази почистването",
		"Clean up now":           "Почисти сега",
		"Done tasks":             "Завършени задачи",
		"Cancelled tasks":        "Отменени задачи",
		"After a number of days": "След брой дни",
		"Days":                   "Дни",
		"Inherit: off":           "Наследява: изключено",
		"Inherit: %d day":        "Наследява: %d ден",
		"Inherit: %d days":       "Наследява: %d дни",
		"%d task would be moved to the trash now.":  "%d задача би била преместена в кошчето сега.",
		"%d tasks would be moved to the trash now.": "%d задачи биха били преместени в кошчето сега.",
		"%d task would be deleted for good now.":    "%d задача би била изтрита завинаги сега.",
		"%d tasks would be deleted for good now.":   "%d задачи биха били изтрити завинаги сега.",
		"cleanup saved": "почистването е запазено",
		"cleanup ran":   "почистването приключи",
		"cleanup ran: %d moved to the trash, %d deleted for good":                  "почистването приключи: %d преместени в кошчето, %d изтрити завинаги",
		"Cleanup acts on this project only: %d to the trash, %d deleted for good.": "Почистването засяга само този проект: %d в кошчето, %d изтрити завинаги.",
		"Clean up this project":                    "Почисти този проект",
		"Paused while the trash is off.":           "Спряно, докато кошчето е изключено.",
		"Nothing to clean up now.":                 "Няма нищо за почистване сега.",
		"cleanup is paused while the trash is off": "почистването е спряно, докато кошчето е изключено",
		"The trash is off, so cleanup is paused: nothing is moved or deleted until the trash is on again.":                                                                                                                     "Кошчето е изключено, затова почистването е спряно: нищо не се премества и не се изтрива, докато кошчето не бъде включено отново.",
		"Overrides the tenant's cleanup defaults for this project. Inherit follows the Tenant screen.":                                                                                                                         "Заменя настройките за почистване на тенанта за този проект. Наследява следва екрана Тенант.",
		"Cleans up finished tasks and the trash on its own, on the same schedule as retention. These are the defaults for every project; a project can override each one in its settings. Everything is off until you set it.": "Почиства приключените задачи и кошчето само, по същия график като съхранението. Това са настройките по подразбиране за всеки проект; проектът може да замени всяка от тях в своите настройки. Всичко е изключено, докато не го зададете.",
		"Tasks that have sat in a done state this many days go to the trash, where they can be restored. A done task with open subtasks stays until they are finished too. Cannot be turned on while the trash is off.":        "Задачите, стояли толкова дни в завършено състояние, отиват в кошчето, откъдето могат да бъдат възстановени. Завършена задача с отворени подзадачи остава, докато и те не приключат. Не може да се включи, докато кошчето е изключено.",
		"Tasks that have sat in a cancelled state this many days go to the trash, where they can be restored. Set apart from done tasks, because abandoned work is usually worth less to keep.":                                "Задачите, стояли толкова дни в отменено състояние, отиват в кошчето, откъдето могат да бъдат възстановени. Отделно от завършените задачи, защото изоставената работа обикновено е по-малко ценна за пазене.",
		"Tasks deleted more than this many days ago are deleted for good, subtasks first. This cannot be undone.":                                                                                                              "Задачите, изтрити преди повече от толкова дни, се изтриват завинаги, първо подзадачите. Това не може да бъде отменено.",
	})
}
