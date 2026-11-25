// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// Creating a user: a password or an invite link.
		"How they sign in":       "Wie sich die Person anmeldet",
		"Set a password":         "Passwort festlegen",
		"Send an invite link":    "Einladungslink senden",
		"Create invite link":     "Einladungslink erstellen",
		"Link expires after":     "Link läuft ab nach",
		"1 day":                  "1 Tag",
		"3 days":                 "3 Tage",
		"Invite links":           "Einladungslinks",
		"Invite link":            "Einladungslink",
		"Send this link to them": "Senden Sie der Person diesen Link",
		"Require a password change at first sign-in":                                                 "Passwortänderung bei der ersten Anmeldung verlangen",
		"Ask to change the password at next sign-in":                                                 "Bei der nächsten Anmeldung eine Passwortänderung verlangen",
		"You get a link to send them. It works once, and they choose their own password with it.":    "Sie erhalten einen Link, den Sie weitergeben. Er funktioniert einmal, und die Person wählt damit ihr eigenes Passwort.",
		"It works once and expires on %s. tix does not send email, and the link is not shown again.": "Er funktioniert einmal und läuft am %s ab. tix verschickt keine E-Mails, und der Link wird nicht erneut angezeigt.",
		"An invite link lets the person choose their own password. It works once and expires; tix sends no email, so you pass the link on yourself. A password you type yourself can be made to change at their first sign-in.": "Mit einem Einladungslink wählt die Person ihr Passwort selbst. Er funktioniert einmal und läuft ab; tix verschickt keine E-Mails, Sie geben den Link selbst weiter. Ein Passwort, das Sie selbst eingeben, kann bei der ersten Anmeldung zur Änderung verlangt werden.",
		"Their next sign-in with a password stops at a page asking for a new one, and goes no further until they choose it. Sessions already open and API tokens are not affected.":                                             "Die nächste Anmeldung mit Passwort hält auf einer Seite an, die ein neues verlangt, und geht erst weiter, wenn eines gewählt ist. Bereits offene Sitzungen und API-Tokens sind nicht betroffen.",

		// A change the account's other tenants keep from a tenant admin.
		"%s also belongs to another tenant, so only they or a global admin can ask them to change their password. Leave the box as it was to save the rest.": "%s gehört auch zu einem anderen Mandanten, daher kann nur die Person selbst oder ein globaler Admin eine Passwortänderung verlangen. Lassen Sie das Kästchen unverändert, um den Rest zu speichern.",
		"%s also belongs to another tenant, so only they or a global admin can clear their second factor.":                                                   "%s gehört auch zu einem anderen Mandanten, daher kann nur die Person selbst oder ein globaler Admin den zweiten Faktor entfernen.",

		// Pending invites.
		"Pending invites": "Offene Einladungen",
		"Links issued and not used yet. Re-issuing stops the old link and gives you a new one to send; revoking stops it and keeps the account, without a password.": "Ausgestellte, noch nicht verwendete Links. Neu ausstellen stoppt den alten Link und gibt Ihnen einen neuen zum Weitergeben; Widerrufen stoppt ihn und behält das Konto, ohne Passwort.",
		"invited":                    "eingeladen",
		"invited by %s":              "eingeladen von %s",
		"expires %s":                 "läuft ab %s",
		"Re-issue":                   "Neu ausstellen",
		"Re-issue the invite for %s": "Einladung für %s neu ausstellen",
		"Revoke the invite for %s":   "Einladung für %s widerrufen",
		"Revoke the invite for %s?":  "Einladung für %s widerrufen?",
		"The link stops working at once. The account stays, without a password, until you delete it or invite it again.": "Der Link funktioniert sofort nicht mehr. Das Konto bleibt ohne Passwort bestehen, bis Sie es löschen oder erneut einladen.",
		"invite created":   "Einladung erstellt",
		"invite re-issued": "Einladung neu ausgestellt",
		"invite revoked":   "Einladung widerrufen",

		// The invite page.
		"Accept your invite": "Einladung annehmen",
		"Join %s":            "%s beitreten",
		"Choose a password to finish setting up your account. This link works once.":    "Wählen Sie ein Passwort, um die Einrichtung Ihres Kontos abzuschließen. Dieser Link funktioniert einmal.",
		"You already have an account. Enter its password to accept; it stays as it is.": "Sie haben bereits ein Konto. Geben Sie dessen Passwort ein, um anzunehmen; es bleibt unverändert.",
		"Signing in as":                "Anmeldung als",
		"Current password":             "Aktuelles Passwort",
		"Password again":               "Passwort wiederholen",
		"At least 12 characters.":      "Mindestens 12 Zeichen.",
		"Accept and sign in":           "Annehmen und anmelden",
		"Code from your authenticator": "Code aus Ihrem Authenticator",
		"Your account has a second factor, so joining another tenant asks for a code from it too.": "Ihr Konto hat einen zweiten Faktor, daher verlangt der Beitritt zu einem weiteren Mandanten auch einen Code daraus.",
		"This link does not work": "Dieser Link funktioniert nicht",
		"This invite link is not valid. It may have expired, been used already or been revoked. Ask whoever invited you for a new one.": "Dieser Einladungslink ist nicht gültig. Er ist vielleicht abgelaufen, schon verwendet oder widerrufen. Bitten Sie die einladende Person um einen neuen.",
		"Go to sign in": "Zur Anmeldung",

		// The password change a sign-in can be stopped at.
		"Choose a new password": "Neues Passwort wählen",
		"Your administrator asked you to replace your password before you go on.": "Ihr Administrator bittet Sie, Ihr Passwort zu ersetzen, bevor Sie fortfahren.",
		"New password":       "Neues Passwort",
		"New password again": "Neues Passwort wiederholen",
		"At least 12 characters, and not the one you have now.": "Mindestens 12 Zeichen und nicht das aktuelle.",
		"Save and sign in": "Speichern und anmelden",
		"Back to sign in":  "Zurück zur Anmeldung",
	})
}
