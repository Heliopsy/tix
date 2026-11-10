// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"File":                                "Файл",
		"Files up to %s. Accepted types: %s.": "Файлове до %s. Приемани типове: %s.",
		"Largest artifact file":               "Най-голям файл на артефакт",
		"Accepted artifact types":             "Приемани типове артефакти",
		"Leave either empty to use the server's limit, shown greyed.": "Оставете празно, за да се ползва ограничението на сървъра, показано в сиво.",
		"The largest file an artifact may carry on this tenant, such as 512KiB or 5MiB. It can be lower than the server's limit but not higher. Files already attached stay as they are.":                                                                          "Най-големият файл, който артефакт може да носи в този тенант, например 512KiB или 5MiB. Може да е по-нисък от ограничението на сървъра, но не и по-висок. Вече прикачените файлове остават както са.",
		"The media types an artifact file may have on this tenant, separated by commas, such as image/*, application/pdf. Only types the server accepts can be listed. Whatever the type, a file is always offered as a download and never opened in the browser.": "Медийните типове, които файл на артефакт може да има в този тенант, разделени със запетаи, например image/*, application/pdf. Може да се изброят само типове, които сървърът приема. Какъвто и да е типът, файлът винаги се предлага за изтегляне и никога не се отваря в браузъра.",
	})
}
