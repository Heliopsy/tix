// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// The quick actions on a task: Assign to me and the due date field in
		// the task, row and lane card menus, and the toasts they answer with.
		"Assign to me":       "M’assigner",
		"Clear the due date": "Retirer l’échéance",
		"assigned to you":    "vous est assignée",
		"assignee changed":   "responsable modifié",
		"assignee cleared":   "responsable retiré",
		"due date set":       "échéance définie",
		"due date cleared":   "échéance retirée",
	})
}
