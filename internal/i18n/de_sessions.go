// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Sessions":             "Sitzungen",
		"Signed-in sessions":   "Angemeldete Sitzungen",
		"Unknown client":       "Unbekannter Client",
		"This browser":         "Dieser Browser",
		"Signed in %s with %s": "Angemeldet %s mit %s",
		"and a second factor":  "und einem zweiten Faktor",
		"Expires %s.":          "Ablauf: %s.",
		"Revoke this session":  "Diese Sitzung widerrufen",
		"Revoke this session?": "Diese Sitzung widerrufen?",
		"session revoked":      "Sitzung widerrufen",
		"No sessions. You are signed in with an API token.":                                                                                                  "Keine Sitzungen. Sie sind mit einem API-Token angemeldet.",
		"Older sessions are listed by tix account sessions --cursor.":                                                                                        "Ältere Sitzungen listet tix account sessions --cursor auf.",
		"Sessions belong to a person, and this session does not speak for one, so there is nothing to manage here.":                                          "Sitzungen gehören einer Person, und diese Sitzung spricht für keine, daher gibt es hier nichts zu verwalten.",
		"Every browser and terminal sign-in you hold in this tenant. Revoke one you do not recognise, or one left signed in on a machine you no longer use.": "Jede Anmeldung per Browser und Terminal, die Sie in diesem Mandanten haben. Widerrufen Sie eine, die Sie nicht kennen, oder eine, die auf einem Rechner angemeldet blieb, den Sie nicht mehr nutzen.",
		"This is the session this browser is using, so revoking it signs you out here.":                                                                      "Dies ist die Sitzung, die dieser Browser verwendet; wenn Sie sie widerrufen, werden Sie hier abgemeldet.",
		"Whatever holds this session is signed out at once. It cannot be undone; signing in again starts a new one.":                                         "Wer diese Sitzung hält, wird sofort abgemeldet. Das lässt sich nicht rückgängig machen; eine erneute Anmeldung startet eine neue.",
	})
}
