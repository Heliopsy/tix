// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// The tenant switcher: the sidebar entry, its dialog and the Choose a
		// tenant screen.
		"Choose a tenant":           "Mandant wählen",
		"Continue here":             "Hier fortfahren",
		"Current":                   "Aktuell",
		"Served at another address": "Unter einer anderen Adresse bereitgestellt",
		"Sign in there":             "Dort anmelden",
		"Sign in to %s":             "Bei %s anmelden",
		"Switch":                    "Wechseln",
		"Switch tenant":             "Mandant wechseln",
		"Switch tenant, now %s":     "Mandant wechseln, aktuell %s",
		"Switch to %s":              "Zu %s wechseln",
		"Your tenants":              "Ihre Mandanten",
		"Tenant %s asks for a second factor, and this session was signed in without one.":                                                 "Mandant %s verlangt einen zweiten Faktor, und diese Sitzung wurde ohne einen angemeldet.",
		"You belong to more than one tenant. Pick the one to work in; the tenant button in the sidebar moves you to another at any time.": "Sie gehören zu mehr als einem Mandanten. Wählen Sie den, in dem Sie arbeiten möchten; über die Mandanten-Schaltfläche in der Seitenleiste wechseln Sie jederzeit zu einem anderen.",
		"You belong to one tenant only.": "Sie gehören nur zu einem Mandanten.",
	})
}
