// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The calendar feed section of the settings screen and the screen showing a
// feed address once.
func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Calendar feed":   "Календарен абонамент",
		"Calendar filter": "Филтър на календара",
		"A private address Google Calendar, Apple Calendar or Outlook subscribes to, so your due dates appear beside the rest of your week. Each task with a due date is an all-day event that links back here. Anybody holding the address can read what it lists, so treat it like a password: rotate it to issue a new one, revoke it to stop it.": "Личен адрес, за който Google Calendar, Apple Calendar или Outlook се абонират, така че крайните ти срокове се показват до останалата част от седмицата ти. Всяка задача с краен срок е събитие за цял ден с връзка обратно тук. Всеки, който има адреса, може да чете това, което той изброява, затова го пази като парола: смени го, за да получиш нов, или го отмени, за да го спреш.",
		"Feed address": "Адрес на абонамента",
		"A calendar feed reads tasks as you, and this session cannot read tasks, so there is nothing to manage here.":                      "Календарният абонамент чете задачите от твое име, а тази сесия не може да чете задачи, така че тук няма какво да се управлява.",
		"The address was shown once, when it was created. Rotate to see a new one; calendars subscribed to the old address stop updating.": "Адресът беше показан веднъж, при създаването му. Смени го, за да видиш нов; календарите, абонирани за стария адрес, спират да се обновяват.",
		"Active since %s.": "Активен от %s.",
		"Feed filter":      "Филтър на абонамента",
		"A filter expression, the same language as the task list's filter box. Leave it empty for the tasks assigned to you that are not finished. Only tasks with a due date are listed either way. Changing it keeps the address.": "Филтриращ израз на същия език като полето за филтър в списъка със задачи. Остави го празно за възложените на теб задачи, които не са завършени. И в двата случая се изброяват само задачи с краен срок. Промяната му запазва адреса.",
		"Which tasks the feed lists. Empty means yours, not finished.": "Кои задачи изброява абонаментът. Празно означава твоите, незавършени.",
		"assigned to you, not finished":                                "възложени на теб, незавършени",
		"Save filter":                                                  "Запази филтъра",
		"Rotate the address":                                           "Смени адреса",
		"Issues a new address with the same filter and stops the old one at once. Use it if the address was shared by mistake, or to subscribe somewhere new.": "Издава нов адрес със същия филтър и веднага спира стария. Използвай го, ако адресът е споделен по погрешка или за абонамент на ново място.",
		"A new address; the old one stops.": "Нов адрес; старият спира.",
		"Rotate":                            "Смени",
		"Revoke the feed":                   "Отмени абонамента",
		"Stops the address at once. Subscribed calendars keep the events they last fetched until you remove the subscription there.": "Спира адреса веднага. Абонираните календари пазят последно изтеглените събития, докато не премахнеш абонамента там.",
		"The address stops answering.":                                          "Адресът спира да отговаря.",
		"You have no calendar feed. Create one to subscribe to your due dates.": "Нямаш календарен абонамент. Създай такъв, за да се абонираш за крайните си срокове.",
		"Create feed": "Създай абонамент",
		"Subscribe to this address in your calendar app. It is not shown again, so copy it now.":                                            "Абонирай се за този адрес в календара си. Няма да бъде показан отново, затова го копирай сега.",
		"This address replaces the old one, which has stopped. Subscribe to it again wherever the old one was used. It is not shown again.": "Този адрес заменя стария, който е спрян. Абонирай се отново навсякъде, където се е ползвал старият. Няма да бъде показан отново.",
		"Lists:": "Изброява:",
		"the tasks assigned to you that are not finished":                                                          "възложените на теб задачи, които не са завършени",
		"<strong>Google Calendar</strong>: Other calendars, then From URL, and paste the address.":                 "<strong>Google Calendar</strong>: Други календари, след това От URL адрес, и постави адреса.",
		"<strong>Apple Calendar</strong>: File, then New Calendar Subscription, or open the subscribe link below.": "<strong>Apple Calendar</strong>: Файл, след това Нов абонамент за календар, или отвори връзката за абонамент по-долу.",
		"<strong>Outlook</strong>: Add calendar, then Subscribe from web, and paste the address.":                  "<strong>Outlook</strong>: Добавяне на календар, след това Абониране от уеб, и постави адреса.",
		"Subscribe in this device's calendar":                                                                      "Абонирай се в календара на това устройство",
		"I have kept it, back to settings":                                                                         "Запазих го, обратно към настройките",
		"calendar feed revoked":                                                                                    "календарният абонамент е отменен",
	})
}
