// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// The trash: the task list filtered by is:trashed.
		"Trash":            "Кошче",
		"%d deleted task":  "%d изтрита задача",
		"%d deleted tasks": "%d изтрити задачи",
		"Restore one to put it back where it was.":              "Възстановете задача, за да я върнете там, където беше.",
		"The trash is empty.":                                   "Кошчето е празно.",
		"The trash is empty":                                    "Кошчето е празно",
		"A deleted task waits here until somebody restores it.": "Изтритата задача чака тук, докато някой не я възстанови.",
		"Deleted %s": "Изтрита %s",
		"deleted":    "изтрита",
		"Restore %s": "Възстанови %s",
		"Restore":    "Възстанови",
	})
}
