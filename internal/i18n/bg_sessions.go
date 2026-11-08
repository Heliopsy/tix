// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{
		"Sessions":             "Сесии",
		"Signed-in sessions":   "Активни сесии",
		"Unknown client":       "Неизвестен клиент",
		"This browser":         "Този браузър",
		"Signed in %s with %s": "Вход %s чрез %s",
		"and a second factor":  "и втори фактор",
		"Expires %s.":          "Изтича %s.",
		"Revoke this session":  "Отмени тази сесия",
		"Revoke this session?": "Да се отмени ли тази сесия?",
		"session revoked":      "сесията е отменена",
		"No sessions. You are signed in with an API token.":                                                                                                  "Няма сесии. Влезли сте с API токен.",
		"Older sessions are listed by tix account sessions --cursor.":                                                                                        "По-старите сесии се извеждат с tix account sessions --cursor.",
		"Sessions belong to a person, and this session does not speak for one, so there is nothing to manage here.":                                          "Сесиите принадлежат на човек, а тази сесия не говори от името на такъв, така че тук няма какво да се управлява.",
		"Every browser and terminal sign-in you hold in this tenant. Revoke one you do not recognise, or one left signed in on a machine you no longer use.": "Всеки ваш вход от браузър или терминал в този наемател. Отменете вход, който не разпознавате или е останал на машина, която вече не ползвате.",
		"This is the session this browser is using, so revoking it signs you out here.":                                                                      "Това е сесията, която този браузър използва, така че отмяната ѝ ще ви отпише тук.",
		"Whatever holds this session is signed out at once. It cannot be undone; signing in again starts a new one.":                                         "Който държи тази сесия, ще бъде отписан веднага. Това не може да се отмени; нов вход започва нова сесия.",
	})
}
