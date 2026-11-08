// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// A project's page: the task screen narrowed to one project, and the
		// view switch whose links keep the filter.
		"Project page": "Страница на проекта",
		"This project's tasks, as a list, a board or lanes, with the same filter, sort and View menu as the task list. The address keeps the filter and the view, so a link to it opens the same screen.":                                                                                      "Задачите на този проект като списък, табло или ленти, със същия филтър, подреждане и меню „Изглед“ като в списъка със задачи. Адресът пази филтъра и изгледа, така че връзка към него отваря същия екран.",
		"The same tasks, drawn three ways. List is one row per task. Board is one column per workflow state, so it needs the listed projects to share their states. Lanes is one lane per project, with finished tasks ticked in place. Switching keeps the filter, the sort and the project.": "Едни и същи задачи, показани по три начина. Списъкът е по един ред за задача. Таблото е по една колона за състояние на работния процес, затова изисква показаните проекти да имат еднакви състояния. Лентите са по една лента за проект, като завършените задачи остават на място, отметнати. При превключване филтърът, подреждането и проектът се запазват.",
	})
}
