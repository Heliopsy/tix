// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// The trash: the task list filtered by is:trashed.
		"Trash":            "Кошче",
		"%d deleted task":  "%d изтрита задача",
		"%d deleted tasks": "%d изтрити задачи",
		"Restore one to put it back where it was.":              "Възстановете задача, за да я върнете там, където беше.",
		"The trash is empty.":                                   "Кошчето е празно.",
		"The trash is empty":                                    "Кошчето е празно",
		"A deleted task waits here until somebody restores it.": "Изтритата задача чака тук, докато някой не я възстанови.",
		"Deleted %s": "Изтрита %s",
		"deleted":    "изтрита",
		"Restore %s": "Възстанови %s",
		"Restore":    "Възстанови",

		// A tenant whose trash is turned off: the confirmation that stands in
		// for the undo, and the setting on the Tenant screen.
		"task deleted permanently":    "задачата е изтрита завинаги",
		"Delete %d task permanently":  "Изтриване на %d задача завинаги",
		"Delete %d tasks permanently": "Изтриване на %d задачи завинаги",
		"Trash is off":                "Кошчето е изключено",
		"This tenant's trash is turned off, so a deleted task is removed for good: it cannot be restored and there is no undo. A tenant admin can turn the trash back on from the Tenant screen.": "Кошчето на този тенант е изключено, затова изтритата задача се премахва завинаги: не може да бъде възстановена и няма отмяна. Администратор на тенанта може да включи кошчето отново от екрана Тенант.",
		"Delete %s?": "Да се изтрие ли %s?",
		"The trash is turned off for this tenant, so this cannot be undone.": "Кошчето на този тенант е изключено, затова това не може да бъде отменено.",
		"Their subtasks are deleted with them.":                              "Подзадачите им се изтриват заедно с тях.",
		"On: deleted tasks can be restored":                                  "Включено: изтритите задачи могат да се възстановят",
		"Off: deleting is permanent":                                         "Изключено: изтриването е завинаги",
		"On, a deleted task goes to the Trash, where anyone who could delete it can restore it, and every delete offers Undo. Off, deleting a task removes it for good: each delete asks first, nothing can be restored, and the Trash leaves the sidebar. Tasks already in the trash when it is turned off are kept, out of reach, and come back when it is turned on again.": "Включено, изтритата задача отива в Кошчето, откъдето всеки, който е можел да я изтрие, може да я възстанови, и всяко изтриване предлага Отмяна. Изключено, изтриването на задача я премахва завинаги: всяко изтриване първо пита, нищо не може да се възстанови, а Кошчето изчезва от страничната лента. Задачите, които вече са в кошчето при изключването му, се пазят недостъпни и се връщат, когато то бъде включено отново.",
		"%d task deleted while the trash was on is kept out of reach until it is turned back on.":                 "%d задача, изтрита докато кошчето беше включено, се пази недостъпна, докато то не бъде включено отново.",
		"%d tasks deleted while the trash was on are kept out of reach until it is turned back on.":               "%d задачи, изтрити докато кошчето беше включено, се пазят недостъпни, докато то не бъде включено отново.",
		"%d task is in the trash. Turning the trash off keeps it, out of reach, until the trash is on again.":     "%d задача е в кошчето. Изключването на кошчето я запазва недостъпна, докато то не бъде включено отново.",
		"%d tasks are in the trash. Turning the trash off keeps them, out of reach, until the trash is on again.": "%d задачи са в кошчето. Изключването на кошчето ги запазва недостъпни, докато то не бъде включено отново.",
	})
}
