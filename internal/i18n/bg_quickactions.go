// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// The quick actions on a task: Assign to me and the due date field in
		// the task, row and lane card menus, and the toasts they answer with.
		"Assign to me":       "Възложи на мен",
		"Clear the due date": "Премахни крайния срок",
		"assigned to you":    "възложена на вас",
		"assignee changed":   "изпълнителят е сменен",
		"assignee cleared":   "изпълнителят е премахнат",
		"due date set":       "крайният срок е зададен",
		"due date cleared":   "крайният срок е премахнат",
	})
}
