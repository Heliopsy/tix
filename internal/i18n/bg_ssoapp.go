// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The page that hands a sign-in through a provider back to a native app.
func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Back to the app": "Обратно към приложението",
		"The sign-in did not complete. The app says why and lets you try again.": "Влизането не завърши. Приложението казва защо и ви позволява да опитате отново.",
		"You are signed in. Your browser is taking you back to the app.":         "Влязохте. Браузърът ви връща към приложението.",
		"Open the app": "Отвори приложението",
	})
}
