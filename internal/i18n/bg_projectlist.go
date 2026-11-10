// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// A project's page: the task screen narrowed to one project, and the
		// view switch whose links keep the filter.
		"The same tasks, drawn three ways. List is one row per task. Board is one column per workflow state, so it needs the listed projects to share their states. Lanes is one lane per project, person, priority, tag or deadline, with finished tasks ticked in place. Switching keeps the filter, the sort and the project.": "Едни и същи задачи, показани по три начина. Списъкът е по един ред за задача. Таблото е по една колона за състояние на работния процес, затова изисква показаните проекти да имат еднакви състояния. Лентите са по една лента за проект, човек, приоритет, етикет или краен срок, като завършените задачи остават на място, отметнати. При превключване филтърът, подреждането и проектът се запазват.",
	})
}
