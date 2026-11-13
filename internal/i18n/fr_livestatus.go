// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The live connection indicator beside the tenant's name and its
// connection details dialog.
func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Live updates":     "Mises à jour en direct",
		"Live updates: %s": "Mises à jour en direct : %s",
		"Connecting":       "Connexion",
		"Reconnecting":     "Reconnexion",
		"Polling":          "Interrogation",
		"Offline":          "Hors ligne",
		"Changes made elsewhere show up here as they happen.":                                                                           "Les modifications faites ailleurs apparaissent ici dès qu’elles ont lieu.",
		"Connecting to live updates. Changes show up once it is through.":                                                               "Connexion aux mises à jour en direct. Les modifications apparaîtront une fois la connexion établie.",
		"The live connection dropped and is being tried again. Changes made meanwhile show up once it is back.":                         "La connexion en direct a été coupée et est retentée. Les modifications faites entre-temps apparaîtront dès son retour.",
		"The live connection keeps failing, so this page asks the server every 15 seconds instead. Changes can take that long to show.": "La connexion en direct échoue sans cesse, cette page interroge donc le serveur toutes les 15 secondes à la place. Les modifications peuvent mettre ce temps à apparaître.",
		"The server cannot be reached. This page keeps trying and catches up when it is back.":                                          "Le serveur est injoignable. Cette page réessaie et rattrapera son retard dès son retour.",
		"Connection details":                 "Détails de connexion",
		"Carried over":                       "Transport",
		"WebSocket":                          "WebSocket",
		"Polling over HTTP every 15 seconds": "Interrogation en HTTP toutes les 15 secondes",
		"Since":                              "Depuis",
		"Last reached the server":            "Dernier contact avec le serveur",
		"Last event":                         "Dernier événement",
		"Nothing yet.":                       "Rien pour l’instant.",
		"Failures in a row":                  "Échecs consécutifs",
		"Next try":                           "Prochain essai",
		"Last error":                         "Dernière erreur",
		"Nothing has failed since this page was loaded.": "Rien n’a échoué depuis le chargement de cette page.",
		"During":                     "Pendant",
		"Fetching the page":          "Récupération de la page",
		"HTTP status":                "Statut HTTP",
		"Close code":                 "Code de fermeture",
		"Message":                    "Message",
		"Could not reach the server": "Serveur injoignable",
		"Connection closed":          "Connexion fermée",
		"Refused by the server":      "Refusé par le serveur",
		"HTTP error":                 "Erreur HTTP",
		"What it may mean":           "Ce que cela peut signifier",
		"A proxy or load balancer may be closing idle connections. Its read timeout should be above 60 s for /api/v1/events.": "Un proxy ou un répartiteur de charge ferme peut-être les connexions inactives. Son délai de lecture doit dépasser 60 s pour /api/v1/events.",
		"The WebSocket fails while plain HTTP works, so the proxy in front of tix may not forward WebSocket upgrades.":        "Le WebSocket échoue alors que le HTTP simple fonctionne, le proxy devant tix ne transmet donc peut-être pas le passage en WebSocket.",
		"The browser could not reach the host. Check the network and that the server is running.":                             "Le navigateur n’a pas pu joindre l’hôte. Vérifiez le réseau et que le serveur fonctionne.",
		"The server refused live updates to this session. Signing in again usually fixes it.":                                 "Le serveur a refusé les mises à jour en direct à cette session. Se reconnecter règle généralement le problème.",
		"The server or a proxy answered HTTP %s. Its logs should say why.":                                                    "Le serveur ou un proxy a répondu HTTP %s. Ses journaux devraient dire pourquoi.",
		"Recent changes": "Changements récents",
		"Reconnect now":  "Reconnecter maintenant",
		"Copy details":   "Copier les détails",
		"Connection details copied. Cookies and tokens are left out.": "Détails de connexion copiés. Les cookies et jetons sont exclus.",
	})
}
