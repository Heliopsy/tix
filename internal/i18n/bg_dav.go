// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The Tasks in other apps section of the settings screen and the screen
// showing an app password once.
func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Tasks in other apps":  "Задачи в други приложения",
		"Add an app password":  "Добави парола за приложение",
		"CalDAV address":       "Адрес за CalDAV",
		"CalDAV":               "CalDAV",
		"Expired":              "Изтекла",
		"Revoked":              "Отменена",
		"read-only":            "само за четене",
		"Created %s.":          "Създадена %s.",
		"Last used %s.":        "Последно използвана %s.",
		"Never used.":          "Не е използвана.",
		"Revoked %s.":          "Отменена %s.",
		"For example, iPhone":  "Например iPhone",
		"Create password":      "Създай парола",
		"app password revoked": "паролата за приложение е отменена",
		"App password":         "Парола за приложение",
		"Sign in with":         "Вход с",
		"Server address":       "Адрес на сървъра",
		"Username":             "Потребителско име",
		"Assigned to me":       "Възложени на мен",
		"One list per project": "По един списък за проект",
		"Both":                 "И двете",
		"Show your tasks in Apple Reminders, Thunderbird, DAVx5 with tasks.org, or Evolution. Each app signs in with an app password of its own.":                                                                         "Виж задачите си в Apple Reminders, Thunderbird, DAVx5 с tasks.org или Evolution. Всяко приложение влиза със своя собствена парола за приложение.",
		"An app password belongs to a person who reads tasks, and this session does not speak for one, so there is nothing to manage here.":                                                                               "Паролата за приложение принадлежи на човек, който чете задачи, а тази сесия не говори от името на такъв, така че тук няма какво да се управлява.",
		"CalDAV is how calendar and reminders apps read tasks from a server. Apple Reminders, Thunderbird, DAVx5 with tasks.org and Evolution all speak it. For now the apps can read your tasks and cannot change them.": "CalDAV е начинът, по който приложенията за календар и напомняния четат задачи от сървър. Apple Reminders, Thunderbird, DAVx5 с tasks.org и Evolution го поддържат. Засега приложенията могат да четат задачите ти, но не и да ги променят.",
		"CalDAV is turned off on this server (caldav.enabled)":                                                                                                                        "CalDAV е изключен на този сървър (caldav.enabled)",
		"Give your app this address, your username and an app password. Your account password does not work here.":                                                                    "Дай на приложението този адрес, потребителското си име и парола за приложение. Паролата на профила ти не работи тук.",
		"No app passwords yet. Add one for each app you sign in.":                                                                                                                     "Още нямаш пароли за приложения. Добави по една за всяко приложение, с което влизаш.",
		"The app or device this password is for, so you know which one to revoke later. Each of your live app passwords has its own name.":                                            "Приложението или устройството, за което е паролата, за да знаеш по-късно коя да отмениш. Всяка твоя активна парола за приложение има собствено име.",
		"One list of the tasks assigned to you, in every project.":                                                                                                                    "Един списък със задачите, възложени на теб, във всички проекти.",
		"A list for each project you can read, with everybody's tasks.":                                                                                                               "Списък за всеки проект, който можеш да четеш, със задачите на всички.",
		"Assigned to me and every project, so your own tasks appear twice.":                                                                                                           "Възложените на мен и всеки проект, така че собствените ти задачи се появяват два пъти.",
		"Read-only, even once apps can change tasks":                                                                                                                                  "Само за четене, дори когато приложенията могат да променят задачи",
		"The app signed in with this password is refused from its next request. It cannot be undone; add a new password to sign the app in again.":                                    "Приложението, влязло с тази парола, ще бъде отказано при следващата си заявка. Това не може да се отмени; добави нова парола, за да влезеш отново в приложението.",
		"An app password signs one calendar or reminders app in to your tasks over CalDAV. It reads tasks as you and nothing else, and revoking it in Settings stops that app alone.": "Паролата за приложение вписва едно приложение за календар или напомняния към задачите ти чрез CalDAV. Тя чете задачи от твое име и нищо друго, а отмяната ѝ в Настройки спира само това приложение.",
		"Sign %s in with these three values. The password is not shown again, so copy it now.":                                                                                        "Впиши %s с тези три стойности. Паролата няма да бъде показана отново, затова я копирай сега.",
		"<strong>Apple Reminders</strong>: Settings, then Calendar, Accounts, Add Account, Other, and Add CalDAV Account.":                                                            "<strong>Apple Reminders</strong>: Настройки, после Календар, Акаунти, Добавяне на акаунт, Друго и Добавяне на CalDAV акаунт.",
		"<strong>Thunderbird</strong>: New Calendar, On the Network, then the username and the server address.":                                                                       "<strong>Thunderbird</strong>: Нов календар, В мрежата, после потребителското име и адреса на сървъра.",
		"<strong>DAVx5</strong>: add an account with a URL and username, then pick tasks.org to show the lists.":                                                                      "<strong>DAVx5</strong>: добави акаунт с URL и потребителско име, после избери tasks.org, за да показва списъците.",
		"<strong>Evolution</strong>: New, then Task List, type CalDAV, and the server address.":                                                                                       "<strong>Evolution</strong>: Нов, после Списък със задачи, тип CalDAV и адреса на сървъра.",
	})
}
