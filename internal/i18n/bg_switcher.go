// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// The tenant switcher: the sidebar entry, its dialog and the Choose a
		// tenant screen.
		"Choose a tenant":           "Изберете тенант",
		"Continue here":             "Продължете тук",
		"Current":                   "Текущ",
		"Served at another address": "Обслужва се на друг адрес",
		"Sign in there":             "Влезте там",
		"Sign in to %s":             "Вход в %s",
		"Switch":                    "Превключи",
		"Switch tenant":             "Смяна на тенант",
		"Switch tenant, now %s":     "Смяна на тенант, сега %s",
		"Switch to %s":              "Превключване към %s",
		"Your tenants":              "Вашите тенанти",
		"Tenant %s asks for a second factor, and this session was signed in without one.":                                                 "Тенант %s изисква втори фактор, а тази сесия е влязла без такъв.",
		"You belong to more than one tenant. Pick the one to work in; the tenant button in the sidebar moves you to another at any time.": "Членувате в повече от един тенант. Изберете в кой да работите; бутонът за тенант в страничната лента ви премества в друг по всяко време.",
		"You belong to one tenant only.": "Членувате само в един тенант.",
	})
}
