// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Keep me signed in":       "Запомни ме",
		"Kept signed in.":         "Запомнен вход.",
		"Sign-in lasts":           "Входът трае",
		"Sign out when idle for":  "Изход при бездействие от",
		"Keep me signed in lasts": "„Запомни ме“ трае",
		"never":                   "никога",
		"Leave any empty to use the server's setting, shown greyed.": "Оставете празно, за да се ползва настройката на сървъра, показана в сиво.",
		"End a session nobody has used for this long, such as 30m, kept signed in or not. It applies to sessions already signed in as soon as it is saved. It can be shorter than the server's but not longer; empty with no server setting means never.": "Прекратява сесия, която никой не е ползвал толкова време, например 30m, със или без „Запомни ме“. Важи за вече влезлите сесии веднага щом се запази. Може да е по-кратко от това на сървъра, но не и по-дълго; празно без настройка на сървъра означава никога.",
		"How long a sign-in lasts when the person ticks Keep me signed in, such as 7d, or off to stop offering it. It can be shorter than the server's but not longer.":                                                                                   "Колко трае входът, когато човекът отметне „Запомни ме“, например 7d, или off, за да не се предлага. Може да е по-кратко от това на сървъра, но не и по-дълго.",
	})
}
