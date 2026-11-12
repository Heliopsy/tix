// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// The trash: the task list filtered by is:trashed.
		"Trash":            "Corbeille",
		"%d deleted task":  "%d tâche supprimée",
		"%d deleted tasks": "%d tâches supprimées",
		"Restore one to put it back where it was.":              "Restaurez-en une pour la remettre à sa place.",
		"The trash is empty.":                                   "La corbeille est vide.",
		"The trash is empty":                                    "La corbeille est vide",
		"A deleted task waits here until somebody restores it.": "Une tâche supprimée attend ici jusqu’à ce que quelqu’un la restaure.",
		"Deleted %s":             "Supprimée %s",
		"deleted":                "supprimée",
		"Restore %s":             "Restaurer %s",
		"Restore":                "Restaurer",
		"Delete %s permanently":  "Supprimer %s définitivement",
		"Delete %s permanently?": "Supprimer %s définitivement ?",
		"%s and its subtasks in the trash are deleted for good, with their comments, links and attachments. It cannot be undone; the audit log keeps the record.": "%s et ses sous-tâches dans la corbeille sont supprimées pour de bon, avec leurs commentaires, liens et pièces jointes. C’est irréversible ; le journal d’audit en garde la trace.",

		// A tenant whose trash is turned off: the confirmation that stands in
		// for the undo, and the setting on the Tenant screen.
		"task deleted permanently":    "tâche supprimée définitivement",
		"Delete %d task permanently":  "Supprimer %d tâche définitivement",
		"Delete %d tasks permanently": "Supprimer %d tâches définitivement",
		"Trash is off":                "Corbeille désactivée",
		"This tenant's trash is turned off, so a deleted task is removed for good: it cannot be restored and there is no undo. A tenant admin can turn the trash back on from the Tenant screen.": "La corbeille de cet espace est désactivée, donc une tâche supprimée l’est pour de bon : elle ne peut pas être restaurée et il n’y a pas d’annulation. Un administrateur de l’espace peut réactiver la corbeille depuis l’écran Espace.",
		"Delete %s?": "Supprimer %s ?",
		"The trash is turned off for this tenant, so this cannot be undone.": "La corbeille est désactivée pour cet espace, cette action est donc irréversible.",
		"Their subtasks are deleted with them.":                              "Leurs sous-tâches sont supprimées avec elles.",
		"On: deleted tasks can be restored":                                  "Activée : les tâches supprimées peuvent être restaurées",
		"Off: deleting is permanent":                                         "Désactivée : la suppression est définitive",
		"On, a deleted task goes to the Trash, where anyone who could delete it can restore it, and every delete offers Undo. Off, deleting a task removes it for good: each delete asks first, nothing can be restored, and the Trash leaves the sidebar. Tasks already in the trash when it is turned off are kept, out of reach, and come back when it is turned on again.": "Activée, une tâche supprimée va dans la Corbeille, où quiconque pouvait la supprimer peut la restaurer, et chaque suppression propose Annuler. Désactivée, supprimer une tâche l’efface pour de bon : chaque suppression demande d’abord confirmation, rien ne peut être restauré, et la Corbeille quitte la barre latérale. Les tâches déjà dans la corbeille au moment de la désactivation sont conservées, hors d’atteinte, et reviennent quand elle est réactivée.",
		"%d task deleted while the trash was on is kept out of reach until it is turned back on.":                 "%d tâche supprimée pendant que la corbeille était activée est conservée hors d’atteinte jusqu’à sa réactivation.",
		"%d tasks deleted while the trash was on are kept out of reach until it is turned back on.":               "%d tâches supprimées pendant que la corbeille était activée sont conservées hors d’atteinte jusqu’à sa réactivation.",
		"%d task is in the trash. Turning the trash off keeps it, out of reach, until the trash is on again.":     "%d tâche est dans la corbeille. Désactiver la corbeille la conserve, hors d’atteinte, jusqu’à ce que la corbeille soit réactivée.",
		"%d tasks are in the trash. Turning the trash off keeps them, out of reach, until the trash is on again.": "%d tâches sont dans la corbeille. Désactiver la corbeille les conserve, hors d’atteinte, jusqu’à ce que la corbeille soit réactivée.",
	})
}
