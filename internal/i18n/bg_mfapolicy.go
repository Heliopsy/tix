// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"This tenant requires a second factor. Set one up to go on.": "Този клиент изисква втори фактор. Настройте го, за да продължите.",
		"This tenant requires a second factor from %s.":              "Този клиент изисква втори фактор от %s.",
		"Set up a second factor":                                     "Настройте втори фактор",
		"This tenant requires a second factor, and this account has none yet. Until you add one you can only set it up, read your account's history or sign out.": "Този клиент изисква втори фактор, а този профил все още няма. Докато не добавите такъв, можете само да го настроите, да прегледате историята на профила си или да излезете.",
		"This tenant requires a second factor from %s. Add one now and nothing changes for you then.":                                                             "Този клиент изисква втори фактор от %s. Добавете го сега и тогава нищо няма да се промени за вас.",
		"A second factor asks for a code from your phone after your password, so a leaked password alone does not sign anybody in.":                               "Вторият фактор иска код от телефона ви след паролата, така че изтекла парола сама по себе си не влиза в профила.",
		"You need an authenticator app on your phone, such as one from your phone's maker or any TOTP app.":                                                       "Нужно ви е приложение за удостоверяване на телефона, например това на производителя му или което и да е TOTP приложение.",
		"Everybody":                "Всички",
		"Without a second factor":  "Без втори фактор",
		"2FA":                      "2FA",
		"no 2FA":                   "без 2FA",
		"Second factor policy":     "Политика за втори фактор",
		"Off: each person chooses": "Изключена: всеки решава сам",
		"Required for everybody":   "Задължителна за всички",
		"Grace period":             "Гратисен период",
		"People without a second factor are refused from %s.": "Хората без втори фактор получават отказ от %s.",
		"Who has none yet": "Кой още няма",
		"Required: on every surface, a person with no second factor can only set one up, read their own account or sign out, with a session or with a token of theirs. Tokens of agents are not affected. You need a second factor yourself before you can turn it on.": "Задължителна: навсякъде човек без втори фактор може само да си настрои такъв, да прегледа собствения си профил или да излезе, със сесия или със свой токен. Токените на агентите не се засягат. Нужен ви е собствен втори фактор, преди да я включите.",
		"How long people without a second factor may keep working after the policy is turned on, with a banner naming the deadline. Written as a duration: 7d, 48h. Empty or 0 applies it at once.":                                                                     "Колко дълго хората без втори фактор могат да продължат да работят, след като политиката се включи, с банер, който посочва крайния срок. Пише се като продължителност: 7d, 48h. Празно или 0 я прилага веднага.",
	})
}
