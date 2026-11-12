// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The calendar feed section of the settings screen and the screen showing a
// feed address once.
func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Calendar feed":   "Kalender-Feed",
		"Calendar filter": "Kalenderfilter",
		"A private address Google Calendar, Apple Calendar or Outlook subscribes to, so your due dates appear beside the rest of your week. Each task with a due date is an all-day event that links back here. Anybody holding the address can read what it lists, so treat it like a password: rotate it to issue a new one, revoke it to stop it.": "Eine private Adresse, die Google Kalender, Apple Kalender oder Outlook abonniert, damit Ihre Fälligkeitsdaten neben dem Rest Ihrer Woche erscheinen. Jede Aufgabe mit Fälligkeitsdatum ist ein ganztägiger Termin mit einem Link hierher. Wer die Adresse hat, kann lesen, was sie auflistet, behandeln Sie sie also wie ein Passwort: Erneuern Sie sie, um eine neue auszustellen, widerrufen Sie sie, um sie zu stoppen.",
		"Feed address": "Feed-Adresse",
		"A calendar feed reads tasks as you, and this session cannot read tasks, so there is nothing to manage here.": "Ein Kalender-Feed liest Aufgaben als Sie, und diese Sitzung kann keine Aufgaben lesen, daher gibt es hier nichts zu verwalten.",
		"Feed filter": "Feed-Filter",
		"A filter expression, the same language as the task list's filter box. Leave it empty for the tasks assigned to you that are not finished. Only tasks with a due date are listed either way. Changing it keeps the address.": "Ein Filterausdruck in derselben Sprache wie das Filterfeld der Aufgabenliste. Leer lassen für die Ihnen zugewiesenen, nicht abgeschlossenen Aufgaben. Aufgelistet werden in jedem Fall nur Aufgaben mit Fälligkeitsdatum. Eine Änderung behält die Adresse.",
		"Which tasks the feed lists. Empty means yours, not finished.": "Welche Aufgaben der Feed auflistet. Leer bedeutet Ihre, nicht abgeschlossene.",
		"Save filter":        "Filter speichern",
		"Rotate the address": "Adresse erneuern",
		"Issues a new address with the same filter and stops the old one at once. Use it if the address was shared by mistake, or to subscribe somewhere new.": "Stellt eine neue Adresse mit demselben Filter aus und stoppt die alte sofort. Verwenden Sie das, wenn die Adresse versehentlich geteilt wurde oder um sie woanders neu zu abonnieren.",
		"A new address; the old one stops.": "Eine neue Adresse; die alte wird gestoppt.",
		"Rotate":                            "Erneuern",
		"Revoke the feed":                   "Feed widerrufen",
		"Stops the address at once. Subscribed calendars keep the events they last fetched until you remove the subscription there.": "Stoppt die Adresse sofort. Abonnierte Kalender behalten die zuletzt abgerufenen Termine, bis Sie das Abonnement dort entfernen.",
		"The address stops answering.":                                          "Die Adresse antwortet nicht mehr.",
		"You have no calendar feed. Create one to subscribe to your due dates.": "Sie haben keinen Kalender-Feed. Legen Sie einen an, um Ihre Fälligkeitsdaten zu abonnieren.",
		"Create feed": "Feed anlegen",
		"No feed":     "Kein Feed",
		"Optional. Empty means yours, not finished.": "Optional. Leer bedeutet Ihre, nicht abgeschlossene.",
		"A filter expression, the same language as the task list's filter box. Leave it empty for the tasks assigned to you that are not finished. Only tasks with a due date are listed either way. It can be changed later without changing the address.": "Ein Filterausdruck in derselben Sprache wie das Filterfeld der Aufgabenliste. Leer lassen für die Ihnen zugewiesenen, nicht abgeschlossenen Aufgaben. Aufgelistet werden in jedem Fall nur Aufgaben mit Fälligkeitsdatum. Er lässt sich später ändern, ohne die Adresse zu ändern.",
		"Subscribe to this address in your calendar app. It is not shown again, so copy it now.":                                            "Abonnieren Sie diese Adresse in Ihrer Kalender-App. Sie wird nicht erneut angezeigt, kopieren Sie sie also jetzt.",
		"This address replaces the old one, which has stopped. Subscribe to it again wherever the old one was used. It is not shown again.": "Diese Adresse ersetzt die alte, die gestoppt wurde. Abonnieren Sie sie überall dort neu, wo die alte verwendet wurde. Sie wird nicht erneut angezeigt.",
		"Lists:": "Listet:",
		"the tasks assigned to you that are not finished":                                                                 "die Ihnen zugewiesenen, nicht abgeschlossenen Aufgaben",
		"<strong>Google Calendar</strong>: Other calendars, then From URL, and paste the address.":                        "<strong>Google Kalender</strong>: Weitere Kalender, dann Per URL, und die Adresse einfügen.",
		"<strong>Apple Calendar</strong>: File, then New Calendar Subscription, or open the subscribe link below.":        "<strong>Apple Kalender</strong>: Ablage, dann Neues Kalenderabonnement, oder den Abonnieren-Link unten öffnen.",
		"<strong>Outlook</strong>: Add calendar, then Subscribe from web, and paste the address.":                         "<strong>Outlook</strong>: Kalender hinzufügen, dann Aus dem Internet abonnieren, und die Adresse einfügen.",
		"Subscribe in this device's calendar":                                                                             "Im Kalender dieses Geräts abonnieren",
		"I have kept it, back to settings":                                                                                "Ich habe sie gesichert, zurück zu den Einstellungen",
		"calendar feed revoked":                                                                                           "Kalender-Feed widerrufen",
		"A private address your calendar subscribes to, so your due dates appear beside the rest of your week.":           "Eine private Adresse, die Ihr Kalender abonniert, damit Ihre Fälligkeitsdaten neben dem Rest Ihrer Woche erscheinen.",
		"Created %s. The address is shown only once, when the feed is created or rotated; Rotate below issues a new one.": "Erstellt %s. Die Adresse wird nur einmal angezeigt, beim Anlegen oder Erneuern des Feeds; Erneuern unten stellt eine neue aus.",
		"Google Calendar, Apple Calendar or Outlook subscribes to the address, and each task with a due date is an all-day event that links back here. Anybody holding the address can read what it lists, so treat it like a password. Only a hash of it is kept, which is why it was shown once, when it was created, and cannot be shown again. Rotate issues a new one; calendars subscribed to the old address stop updating. More in docs/calendar.md.": "Google Kalender, Apple Kalender oder Outlook abonniert die Adresse, und jede Aufgabe mit Fälligkeitsdatum ist ein ganztägiger Termin mit einem Link hierher. Wer die Adresse hat, kann lesen, was sie auflistet, behandeln Sie sie also wie ein Passwort. Nur ein Hash davon wird gespeichert, deshalb wurde sie nur einmal beim Anlegen gezeigt und kann nicht erneut angezeigt werden. Erneuern stellt eine neue aus; Kalender, die die alte Adresse abonniert haben, werden nicht mehr aktualisiert. Mehr in docs/calendar.md.",
		"Google Calendar, Apple Calendar or Outlook subscribes to the address, and each task with a due date is an all-day event that links back here. Creating a feed shows its address once, on the screen that follows, with a Copy button. Anybody holding it can read what it lists, so treat it like a password. Only a hash of it is kept, so it cannot be shown again; rotating issues a new one. More in docs/calendar.md.":                          "Google Kalender, Apple Kalender oder Outlook abonniert die Adresse, und jede Aufgabe mit Fälligkeitsdatum ist ein ganztägiger Termin mit einem Link hierher. Beim Anlegen eines Feeds wird seine Adresse einmal auf der folgenden Seite mit einer Kopieren-Schaltfläche angezeigt. Wer sie hat, kann lesen, was sie auflistet, behandeln Sie sie also wie ein Passwort. Nur ein Hash davon wird gespeichert, sie kann also nicht erneut angezeigt werden; Erneuern stellt eine neue aus. Mehr in docs/calendar.md.",
		"For example, project:infra tag:ops": "Zum Beispiel project:infra tag:ops",
	})
}
