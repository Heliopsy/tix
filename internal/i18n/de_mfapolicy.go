// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"This tenant requires a second factor. Set one up to go on.": "Dieser Mandant verlangt einen zweiten Faktor. Richten Sie einen ein, um fortzufahren.",
		"This tenant requires a second factor from %s.":              "Dieser Mandant verlangt ab %s einen zweiten Faktor.",
		"Set up a second factor":                                     "Zweiten Faktor einrichten",
		"This tenant requires a second factor, and this account has none yet. Until you add one you can only set it up, read your account's history or sign out.": "Dieser Mandant verlangt einen zweiten Faktor, und dieses Konto hat noch keinen. Bis Sie einen hinzufügen, können Sie ihn nur einrichten, den Verlauf Ihres Kontos lesen oder sich abmelden.",
		"This tenant requires a second factor from %s. Add one now and nothing changes for you then.":                                                             "Dieser Mandant verlangt ab %s einen zweiten Faktor. Fügen Sie jetzt einen hinzu, dann ändert sich für Sie nichts.",
		"A second factor asks for a code from your phone after your password, so a leaked password alone does not sign anybody in.":                               "Ein zweiter Faktor fragt nach Ihrem Passwort einen Code von Ihrem Telefon ab, sodass ein geleaktes Passwort allein niemanden anmeldet.",
		"You need an authenticator app on your phone, such as one from your phone's maker or any TOTP app.":                                                       "Sie brauchen eine Authenticator-App auf Ihrem Telefon, etwa die des Herstellers oder eine beliebige TOTP-App.",
		"Everybody":                "Alle",
		"Without a second factor":  "Ohne zweiten Faktor",
		"2FA":                      "2FA",
		"no 2FA":                   "keine 2FA",
		"Second factor policy":     "Richtlinie für den zweiten Faktor",
		"Off: each person chooses": "Aus: jede Person entscheidet",
		"Required for everybody":   "Für alle verpflichtend",
		"Grace period":             "Übergangsfrist",
		"People without a second factor are refused from %s.": "Personen ohne zweiten Faktor werden ab %s abgewiesen.",
		"Who has none yet": "Wer noch keinen hat",
		"Required: on every surface, a person with no second factor can only set one up, read their own account or sign out, with a session or with a token of theirs. Tokens of agents are not affected. You need a second factor yourself before you can turn it on.": "Verpflichtend: Auf jeder Oberfläche kann eine Person ohne zweiten Faktor nur einen einrichten, ihr eigenes Konto lesen oder sich abmelden, mit einer Sitzung oder einem eigenen Token. Tokens von Agenten sind nicht betroffen. Sie brauchen selbst einen zweiten Faktor, bevor Sie dies einschalten können.",
		"How long people without a second factor may keep working after the policy is turned on, with a banner naming the deadline. Written as a duration: 7d, 48h. Empty or 0 applies it at once.":                                                                     "Wie lange Personen ohne zweiten Faktor nach dem Einschalten der Richtlinie weiterarbeiten dürfen, mit einem Banner, das die Frist nennt. Als Dauer geschrieben: 7d, 48h. Leer oder 0 wendet sie sofort an.",
	})
}
