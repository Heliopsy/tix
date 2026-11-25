// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Rotate":           "Смени",
		"Rotate %s":        "Смени %s",
		"Rotate %s?":       "Смяна на %s?",
		"Rotating a token": "Смяна на токен",
		"Old token":        "Старият токен",
		"retiring":         "оттегля се",
		"token rotated":    "токенът е сменен",
		"A new token with the same name, scopes and lifetime replaces this one, and its value is shown once.":                                                                                                            "Нов токен със същото име, обхвати и срок замества този, а стойността му се показва веднъж.",
		"This token is %s's, not yours, and the trail will record that you rotated somebody else's.":                                                                                                                     "Този токен е на %s, не ваш, и в историята ще остане, че сте сменили чужд.",
		"Rotate a token on a schedule, or at once when it may have leaked. Whatever holds it has to be given the new value; a grace window keeps the old one working meanwhile, and revoking it ends that window early.": "Сменяйте токена по график или веднага, ако може да е изтекъл. Всичко, което го държи, трябва да получи новата стойност; гратисният период оставя стария да работи междувременно, а отмяната му прекратява периода по-рано.",
		"How long the old value keeps authenticating beside the new one, up to a day, so a running agent can pick up the new value without failing a request. After a leak, stop it now.":                                "Колко дълго старата стойност продължава да се удостоверява до новата, най-много един ден, за да може работещ агент да вземе новата стойност, без да провали заявка. След изтичане го спрете веднага.",
		"Stop the old token now":         "Спри стария токен веднага",
		"Keep it working for 10 minutes": "Остави го да работи още 10 минути",
		"Keep it working for 1 hour":     "Остави го да работи още 1 час",
		"Keep it working for 24 hours":   "Остави го да работи още 24 часа",
	})
}
