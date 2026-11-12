// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// A project's page: the task screen narrowed to one project, and the
		// view switch whose links keep the filter.
		"The same tasks, drawn three ways. List is one row per task. Board is one column per workflow state, so it needs the listed projects to share their states. Lanes is one lane per project, person, priority, tag or deadline, with finished tasks ticked in place. Switching keeps the filter, the sort and the project.": "Dieselben Aufgaben, auf drei Arten dargestellt. Liste zeigt eine Zeile pro Aufgabe. Board zeigt eine Spalte pro Workflow-Zustand und setzt daher voraus, dass die angezeigten Projekte dieselben Zustände haben. Bahnen zeigt eine Bahn pro Projekt, Person, Priorität, Tag oder Frist, erledigte Aufgaben bleiben abgehakt an ihrem Platz. Beim Wechseln bleiben Filter, Sortierung und Projekt erhalten.",
	})
}
