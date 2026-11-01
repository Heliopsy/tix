// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Inbox":                        "Входящи",
		"Inbox items":                  "Входящи известия",
		"%s assigned you":              "%s ви възложи",
		"%s commented":                 "%s коментира",
		"%s mentioned you":             "%s ви спомена",
		"%s moved it from %s to %s":    "%s я премести от %s в %s",
		"(unread)":                     "(непрочетено)",
		"All":                          "Всички",
		"Unread":                       "Непрочетени",
		"unread":                       "непрочетени",
		"Mark all read":                "Отбележи всички като прочетени",
		"Mark read":                    "Отбележи като прочетено",
		"Mark unread":                  "Отбележи като непрочетено",
		"Nothing unread.":              "Няма непрочетени.",
		"Which items to show":          "Кои известия да се показват",
		"a task that has been deleted": "изтрита задача",
		"inbox marked read":            "входящите са отбелязани като прочетени",
		"items":                        "известия",
		"somebody":                     "някой",
		"People to mention":            "Хора, които да споменете",
		"Mentions":                     "Споменавания",
		"Assignments":                  "Възлагания",
		"Status changes":               "Промени на състоянието",
		"Comments":                     "Коментари",
		"Somebody writes @you in a comment or a description.":                                                               "Някой пише @вас в коментар или описание.",
		"A task is assigned to you.":                                                                                        "Възложена ви е задача.",
		"A task assigned to you moves to another status.":                                                                   "Задача, възложена на вас, преминава в друго състояние.",
		"Somebody comments on a task you are assigned to or starred.":                                                       "Някой коментира задача, възложена на вас или отбелязана от вас със звезда.",
		"Mentions, assignments, comments and status changes that concern you, newest first.":                                "Споменавания, възлагания, коментари и промени на състоянието, които ви засягат, най-новите първи.",
		"Nothing here yet. When somebody mentions you, assigns you a task or comments on one you follow, it shows up here.": "Все още няма нищо. Когато някой ви спомене, ви възложи задача или коментира задача, която следите, ще се появи тук.",
		"What reaches your inbox. Kept with your account, so it applies everywhere you sign in.":                            "Какво стига до входящите ви. Пази се към профила ви, така че важи навсякъде, където влезете.",
		"What concerns you: somebody mentions you as @handle in a comment or a description, assigns you a task, comments on a task you are assigned to or starred, or moves a task assigned to you. Your own changes never appear here. Turn kinds off under Settings, Inbox.": "Това, което ви засяга: някой ви споменава като @име в коментар или описание, възлага ви задача, коментира задача, възложена на вас или отбелязана от вас със звезда, или премества задача, възложена на вас. Вашите собствени промени никога не се появяват тук. Видовете се изключват от Настройки, Входящи.",
	})
}
