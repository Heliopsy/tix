// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The page that hands a sign-in through a provider back to a native app.
func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Back to the app": "Retour à l’application",
		"The sign-in did not complete. The app says why and lets you try again.": "La connexion n’a pas abouti. L’application indique pourquoi et vous permet de réessayer.",
		"You are signed in. Your browser is taking you back to the app.":         "Vous êtes connecté. Votre navigateur vous ramène à l’application.",
		"Open the app": "Ouvrir l’application",
	})
}
