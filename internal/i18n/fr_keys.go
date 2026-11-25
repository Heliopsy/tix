// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The keyboard shortcuts the web shares with the app: the help overlay's
// groups and the actions added with the shared key table.
func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Navigate": "Navigation",
		"Lists":    "Listes",
		"Task":     "Tâche",
		"General":  "Général",

		"Go to Inbox":      "Aller à la boîte de réception",
		"Go to My work":    "Aller à Mon travail",
		"Go to Tasks":      "Aller aux tâches",
		"Go to Projects":   "Aller aux projets",
		"Go to Activity":   "Aller à l'activité",
		"Go to Settings":   "Aller aux paramètres",
		"Go to Statistics": "Aller aux statistiques",

		"Page down":   "Page suivante",
		"Page up":     "Page précédente",
		"Refresh":     "Actualiser",
		"Switch view": "Changer de vue",

		"Mark done":              "Marquer comme terminée",
		"Edit the description":   "Modifier la description",
		"Move to another status": "Passer à un autre statut",
		"Star or unstar":         "Ajouter ou retirer des favoris",
		"Set the priority":       "Définir la priorité",
		"Set the due date":       "Définir l'échéance",
		"Snooze or unsnooze":     "Reporter ou ne plus reporter",
		"Copy the reference":     "Copier la référence",
		"Copied %s":              "%s copié",
	})
}
