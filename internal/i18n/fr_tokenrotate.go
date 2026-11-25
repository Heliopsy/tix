// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Rotate":           "Renouveler",
		"Rotate %s":        "Renouveler %s",
		"Rotate %s?":       "Renouveler %s ?",
		"Rotating a token": "Renouveler un jeton",
		"Old token":        "Ancien jeton",
		"retiring":         "en retrait",
		"token rotated":    "jeton renouvelé",
		"A new token with the same name, scopes and lifetime replaces this one, and its value is shown once.":                                                                                                            "Un nouveau jeton avec le même nom, les mêmes portées et la même durée de vie remplace celui-ci, et sa valeur n’est affichée qu’une fois.",
		"This token is %s's, not yours, and the trail will record that you rotated somebody else's.":                                                                                                                     "Ce jeton appartient à %s, pas à vous, et la trace enregistrera que vous avez renouvelé celui de quelqu’un d’autre.",
		"Rotate a token on a schedule, or at once when it may have leaked. Whatever holds it has to be given the new value; a grace window keeps the old one working meanwhile, and revoking it ends that window early.": "Renouvelez un jeton selon un calendrier, ou immédiatement s’il a pu fuiter. Ce qui le détient doit recevoir la nouvelle valeur ; une période de grâce garde l’ancien fonctionnel entre-temps, et le révoquer met fin à cette période plus tôt.",
		"How long the old value keeps authenticating beside the new one, up to a day, so a running agent can pick up the new value without failing a request. After a leak, stop it now.":                                "Combien de temps l’ancienne valeur reste valide à côté de la nouvelle, jusqu’à un jour, pour qu’un agent en cours puisse récupérer la nouvelle valeur sans faire échouer de requête. Après une fuite, arrêtez-le maintenant.",
		"Stop the old token now":         "Arrêter l’ancien jeton maintenant",
		"Keep it working for 10 minutes": "Le garder actif 10 minutes",
		"Keep it working for 1 hour":     "Le garder actif 1 heure",
		"Keep it working for 24 hours":   "Le garder actif 24 heures",
	})
}
