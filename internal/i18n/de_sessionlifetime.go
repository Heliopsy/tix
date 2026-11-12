// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Keep me signed in":         "Angemeldet bleiben",
		"Up to %s on this browser.": "Bis zu %s in diesem Browser.",
		"Kept signed in.":           "Angemeldet geblieben.",
		"Sign-in lasts":             "Anmeldung gilt",
		"Sign out when idle for":    "Abmelden nach Inaktivität von",
		"Keep me signed in lasts":   "Angemeldet bleiben gilt",
		"never":                     "nie",
		"Leave any empty to use the server's setting, shown greyed.": "Lassen Sie ein Feld leer, um die grau angezeigte Einstellung des Servers zu verwenden.",
		"Stay signed in on this browser after it closes, until the time shown. Leave it unticked on a shared computer. If this tenant ends sessions left unused, that still applies.":                                                                     "Bleiben Sie in diesem Browser auch nach dem Schließen angemeldet, bis zur angezeigten Zeit. Auf einem gemeinsam genutzten Computer nicht ankreuzen. Wenn dieser Mandant unbenutzte Sitzungen beendet, gilt das weiterhin.",
		"How long a sign-in on this tenant lasts from the moment it is made, such as 8h. It can be shorter than the server's but not longer. Sessions already signed in keep the expiry they were given.":                                                 "Wie lange eine Anmeldung in diesem Mandanten ab ihrem Zeitpunkt gilt, etwa 8h. Sie kann kürzer sein als die des Servers, aber nicht länger. Bereits angemeldete Sitzungen behalten ihren Ablauf.",
		"End a session nobody has used for this long, such as 30m, kept signed in or not. It applies to sessions already signed in as soon as it is saved. It can be shorter than the server's but not longer; empty with no server setting means never.": "Beendet eine Sitzung, die so lange niemand benutzt hat, etwa 30m, ob mit Angemeldet bleiben oder ohne. Gilt für bereits angemeldete Sitzungen, sobald gespeichert wird. Kann kürzer sein als die des Servers, aber nicht länger; leer ohne Servereinstellung bedeutet nie.",
		"How long a sign-in lasts when the person ticks Keep me signed in, such as 7d, or off to stop offering it. It can be shorter than the server's but not longer.":                                                                                   "Wie lange eine Anmeldung gilt, wenn die Person Angemeldet bleiben ankreuzt, etwa 7d, oder off, um es nicht anzubieten. Kann kürzer sein als die des Servers, aber nicht länger.",
	})
}
