// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The Live events screen: the tenant's raw event stream, followed live.
func init() {
	register(Bulgarian, map[string]string{
		"Live events": "Събития на живо",
		"Every event this tenant emits, as clients receive it: the stream tix watch prints. Nothing here is stored; a reload starts from now unless you replay from a sequence number. Activity is the audit log, kept and searchable.": "Всяко събитие на този наемател, както го получават клиентите: потокът, който извежда tix watch. Нищо тук не се пази; презареждането започва от сега, освен ако не възпроизведете от пореден номер. Дейността е одитният журнал, който се пази и може да се търси.",
		"The tenant's events as they happen, newest first.": "Събитията на наемателя, докато се случват, най-новите първи.",
		"Event types": "Видове събития",
		"Comma separated, sent to the server. A trailing * matches a prefix, as in task.*.": "Разделени със запетая, изпращат се към сървъра. Звездичка * в края съвпада с представка, както в task.*.",
		"Actors": "Участници",
		"Handles or ids, comma separated. Matched in this browser, as tix watch does.": "Потребителски имена или идентификатори, разделени със запетая. Сравняват се в този браузър, както прави tix watch.",
		"Replay from sequence number": "Възпроизвеждане от пореден номер",
		"Shows the events after this number again, then follows live. Empty starts from now.":                         "Показва отново събитията след този номер, после следи на живо. Празно започва от сега.",
		"This picker does not list every project in this tenant, so a project missing from it cannot be chosen here.": "Този избор не показва всички проекти на наемателя, така че липсващ проект не може да бъде избран тук.",
		"type %s":                    "вид %s",
		"project %s":                 "проект %s",
		"actor %s":                   "участник %s",
		"after #%s":                  "след №%s",
		"No project has the key %s.": "Няма проект с ключ %s.",
		"The sequence number %s is not a whole number above zero.": "Поредният номер %s не е цяло число над нула.",
		"Follow every event":                  "Следи всички събития",
		"Connecting to the event stream":      "Свързване с потока от събития",
		"This server does not stream events.": "Този сървър не изпраща поток от събития.",
		"Pause":                               "Пауза",
		"Resume":                              "Продължи",
		"Clear the list":                      "Изчисти списъка",
		"Following events needs JavaScript. The filters above still make a link you can send.": "Следенето на събития изисква JavaScript. Филтрите горе все пак дават връзка, която можете да изпратите.",
		"Keeping the latest %d events; older ones are dropped.":                                "Пазят се последните %d събития; по-старите отпадат.",
		"Waiting for events": "Изчакване на събития",
		"Changes in this tenant show up here as they happen. Earlier ones are not shown unless you replay from a sequence number.": "Промените в този наемател се появяват тук, докато се случват. По-ранните не се показват, освен ако не възпроизведете от пореден номер.",
		"Event":                  "Събитие",
		"Event #%d":              "Събитие №%d",
		"Open":                   "Отвори",
		"Details":                "Подробности",
		"Live":                   "На живо",
		"Offline. Reconnecting.": "Няма връзка. Свързване отново.",
		"Not following events.":  "Събитията не се следят.",
		"Paused.":                "На пауза.",
		"Paused, 1 new event.":   "На пауза, 1 ново събитие.",
		"Paused, %d new events.": "На пауза, %d нови събития.",
		"Events after #%d are no longer kept on the server. Showing new events from now.": "Събитията след №%d вече не се пазят на сървъра. Показват се новите събития от сега.",
		"The server refused this subscription: %s":                                        "Сървърът отказа този абонамент: %s",
	})
}
