// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Preview":             "Преглед",
		"Write":               "Писане",
		"Loading preview…":    "Зареждане на прегледа…",
		"Nothing to preview.": "Няма какво да се покаже.",
		"The preview could not be drawn. Your text is unchanged.": "Прегледът не можа да се покаже. Текстът ви не е променен.",
		"Checklist":                     "Списък за отметки",
		"%d of %d checklist items done": "%d от %d точки от списъка са отметнати",
		"item ticked":                   "точката е отметната",
		"item cleared":                  "отметката е махната",
		"Written in Markdown: **bold**, lists, tables, `code`, links, and - [ ] for a checklist the task screen can tick. A reference such as infra-8 and an @handle become links. HTML is shown as typed, and images are linked rather than loaded.": "Пише се в Markdown: **удебелено**, списъци, таблици, `код`, връзки и - [ ] за списък с отметки, който екранът на задачата може да отмята. Препратка като infra-8 и @потребител стават връзки. HTML се показва както е написан, а изображенията се свързват, без да се зареждат.",
	})
}
