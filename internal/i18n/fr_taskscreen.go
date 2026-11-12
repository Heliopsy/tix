// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Properties":                      "Propriétés",
		"Claim":                           "Prendre",
		"Created by":                      "Créée par",
		"Snoozed":                         "Reportée",
		"Snooze…":                         "Reporter…",
		"Reclaim the lease…":              "Reprendre le bail…",
		"Does not repeat":                 "Ne se répète pas",
		"No subtasks.":                    "Aucune sous-tâche.",
		"Waits for nothing.":              "N’attend rien.",
		"Depends on, for example infra-2": "Dépend de, par exemple infra-2",
		"Remove the dependency on %s":     "Retirer la dépendance à %s",
		"in":                              "dans",
	})
}
