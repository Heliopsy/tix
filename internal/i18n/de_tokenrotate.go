// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Rotate":           "Erneuern",
		"Rotate %s":        "%s erneuern",
		"Rotate %s?":       "%s erneuern?",
		"Rotating a token": "Ein Token erneuern",
		"Old token":        "Altes Token",
		"retiring":         "läuft aus",
		"token rotated":    "Token erneuert",
		"A new token with the same name, scopes and lifetime replaces this one, and its value is shown once.":                                                                                                            "Ein neues Token mit demselben Namen, denselben Berechtigungen und derselben Laufzeit ersetzt dieses, und sein Wert wird einmal angezeigt.",
		"This token is %s's, not yours, and the trail will record that you rotated somebody else's.":                                                                                                                     "Dieses Token gehört %s, nicht Ihnen, und das Protokoll wird festhalten, dass Sie das Token einer anderen Person erneuert haben.",
		"Rotate a token on a schedule, or at once when it may have leaked. Whatever holds it has to be given the new value; a grace window keeps the old one working meanwhile, and revoking it ends that window early.": "Erneuern Sie ein Token nach Plan oder sofort, wenn es möglicherweise offengelegt wurde. Was es verwendet, muss den neuen Wert erhalten; ein Übergangsfenster hält das alte so lange gültig, und ein Widerruf beendet dieses Fenster vorzeitig.",
		"How long the old value keeps authenticating beside the new one, up to a day, so a running agent can pick up the new value without failing a request. After a leak, stop it now.":                                "Wie lange der alte Wert neben dem neuen noch authentifiziert, bis zu einem Tag, damit ein laufender Agent den neuen Wert übernehmen kann, ohne dass eine Anfrage fehlschlägt. Nach einem Leck sofort beenden.",
		"Stop the old token now":         "Altes Token sofort beenden",
		"Keep it working for 10 minutes": "10 Minuten gültig lassen",
		"Keep it working for 1 hour":     "1 Stunde gültig lassen",
		"Keep it working for 24 hours":   "24 Stunden gültig lassen",
	})
}
