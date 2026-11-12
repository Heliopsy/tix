// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The live connection indicator beside the tenant's name and its
// connection details dialog.
func init() {
	register(Bulgarian, map[string]string{
		"Live updates":                       "Обновяване на живо",
		"Live updates: %s":                   "Обновяване на живо: %s",
		"Live":                               "На живо",
		"Connecting":                         "Свързване",
		"Polling":                            "Периодична проверка",
		"Reconnecting":                       "Повторно свързване",
		"Offline":                            "Няма връзка",
		"Connection details":                 "Подробности за връзката",
		"Carried over":                       "Пренася се чрез",
		"Since":                              "От",
		"Last reached the server":            "Последна връзка със сървъра",
		"Last event":                         "Последно събитие",
		"Failures in a row":                  "Неуспехи подред",
		"Next try":                           "Следващ опит",
		"Last error":                         "Последна грешка",
		"During":                             "При",
		"Close code":                         "Код на затваряне",
		"HTTP status":                        "HTTP статус",
		"Message":                            "Съобщение",
		"What it may mean":                   "Какво може да означава",
		"Recent changes":                     "Последни промени",
		"Nothing yet.":                       "Все още нищо.",
		"Copy details":                       "Копирай подробностите",
		"Reconnect now":                      "Свържи отново сега",
		"WebSocket":                          "WebSocket",
		"Polling over HTTP every 15 seconds": "Проверка по HTTP на всеки 15 секунди",
		"Fetching the page":                  "Зареждане на страницата",
		"Could not reach the server":         "Сървърът не може да бъде достигнат",
		"Connection closed":                  "Връзката е затворена",
		"Refused by the server":              "Отказано от сървъра",
		"HTTP error":                         "HTTP грешка",
		"Nothing has failed since this page was loaded.":                                                                                "Нищо не се е проваляло, откакто страницата е заредена.",
		"Changes made elsewhere show up here as they happen.":                                                                           "Промените, направени другаде, се появяват тук веднага.",
		"Connecting to live updates. Changes show up once it is through.":                                                               "Свързване към обновяването на живо. Промените ще се появят, щом връзката се осъществи.",
		"The live connection keeps failing, so this page asks the server every 15 seconds instead. Changes can take that long to show.": "Връзката на живо продължава да се проваля, затова страницата пита сървъра на всеки 15 секунди. Промените може да се появят с такова закъснение.",
		"The live connection dropped and is being tried again. Changes made meanwhile show up once it is back.":                         "Връзката на живо прекъсна и се опитва отново. Промените междувременно ще се появят, щом се възстанови.",
		"The server cannot be reached. This page keeps trying and catches up when it is back.":                                          "Сървърът не може да бъде достигнат. Страницата продължава да опитва и наваксва, когато се върне.",
		"A proxy or load balancer may be closing idle connections. Its read timeout should be above 60 s for /api/v1/events.":           "Прокси или балансьор може да затваря неактивните връзки. Времето му за четене трябва да е над 60 s за /api/v1/events.",
		"The WebSocket fails while plain HTTP works, so the proxy in front of tix may not forward WebSocket upgrades.":                  "WebSocket се проваля, а обикновеният HTTP работи, така че проксито пред tix може да не препраща WebSocket връзки.",
		"The browser could not reach the host. Check the network and that the server is running.":                                       "Браузърът не достигна хоста. Проверете мрежата и дали сървърът работи.",
		"The server refused live updates to this session. Signing in again usually fixes it.":                                           "Сървърът отказа обновяване на живо за тази сесия. Повторното влизане обикновено помага.",
		"The server or a proxy answered HTTP %s. Its logs should say why.":                                                              "Сървърът или прокси отговори с HTTP %s. Журналите му трябва да казват защо.",
		"Connection details copied. Cookies and tokens are left out.":                                                                   "Подробностите за връзката са копирани. Бисквитките и токените са изпуснати.",
	})
}
