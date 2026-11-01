// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Snooze":                                 "Отложи",
		"Unsnooze":                               "Върни сега",
		"Snooze %s":                              "Отложи %s",
		"Snoozed until %s":                       "Отложена до %s",
		"Snoozed by %s":                          "Отложена от %s",
		"snoozed by %s":                          "отложена от %s",
		"until %s":                               "до %s",
		"Later today":                            "По-късно днес",
		"Tomorrow morning":                       "Утре сутринта",
		"Next week":                              "Следващата седмица",
		"Pick a date":                            "Изберете дата и час",
		"Hide it from every listing until later": "Скрийте я от всички списъци до по-късно",
		"A snoozed task leaves every listing, for everybody, until the time you pick, then comes back by itself. Agents cannot claim it meanwhile. is:snoozed finds it.": "Отложената задача изчезва от всички списъци, за всички, до избрания момент, и после се връща сама. Междувременно агентите не могат да я поемат. is:snoozed я намира.",
	})
}
