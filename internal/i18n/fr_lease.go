// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// Lease length: the task's Lease property and the board's Claims
		// panel. A claim is a поемане and its lease a резервация, as on the
		// Tenant screen.
		"Lease":                             "Bail",
		"Lease length":                      "Durée de prise en charge",
		"Longest lease":                     "Durée maximale",
		"Inherited":                         "Hérité",
		"Claims":                            "Prises en charge",
		"Save claim length":                 "Enregistrer la durée de prise en charge",
		"claim length saved":                "durée de prise en charge enregistrée",
		"Claims last %s, set on this task":  "Les prises en charge durent %s, défini sur cette tâche",
		"Claims last %s, from the project":  "Les prises en charge durent %s, d’après le projet",
		"Claims last %s, from the workflow": "Les prises en charge durent %s, d’après le workflow",
		"Claims last %s, from the tenant":   "Les prises en charge durent %s, d’après l’espace",
		"Claims last %s, from the server configuration": "Les prises en charge durent %s, d’après la configuration du serveur",
		"Claims last %s, the built-in default":          "Les prises en charge durent %s, la valeur intégrée par défaut",
		"Claims last %s, capped by the project maximum": "Les prises en charge durent %s, plafonné par le maximum du projet",
		"Claims last %s, capped by the tenant maximum":  "Les prises en charge durent %s, plafonné par le maximum de l’espace",
		"Claims last %s, capped by the server maximum":  "Les prises en charge durent %s, plafonné par le maximum du serveur",
		"How long a claim on this task holds before it expires, when the claim names no length of its own. Empty uses the project's, then an authored workflow's, then the tenant's. Never longer than the maximum.": "La durée d’une prise en charge de cette tâche avant expiration, quand la prise en charge ne précise pas sa propre durée. Vide utilise celle du projet, puis celle d’un workflow personnalisé, puis celle de l’espace. Jamais plus longue que le maximum.",
		"How long a claim on this project's tasks holds when neither the claim nor the task says. It outranks the workflow's lease and the tenant's. Empty inherits the tenant's.":                                   "La durée d’une prise en charge des tâches de ce projet quand ni la prise en charge ni la tâche ne la précisent. Elle prime sur le bail du workflow et celui de l’espace. Vide hérite de celle de l’espace.",
		"No claim on this project holds longer, whatever it asks for. It can only be lower than the tenant's maximum.":                                                                                               "Aucune prise en charge sur ce projet ne dure plus longtemps, quoi qu’elle demande. Ne peut être que plus basse que le maximum de l’espace.",
	})
}
