// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Revoke %s?":       "Révoquer %s ?",
		"Remove %s?":       "Retirer %s ?",
		"Archive %s?":      "Archiver %s ?",
		"Clean up %s now?": "Nettoyer %s maintenant ?",
		"Settings for %s":  "Paramètres de %s",
		"Nothing is subscribed to this tenant's events.":                                     "Rien n’est abonné aux événements de cet espace.",
		"Which key each scheme uses for each action.":                                        "Quelle touche chaque schéma utilise pour chaque action.",
		"Archiving hides %s from active lists and keeps its tasks.":                          "L’archivage masque %s des listes actives et conserve ses tâches.",
		"Deleting removes the project for good. You choose what happens to its tasks first.": "La suppression retire le projet définitivement. Vous choisissez d’abord ce que deviennent ses tâches.",
		"Deleting removes the account and its membership. It cannot be undone.":              "La suppression retire le compte et son appartenance. C’est irréversible.",
		"What this project is called and how it looks, how long a claim on its tasks holds, how it cleans up its finished tasks and the trash, and archiving or deleting it.":                                                                                                                                                                                                                                      "Le nom du projet et son apparence, la durée d’une prise en charge de ses tâches, la façon dont il nettoie ses tâches terminées et la corbeille, et son archivage ou sa suppression.",
		"A row appears when an import <strong>finishes</strong>, read back from the audit entry it wrote. A <strong>dry run</strong> writes nothing, so it leaves no row, and a <strong>failed</strong> run stops without writing one: it shows as its source's status in the second table. A row cannot show where the cursor stood after its run; the second table shows where each cursor stands <em>now</em>.": "Une ligne apparaît quand un import <strong>se termine</strong>, relue depuis l’entrée d’audit qu’il a écrite. Une <strong>simulation</strong> n’écrit rien et ne laisse donc aucune ligne, et une exécution <strong>échouée</strong> s’arrête sans en écrire : elle apparaît dans le statut de sa source, dans le second tableau. Une ligne ne peut pas montrer où se trouvait le curseur après son exécution ; le second tableau montre où se trouve chaque curseur <em>maintenant</em>.",
		"The tasks an import created or updated are audited one by one, in <a href=\"/activity?source=system\">activity</a>, attributed to the System actor.":                                                                                                                                                                                                                                                      "Les tâches qu’un import a créées ou mises à jour sont auditées une par une, dans l’<a href=\"/activity?source=system\">activité</a>, attribuées à l’acteur System.",
	})
}
