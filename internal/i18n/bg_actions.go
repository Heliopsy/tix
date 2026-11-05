// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Revoke %s?":       "Да се отмени ли %s?",
		"Remove %s?":       "Да се премахне ли %s?",
		"Archive %s?":      "Да се архивира ли %s?",
		"Clean up %s now?": "Да се почисти ли %s сега?",
		"Settings for %s":  "Настройки на %s",
		"Nothing is subscribed to this tenant's events.":                                     "Нищо не е абонирано за събитията на този тенант.",
		"Which key each scheme uses for each action.":                                        "Кой клавиш използва всяка схема за всяко действие.",
		"Archiving hides %s from active lists and keeps its tasks.":                          "Архивирането скрива %s от активните списъци и запазва задачите му.",
		"Deleting removes the project for good. You choose what happens to its tasks first.": "Изтриването премахва проекта завинаги. Първо избирате какво става със задачите му.",
		"Deleting removes the account and its membership. It cannot be undone.":              "Изтриването премахва акаунта и членството му. Не може да се отмени.",
		"What this project is called and how it looks, how long a claim on its tasks holds, how it cleans up its finished tasks and the trash, and archiving or deleting it.":                                                                                                                                                                                                                                      "Как се казва проектът и как изглежда, колко трае поемането на задачите му, как почиства завършените задачи и кошчето, и архивирането или изтриването му.",
		"A row appears when an import <strong>finishes</strong>, read back from the audit entry it wrote. A <strong>dry run</strong> writes nothing, so it leaves no row, and a <strong>failed</strong> run stops without writing one: it shows as its source's status in the second table. A row cannot show where the cursor stood after its run; the second table shows where each cursor stands <em>now</em>.": "Ред се появява, когато вносът <strong>завърши</strong>, прочетен от одитния запис, който е направил. <strong>Пробното изпълнение</strong> не записва нищо, затова не оставя ред, а <strong>неуспешното</strong> спира, без да запише такъв: той се вижда като състояние на източника във втората таблица. Редът не може да покаже къде е бил курсорът след своето изпълнение; втората таблица показва къде е всеки курсор <em>сега</em>.",
		"The tasks an import created or updated are audited one by one, in <a href=\"/activity?source=system\">activity</a>, attributed to the System actor.":                                                                                                                                                                                                                                                      "Задачите, които вносът е създал или обновил, се одитират поотделно в <a href=\"/activity?source=system\">активността</a>, от името на системния актьор.",
	})
}
