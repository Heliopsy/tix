// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Snooze":           "Отложи",
		"Unsnooze":         "Върни сега",
		"Snoozed until %s": "Отложена до %s",
		"Snoozed by %s":    "Отложена от %s",
		"snoozed by %s":    "отложена от %s",
		"until %s":         "до %s",
		"Later today":      "По-късно днес",
		"Tomorrow morning": "Утре сутринта",
		"Next week":        "Следващата седмица",
		"Pick a date":      "Изберете дата и час",
	})
}
