// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The Live events screen: the tenant's raw event stream, followed live.
func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Live events": "Événements en direct",
		"Live":        "En direct",
		"The tenant's events as they happen, newest first.": "Les événements de l’espace dès qu’ils ont lieu, les plus récents d’abord.",
		"Every event this tenant emits, as clients receive it: the stream tix watch prints. Nothing here is stored; a reload starts from now unless you replay from a sequence number. Activity is the audit log, kept and searchable.": "Chaque événement émis par cet espace, tel que les clients le reçoivent : le flux qu’affiche tix watch. Rien n’est conservé ici ; un rechargement repart de maintenant, sauf si vous rejouez depuis un numéro de séquence. Activité est le journal d’audit, conservé et consultable.",
		"Changes in this tenant show up here as they happen. Earlier ones are not shown unless you replay from a sequence number.":                                                                                                      "Les modifications de cet espace apparaissent ici dès qu’elles ont lieu. Les précédentes ne sont pas affichées, sauf si vous rejouez depuis un numéro de séquence.",
		"Event types": "Types d’événements",
		"Comma separated, sent to the server. A trailing * matches a prefix, as in task.*.": "Séparés par des virgules, envoyés au serveur. Un * final correspond à un préfixe, comme dans task.*.",
		"Actors": "Acteurs",
		"Handles or ids, comma separated. Matched in this browser, as tix watch does.":                                "Identifiants ou id, séparés par des virgules. Comparés dans ce navigateur, comme le fait tix watch.",
		"This picker does not list every project in this tenant, so a project missing from it cannot be chosen here.": "Ce sélecteur ne liste pas tous les projets de cet espace, un projet absent ne peut donc pas être choisi ici.",
		"Replay from sequence number": "Rejouer depuis le numéro de séquence",
		"Shows the events after this number again, then follows live. Empty starts from now.": "Réaffiche les événements après ce numéro, puis suit en direct. Vide repart de maintenant.",
		"Follow every event":                                       "Suivre tous les événements",
		"No project has the key %s.":                               "Aucun projet n’a la clé %s.",
		"The sequence number %s is not a whole number above zero.": "Le numéro de séquence %s n’est pas un entier supérieur à zéro.",
		"project %s":                     "projet %s",
		"type %s":                        "type %s",
		"actor %s":                       "acteur %s",
		"after #%s":                      "après #%s",
		"Pause":                          "Pause",
		"Resume":                         "Reprendre",
		"Clear the list":                 "Vider la liste",
		"Event":                          "Événement",
		"Event #%d":                      "Événement #%d",
		"Details":                        "Détails",
		"Waiting for events":             "En attente d’événements",
		"Connecting to the event stream": "Connexion au flux d’événements",
		"Not following events.":          "Les événements ne sont pas suivis.",
		"Offline. Reconnecting.":         "Hors ligne. Reconnexion.",
		"Paused.":                        "En pause.",
		"Paused, 1 new event.":           "En pause, 1 nouvel événement.",
		"Paused, %d new events.":         "En pause, %d nouveaux événements.",
		"Keeping the latest %d events; older ones are dropped.":                                "Les %d derniers événements sont conservés ; les plus anciens sont abandonnés.",
		"Events after #%d are no longer kept on the server. Showing new events from now.":      "Les événements après #%d ne sont plus conservés sur le serveur. Affichage des nouveaux événements à partir de maintenant.",
		"The server refused this subscription: %s":                                             "Le serveur a refusé cet abonnement : %s",
		"This server does not stream events.":                                                  "Ce serveur ne diffuse pas d’événements.",
		"Following events needs JavaScript. The filters above still make a link you can send.": "Suivre les événements nécessite JavaScript. Les filtres ci-dessus produisent tout de même un lien que vous pouvez envoyer.",
	})
}
