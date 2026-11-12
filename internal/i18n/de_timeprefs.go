// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// Date preferences kept with the account, the tenant's default under
		// them and the server's under that.
		"Kept with your account, so every browser and terminal you sign in from shows dates this way. Inherit follows your tenant's default, which follows the server's.":                                                                                                              "Wird mit Ihrem Konto gespeichert, sodass jeder Browser und jedes Terminal, in dem Sie sich anmelden, Datumsangaben so anzeigt. Erben folgt dem Standard Ihres Mandanten, der dem des Servers folgt.",
		"The zone every timestamp on every screen is written in. Inherit uses your tenant's default, and the server's when the tenant has none.":                                                                                                                                       "Die Zeitzone, in der jeder Zeitstempel auf jedem Bildschirm angegeben wird. Erben verwendet den Standard Ihres Mandanten und den des Servers, wenn der Mandant keinen hat.",
		"How a timestamp is written, and how a date you type is read: 06.11.2026 is the 6th of November under Dotted and Day first, and 11/06/2026 is under US. 2026-11-06 is read under every format. Inherit uses your tenant's default, and the server's when the tenant has none.": "Wie ein Zeitstempel geschrieben und ein eingegebenes Datum gelesen wird: 06.11.2026 ist bei Punktiert und Tag zuerst der 6. November, und 11/06/2026 ist es bei US. 2026-11-06 wird in jedem Format gelesen. Erben verwendet den Standard Ihres Mandanten und den des Servers, wenn der Mandant keinen hat.",
		"In use:":                "Verwendet:",
		"Tenant default in use:": "Verwendeter Mandanten-Standard:",
		"from server default":    "vom Server-Standard",
		"from tenant default":    "vom Mandanten-Standard",
		"from your choice":       "von Ihrer Wahl",
		"server default":         "Server-Standard",
		"tenant default":         "Mandanten-Standard",
		"your choice":            "Ihre Wahl",
		"Inherit":                "Erben",
		"the server's own zone":  "die eigene Zeitzone des Servers",
		"How this tenant's people read timestamps unless they chose their own under Settings. Inherit uses the server's output.time_format.":      "Wie die Personen dieses Mandanten Zeitstempel lesen, sofern sie unter Einstellungen nichts Eigenes gewählt haben. Erben verwendet output.time_format des Servers.",
		"The zone this tenant's people read timestamps in unless they chose their own under Settings. Inherit uses the server's output.timezone.": "Die Zeitzone, in der die Personen dieses Mandanten Zeitstempel lesen, sofern sie unter Einstellungen nichts Eigenes gewählt haben. Erben verwendet output.timezone des Servers.",
		"Not yet": "Noch nicht",
	})
}
