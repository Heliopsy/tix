// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// Task templates.
		"Templates":                    "Modèles",
		"Template":                     "Modèle",
		"Template name":                "Nom du modèle",
		"Task templates":               "Modèles de tâche",
		"Save as template":             "Enregistrer comme modèle",
		"Save template":                "Enregistrer le modèle",
		"Delete template":              "Supprimer le modèle",
		"New from template":            "Nouvelle à partir d’un modèle",
		"From template":                "À partir d’un modèle",
		"All templates…":               "Tous les modèles…",
		"All projects":                 "Tous les projets",
		"This project":                 "Ce projet",
		"Use in":                       "Utiliser dans",
		"Also keep":                    "Garder aussi",
		"Due in":                       "Échéance dans",
		"Use":                          "Utiliser",
		"Uses":                         "Utilisations",
		"Used %d times":                "Utilisé %d fois",
		"Actions":                      "Actions",
		"Task title":                   "Titre de la tâche",
		"The template's project":       "Le projet du modèle",
		"The template's title":         "Le titre du modèle",
		"3d":                           "3d",
		"template saved":               "modèle enregistré",
		"template deleted":             "modèle supprimé",
		"saved as a template":          "enregistrée comme modèle",
		"task created from a template": "tâche créée à partir d’un modèle",
		"No templates yet. Open a task and pick Save as template.": "Aucun modèle pour l’instant. Ouvrez une tâche et choisissez Enregistrer comme modèle.",
		"Tasks already made from this template stay as they are.":  "Les tâches déjà créées à partir de ce modèle restent telles quelles.",
		"Titles may carry placeholders, filled in each time a template is used: {DATE} is today's date, {N} the use number and {PROJECT} the project's key.":                                                                                                                                                                               "Les titres peuvent contenir des espaces réservés, remplis à chaque utilisation d’un modèle : {DATE} est la date du jour, {N} le numéro d’utilisation et {PROJECT} la clé du projet.",
		"A template is a named shape of a task: title, description, priority, tags, custom fields, repeat rule and, when saved with them, subtasks. It is not a task, so it never shows in a listing, a claim or the statistics. Save one from a task's Save as template, and create tasks from it with New from template in the add box.": "Un modèle est la forme nommée d’une tâche : titre, description, priorité, tags, champs personnalisés, règle de répétition et, s’il est enregistré avec elles, sous-tâches. Ce n’est pas une tâche, il n’apparaît donc jamais dans une liste, une prise ni les statistiques. Enregistrez-en un depuis Enregistrer comme modèle d’une tâche, et créez des tâches à partir de lui avec Nouvelle à partir d’un modèle dans la zone d’ajout.",
		"Changes apply to tasks made from now on; tasks already made from the template keep what they were made with. Subtasks change only by saving a task as a template again.":                                                                                                                                                          "Les modifications s’appliquent aux tâches créées à partir de maintenant ; les tâches déjà créées à partir du modèle gardent ce avec quoi elles ont été créées. Les sous-tâches ne changent qu’en enregistrant à nouveau une tâche comme modèle.",
		"Creates a task, and a subtask for each one the template holds. It starts in the project's first state and keeps the custom fields that project defines.":                                                                                                                                                                          "Crée une tâche, et une sous-tâche pour chacune de celles du modèle. Elle démarre dans le premier état du projet et garde les champs personnalisés que ce projet définit.",
		"The template keeps this task's title, description, priority, tags, custom fields and repeat rule, and its subtasks when ticked. Status, assignee, due date, comments and history stay with the task. Templates never show in a listing; find them under Templates.":                                                               "Le modèle garde le titre, la description, la priorité, les tags, les champs personnalisés et la règle de répétition de cette tâche, et ses sous-tâches si la case est cochée. Le statut, le responsable, l’échéance, les commentaires et l’historique restent avec la tâche. Les modèles n’apparaissent jamais dans une liste ; retrouvez-les sous Modèles.",
		"Each task made from the template falls due this long after it is made, such as 3d or 12h. Leave it empty for no due date.":                                                                                                                                                                                                        "Chaque tâche créée à partir du modèle arrive à échéance ce délai après sa création, par exemple 3d ou 12h. Laissez vide pour ne pas avoir d’échéance.",
		"How long from now the task falls due, such as 3d or 12h. Leave it empty for the template's own offset; 0 sets no due date.":                                                                                                                                                                                                       "Dans combien de temps la tâche arrive à échéance, par exemple 3d ou 12h. Laissez vide pour le délai propre au modèle ; 0 ne définit pas d’échéance.",
		"Leave it empty for the template's own title. Placeholders work here too: {DATE} is today's date, {N} the use number and {PROJECT} the project's key.":                                                                                                                                                                             "Laissez vide pour le titre propre au modèle. Les espaces réservés fonctionnent ici aussi : {DATE} est la date du jour, {N} le numéro d’utilisation et {PROJECT} la clé du projet.",
		"One key=value per line. A line removed removes the field. The project a task lands in keeps the fields it defines and drops the rest.":                                                                                                                                                                                            "Une clé=valeur par ligne. Une ligne retirée retire le champ. Le projet dans lequel arrive une tâche garde les champs qu’il définit et abandonne les autres.",
		"Placeholders are filled in each time the template is used, in any case: {DATE} is today's date as YYYY-MM-DD, {N} the use number and {PROJECT} the key of the project the task lands in.":                                                                                                                                         "Les espaces réservés sont remplis à chaque utilisation du modèle, dans tous les cas : {DATE} est la date du jour au format YYYY-MM-DD, {N} le numéro d’utilisation et {PROJECT} la clé du projet dans lequel arrive la tâche.",
	})
}
