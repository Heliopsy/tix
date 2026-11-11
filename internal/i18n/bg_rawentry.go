// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The dialog a history row's Raw control opens: one audit entry as recorded.
func init() {
	register(Bulgarian, map[string]string{
		"Raw entry":                        "Сурови данни за записа",
		"Entry":                            "Запис",
		"Step %d of %d":                    "Стъпка %d от %d",
		"Actor":                            "Извършил",
		"Subject":                          "Обект",
		"Time":                             "Време",
		"Sequence":                         "Пореден номер",
		"system":                           "системата",
		"Changed fields":                   "Променени полета",
		"Field":                            "Поле",
		"Before":                           "Преди",
		"After":                            "След",
		"added":                            "добавено",
		"removed":                          "премахнато",
		"(none)":                           "(няма)",
		"This entry recorded no snapshot.": "Този запис не съдържа моментна снимка.",
		"Copy all as JSON":                 "Копирай всичко като JSON",
		"Copy JSON":                        "Копирай JSON",
		"Copy JSON of this entry":          "Копирай JSON на този запис",
		"Copy JSON of these entries":       "Копирай JSON на тези записи",
		"Copy JSON of step %d":             "Копирай JSON на стъпка %d",
	})
}
