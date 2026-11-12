// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Sessions":             "Sessions",
		"Signed-in sessions":   "Sessions connectées",
		"Unknown client":       "Client inconnu",
		"This browser":         "Ce navigateur",
		"Signed in %s with %s": "Connecté %s avec %s",
		"and a second factor":  "et un second facteur",
		"Expires %s.":          "Expire %s.",
		"Revoke this session":  "Révoquer cette session",
		"Revoke this session?": "Révoquer cette session ?",
		"session revoked":      "session révoquée",
		"No sessions. You are signed in with an API token.":                                                                                                  "Aucune session. Vous êtes connecté avec un jeton d’API.",
		"Older sessions are listed by tix account sessions --cursor.":                                                                                        "Les sessions plus anciennes sont listées par tix account sessions --cursor.",
		"Sessions belong to a person, and this session does not speak for one, so there is nothing to manage here.":                                          "Les sessions appartiennent à une personne, et cette session n’en représente aucune, il n’y a donc rien à gérer ici.",
		"Every browser and terminal sign-in you hold in this tenant. Revoke one you do not recognise, or one left signed in on a machine you no longer use.": "Chaque connexion par navigateur et terminal que vous détenez dans cet espace. Révoquez celle que vous ne reconnaissez pas, ou celle restée connectée sur une machine que vous n’utilisez plus.",
		"This is the session this browser is using, so revoking it signs you out here.":                                                                      "C’est la session utilisée par ce navigateur, la révoquer vous déconnecte donc ici.",
		"Whatever holds this session is signed out at once. It cannot be undone; signing in again starts a new one.":                                         "Ce qui détient cette session est déconnecté immédiatement. C’est irréversible ; se reconnecter en ouvre une nouvelle.",
	})
}
