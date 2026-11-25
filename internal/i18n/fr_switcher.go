// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// The tenant switcher: the sidebar entry, its dialog and the Choose a
		// tenant screen.
		"Choose a tenant":           "Choisir un espace",
		"Continue here":             "Continuer ici",
		"Current":                   "Actuel",
		"Served at another address": "Servi à une autre adresse",
		"Sign in there":             "S’y connecter",
		"Sign in to %s":             "Se connecter à %s",
		"Switch":                    "Changer",
		"Switch tenant":             "Changer d’espace",
		"Switch tenant, now %s":     "Changer d’espace, actuellement %s",
		"Switch to %s":              "Passer à %s",
		"Your tenants":              "Vos espaces",
		"Tenant %s asks for a second factor, and this session was signed in without one.":                                                 "L’espace %s exige un second facteur, et cette session s’est connectée sans.",
		"You belong to more than one tenant. Pick the one to work in; the tenant button in the sidebar moves you to another at any time.": "Vous appartenez à plusieurs espaces. Choisissez celui dans lequel travailler ; le bouton d’espace dans la barre latérale vous fait passer à un autre à tout moment.",
		"You belong to one tenant only.": "Vous n’appartenez qu’à un seul espace.",
	})
}
