// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// Housekeeping: the cleanup rules on the Tenant screen and in a
		// project's settings.
		"Housekeeping":           "Entretien",
		"Save housekeeping":      "Enregistrer l’entretien",
		"Clean up now":           "Nettoyer maintenant",
		"Done tasks":             "Tâches terminées",
		"Cancelled tasks":        "Tâches annulées",
		"After a number of days": "Après un nombre de jours",
		"days":                   "jours",
		"Inherit: off":           "Hériter : désactivé",
		"Inherit: %d day":        "Hériter : %d jour",
		"Inherit: %d days":       "Hériter : %d jours",
		"%d task would be moved to the trash now.":  "%d tâche serait placée dans la corbeille maintenant.",
		"%d tasks would be moved to the trash now.": "%d tâches seraient placées dans la corbeille maintenant.",
		"%d task would be deleted for good now.":    "%d tâche serait supprimée définitivement maintenant.",
		"%d tasks would be deleted for good now.":   "%d tâches seraient supprimées définitivement maintenant.",
		"cleanup saved": "nettoyage enregistré",
		"cleanup ran":   "nettoyage effectué",
		"cleanup ran: %d moved to the trash, %d deleted for good":                  "nettoyage effectué : %d placées dans la corbeille, %d supprimées définitivement",
		"Cleanup acts on this project only: %d to the trash, %d deleted for good.": "Le nettoyage n’agit que sur ce projet : %d vers la corbeille, %d supprimées définitivement.",
		"Clean up this project":                    "Nettoyer ce projet",
		"Paused while the trash is off.":           "En pause tant que la corbeille est désactivée.",
		"Nothing to clean up now.":                 "Rien à nettoyer maintenant.",
		"cleanup is paused while the trash is off": "le nettoyage est en pause tant que la corbeille est désactivée",
		"The trash is off, so cleanup is paused: nothing is moved or deleted until the trash is on again.":                                                                                                                     "La corbeille est désactivée, le nettoyage est donc en pause : rien n’est déplacé ni supprimé tant que la corbeille n’est pas réactivée.",
		"Overrides the tenant's cleanup defaults for this project. Inherit follows the Tenant screen.":                                                                                                                         "Remplace les valeurs de nettoyage par défaut de l’espace pour ce projet. Hériter suit l’écran Espace.",
		"Cleans up finished tasks and the trash on its own, on the same schedule as retention. These are the defaults for every project; a project can override each one in its settings. Everything is off until you set it.": "Nettoie de lui-même les tâches terminées et la corbeille, au même rythme que la conservation. Ce sont les valeurs par défaut de chaque projet ; un projet peut remplacer chacune dans ses paramètres. Tout est désactivé tant que vous ne l’avez pas défini.",
		"Tasks that have sat in a done state this many days go to the trash, where they can be restored. A done task with open subtasks stays until they are finished too. Cannot be turned on while the trash is off.":        "Les tâches restées dans un état terminé pendant ce nombre de jours vont dans la corbeille, où elles peuvent être restaurées. Une tâche terminée avec des sous-tâches ouvertes reste jusqu’à ce qu’elles soient terminées aussi. Ne peut pas être activé tant que la corbeille est désactivée.",
		"Tasks that have sat in a cancelled state this many days go to the trash, where they can be restored. Set apart from done tasks, because abandoned work is usually worth less to keep.":                                "Les tâches restées dans un état annulé pendant ce nombre de jours vont dans la corbeille, où elles peuvent être restaurées. Séparé des tâches terminées, car le travail abandonné vaut généralement moins la peine d’être conservé.",
		"Tasks deleted more than this many days ago are deleted for good, subtasks first. This cannot be undone.":                                                                                                              "Les tâches supprimées il y a plus de ce nombre de jours sont supprimées définitivement, sous-tâches d’abord. C’est irréversible.",
	})
}
