// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Keep me signed in":         "Запомни ме",
		"Up to %s on this browser.": "До %s в този браузър.",
		"Kept signed in.":           "Запомнен вход.",
		"Sign-in lasts":             "Входът трае",
		"Sign out when idle for":    "Изход при бездействие от",
		"Keep me signed in lasts":   "„Запомни ме“ трае",
		"never":                     "никога",
		"Leave any empty to use the server's setting, shown greyed.": "Оставете празно, за да се ползва настройката на сървъра, показана в сиво.",
		"Stay signed in on this browser after it closes, until the time shown. Leave it unticked on a shared computer. If this tenant ends sessions left unused, that still applies.":                                                                     "Останете влезли в този браузър и след затварянето му, до показаното време. Не го отмятайте на споделен компютър. Ако този тенант прекратява неизползвани сесии, това продължава да важи.",
		"How long a sign-in on this tenant lasts from the moment it is made, such as 8h. It can be shorter than the server's but not longer. Sessions already signed in keep the expiry they were given.":                                                 "Колко трае входът в този тенант от момента, в който е направен, например 8h. Може да е по-кратко от това на сървъра, но не и по-дълго. Вече влезлите сесии запазват изтичането, което са получили.",
		"End a session nobody has used for this long, such as 30m, kept signed in or not. It applies to sessions already signed in as soon as it is saved. It can be shorter than the server's but not longer; empty with no server setting means never.": "Прекратява сесия, която никой не е ползвал толкова време, например 30m, със или без „Запомни ме“. Важи за вече влезлите сесии веднага щом се запази. Може да е по-кратко от това на сървъра, но не и по-дълго; празно без настройка на сървъра означава никога.",
		"How long a sign-in lasts when the person ticks Keep me signed in, such as 7d, or off to stop offering it. It can be shorter than the server's but not longer.":                                                                                   "Колко трае входът, когато човекът отметне „Запомни ме“, например 7d, или off, за да не се предлага. Може да е по-кратко от това на сървъра, но не и по-дълго.",
	})
}
