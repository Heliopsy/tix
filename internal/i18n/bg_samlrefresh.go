// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{
		"%d retiring certificates":                                           "%d сертификата в оттегляне",
		"IdP metadata not read yet":                                          "Метаданните на IdP още не са прочетени",
		"IdP metadata read failed":                                           "Четенето на метаданните на IdP не успя",
		"IdP metadata read, certificates updated":                            "Метаданните на IdP са прочетени, сертификатите са обновени",
		"IdP metadata read, nothing changed":                                 "Метаданните на IdP са прочетени, нищо не се промени",
		"Read the IdP metadata again now and merge its signing certificates": "Прочита метаданните на IdP отново сега и добавя сертификатите им за подпис",
		"Refresh metadata":                                                   "Обнови метаданните",
		"metadata refresh failed":                                            "обновяването на метаданните не успя",
		"metadata refreshed, certificates updated":                           "метаданните са обновени, сертификатите са променени",
		"metadata refreshed, nothing changed":                                "метаданните са обновени, нищо не се промени",
		"next read":                                                          "следващо четене",
		"not read on a schedule":                                             "не се чете по график",
		"the stored certificates keep working":                               "запазените сертификати продължават да работят",
	})
}
