// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// How a project is named in a picker.
		"Project names in pickers":                  "Имена на проекти в списъците за избор",
		"How a project reads where you choose one.": "Как изглежда проектът там, където го избирате.",
		"How a project reads in every list you choose one from: the add box, Add with details, Copy to, New from template, a heading's New task in, the move target when a project is deleted, and the statistics filter. The name with its key, the key first, the name alone, or the key alone. A long name is shortened; the key never is.": "Как изглежда проектът във всеки списък, от който го избирате: полето за добавяне, Добавяне с подробности, Копиране в, Нова от шаблон, Нова задача в от менюто на заглавие, проектът, в който отиват задачите при изтриване на проект, и филтърът на статистиката. Името с ключа, първо ключът, само името или само ключът. Дългото име се съкращава; ключът никога.",
		"Apply project names": "Приложи имената на проекти",
		"Name (KEY)":          "Име (КЛЮЧ)",
		"KEY · Name":          "КЛЮЧ · Име",
		"Name":                "Име",
		"KEY":                 "КЛЮЧ",
	})
}
