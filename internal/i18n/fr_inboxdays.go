// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Summary":                                "Résumé",
		"Open":                                   "Ouvertes",
		"Due today":                              "Échéance aujourd’hui",
		"Done in 7 days":                         "Terminées en 7 jours",
		"Kinds to show":                          "Types à afficher",
		"Nothing of these kinds.":                "Rien de ces types.",
		"Yesterday":                              "Hier",
		"Earlier":                                "Plus tôt",
		"Stop mentions":                          "Arrêter les mentions",
		"Stop assignments":                       "Arrêter les assignations",
		"Stop comments":                          "Arrêter les commentaires",
		"Stop status changes":                    "Arrêter les changements de statut",
		"Mentions no longer reach your inbox":    "Les mentions n’arrivent plus dans votre boîte de réception",
		"Assignments no longer reach your inbox": "Les assignations n’arrivent plus dans votre boîte de réception",
		"Comments no longer reach your inbox":    "Les commentaires n’arrivent plus dans votre boîte de réception",
		"Status changes no longer reach your inbox":                                "Les changements de statut n’arrivent plus dans votre boîte de réception",
		"Items already in your inbox stay. Turn it back on under Settings, Inbox.": "Les éléments déjà dans votre boîte de réception restent. Réactivez-le sous Paramètres, Boîte de réception.",
	})
}
