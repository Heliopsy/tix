// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// A project's page: the task screen narrowed to one project, and the
		// view switch whose links keep the filter.
		"The same tasks, drawn three ways. List is one row per task. Board is one column per workflow state, so it needs the listed projects to share their states. Lanes is one lane per project, person, priority, tag or deadline, with finished tasks ticked in place. Switching keeps the filter, the sort and the project.": "Les mêmes tâches, présentées de trois façons. Liste affiche une ligne par tâche. Tableau affiche une colonne par état du workflow, donc les projets listés doivent partager leurs états. Couloirs affiche un couloir par projet, personne, priorité, tag ou échéance, avec les tâches terminées cochées sur place. Changer de vue conserve le filtre, le tri et le projet.",
	})
}
