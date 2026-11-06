// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"For example:": "Например:",
		"No due date":  "Без краен срок",
		"A word, an offset from today or a date in your date format. Leave it empty for no due date.": "Дума, отместване от днес или дата във вашия формат за дата. Оставете празно, ако няма краен срок.",
	})
}
