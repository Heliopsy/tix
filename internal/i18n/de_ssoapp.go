// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The page that hands a sign-in through a provider back to a native app.
func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Back to the app": "Zurück zur App",
		"The sign-in did not complete. The app says why and lets you try again.": "Die Anmeldung wurde nicht abgeschlossen. Die App nennt den Grund und lässt Sie es erneut versuchen.",
		"You are signed in. Your browser is taking you back to the app.":         "Sie sind angemeldet. Ihr Browser bringt Sie zurück zur App.",
		"Open the app": "App öffnen",
	})
}
