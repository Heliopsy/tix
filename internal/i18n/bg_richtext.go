// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Editor":         "Редактор",
		"Rich text":      "Форматиран текст",
		"Markdown":       "Markdown",
		"Formatting":     "Форматиране",
		"Bold":           "Удебелен",
		"Italic":         "Курсив",
		"Strikethrough":  "Зачертан",
		"Heading":        "Заглавие",
		"Bulleted list":  "Списък с точки",
		"Numbered list":  "Номериран списък",
		"Quote":          "Цитат",
		"Link":           "Връзка",
		"Table":          "Таблица",
		"Add row":        "Добавяне на ред",
		"Add column":     "Добавяне на колона",
		"Delete row":     "Изтриване на реда",
		"Delete column":  "Изтриване на колоната",
		"Delete table":   "Изтриване на таблицата",
		"Link address":   "Адрес на връзката",
		"Add link":       "Добавяне на връзка",
		"Checklist item": "Точка от списъка за отметки",
		"The rich text editor could not be loaded. Your text is unchanged.":                          "Редакторът за форматиран текст не можа да се зареди. Текстът ви не е променен.",
		"This text uses Markdown the rich text editor cannot keep exactly, so it stays in Markdown.": "Този текст съдържа Markdown, който редакторът за форматиран текст не може да запази точно, затова остава в Markdown.",
	})
}
