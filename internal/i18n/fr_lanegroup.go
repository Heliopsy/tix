// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Group lanes by":        "Regrouper les couloirs par",
		"Unassigned":            "Non assignée",
		"No tag":                "Aucun tag",
		"Show only these tasks": "Afficher seulement ces tâches",
		"A task with several tags is drawn once, in the first lane among its tags. Its card lists them all.":                                                                                                                                              "Une tâche avec plusieurs tags n’est dessinée qu’une fois, dans le premier couloir parmi ses tags. Sa carte les liste tous.",
		"Each lane is a project, a person, a priority, a tag or a deadline. A task with several tags is drawn once, in the first lane among its tags. The choice is in the address, so a link keeps it; the task list also remembers it in this browser.": "Chaque couloir est un projet, une personne, une priorité, un tag ou une échéance. Une tâche avec plusieurs tags n’est dessinée qu’une fois, dans le premier couloir parmi ses tags. Le choix figure dans l’adresse : un lien le conserve ; la liste des tâches le retient aussi dans ce navigateur.",
	})
}
