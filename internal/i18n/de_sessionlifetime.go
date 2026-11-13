// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Keep me signed in":       "Angemeldet bleiben",
		"Kept signed in.":         "Angemeldet geblieben.",
		"Sign-in lasts":           "Anmeldung gilt",
		"Sign out when idle for":  "Abmelden nach Inaktivität von",
		"Keep me signed in lasts": "Angemeldet bleiben gilt",
		"never":                   "nie",
		"Leave any empty to use the server's setting, shown greyed.": "Lassen Sie ein Feld leer, um die grau angezeigte Einstellung des Servers zu verwenden.",
		"End a session nobody has used for this long, such as 30m, kept signed in or not. It applies to sessions already signed in as soon as it is saved. It can be shorter than the server's but not longer; empty with no server setting means never.": "Beendet eine Sitzung, die so lange niemand benutzt hat, etwa 30m, ob mit Angemeldet bleiben oder ohne. Gilt für bereits angemeldete Sitzungen, sobald gespeichert wird. Kann kürzer sein als die des Servers, aber nicht länger; leer ohne Servereinstellung bedeutet nie.",
		"How long a sign-in lasts when the person ticks Keep me signed in, such as 7d, or off to stop offering it. It can be shorter than the server's but not longer.":                                                                                   "Wie lange eine Anmeldung gilt, wenn die Person Angemeldet bleiben ankreuzt, etwa 7d, oder off, um es nicht anzubieten. Kann kürzer sein als die des Servers, aber nicht länger.",
	})
}
