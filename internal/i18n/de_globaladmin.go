// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// The global admin area: every tenant, the global admins, and the
		// users of any tenant.
		"Global admin":  "Globaler Admin",
		"Global admins": "Globale Admins",
		"Every tenant of this installation, and who may administer all of them.": "Jeder Mandant dieser Installation und wer sie alle verwalten darf.",
		"Tenants":           "Mandanten",
		"No tenants yet":    "Noch keine Mandanten",
		"Create a tenant":   "Mandanten anlegen",
		"Create tenant":     "Mandanten anlegen",
		"First admins":      "Erste Admins",
		"Rename %s":         "%s umbenennen",
		"Users of %s":       "Benutzer von %s",
		"Users of a tenant": "Benutzer eines Mandanten",
		"One email per line. Each becomes an admin of the new tenant. A new address gets a password generated and shown to you once; an address that already has an account keeps its own password and second factor.": "Eine E-Mail-Adresse pro Zeile. Jede wird Admin des neuen Mandanten. Für eine neue Adresse wird ein Passwort erzeugt und Ihnen einmal angezeigt; eine Adresse, die bereits ein Konto hat, behält ihr eigenes Passwort und ihren zweiten Faktor.",
		"Copy these passwords now": "Kopieren Sie diese Passwörter jetzt",
		"They are shown once and cannot be retrieved again. If one is lost, set a new password from the tenant's users.":                                          "Sie werden einmal angezeigt und können nicht erneut abgerufen werden. Geht eines verloren, setzen Sie über die Benutzer des Mandanten ein neues Passwort.",
		"Its people can no longer sign in, its data stops being reachable and its domains stop resolving. Nothing is destroyed, but nothing here brings it back.": "Seine Personen können sich nicht mehr anmelden, seine Daten sind nicht mehr erreichbar und seine Domains werden nicht mehr aufgelöst. Nichts wird zerstört, aber nichts hier bringt ihn zurück.",
		"Grant global admin":           "Globalen Admin erteilen",
		"Revoke global admin from %s":  "Globalen Admin von %s widerrufen",
		"Revoke global admin from %s?": "Globalen Admin von %s widerrufen?",
		"A global admin creates tenants and administers every tenant here. Name a user who already exists, in any tenant.":                     "Ein globaler Admin legt Mandanten an und verwaltet jeden Mandanten hier. Nennen Sie einen bereits vorhandenen Benutzer aus einem beliebigen Mandanten.",
		"They keep their own tenant and their role in it, and stop administering the others at once. The last global admin cannot be revoked.": "Die Person behält ihren eigenen Mandanten und ihre Rolle darin und verwaltet die anderen ab sofort nicht mehr. Der letzte globale Admin kann nicht widerrufen werden.",
		"tenant created":       "Mandant angelegt",
		"tenant renamed":       "Mandant umbenannt",
		"tenant deleted":       "Mandant gelöscht",
		"global admin granted": "globaler Admin erteilt",
		"global admin revoked": "globaler Admin widerrufen",
		// Shared domains, set by a global admin on a tenant's screen.
		"Shared domains":   "Geteilte Domains",
		"Shared":           "Geteilt",
		"this tenant only": "nur dieser Mandant",
		"Share this host":  "Diesen Host teilen",
		"Stop sharing":     "Nicht mehr teilen",
		"A shared domain's host also serves the tenant a session, a token or the X-Tix-Tenant header names, so several tenants work on one address. With neither it serves this tenant. A domain that is not shared serves this tenant alone and refuses every other tenant's credential. Only a global admin changes it.": "Der Host einer geteilten Domain bedient auch den Mandanten, den eine Sitzung, ein Token oder der X-Tix-Tenant-Header nennt, sodass mehrere Mandanten unter einer Adresse arbeiten. Ohne diese bedient er diesen Mandanten. Eine nicht geteilte Domain bedient nur diesen Mandanten und lehnt die Zugänge jedes anderen Mandanten ab. Nur ein globaler Admin kann das ändern.",
		"domain shared":           "Domain geteilt",
		"domain no longer shared": "Domain nicht mehr geteilt",
	})
}
