// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Something wrong?": "Stimmt etwas nicht?",
		"Copy these facts into a bug report on the tix project's issue tracker. They name versions and this browser, never a token, a password or your work.": "Kopieren Sie diese Angaben in einen Fehlerbericht im Issue-Tracker des tix-Projekts. Sie nennen Versionen und diesen Browser, niemals ein Token, ein Passwort oder Ihre Arbeit.",
		"Include the server address":                                 "Serveradresse einschließen",
		"Off keeps a private server's address out of what you copy.": "Ausgeschaltet bleibt die Adresse eines privaten Servers aus dem Kopierten heraus.",
		"Copy diagnostics":                                           "Diagnose kopieren",
	})
}
