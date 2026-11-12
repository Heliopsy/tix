// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// Settings, Profile.
		"Profile": "Profil",
		"Who you are signed in as here, and with what authority.": "Als wer Sie hier angemeldet sind und mit welchen Rechten.",
		"Open your actor page": "Ihre Akteurseite öffnen",
		"The short name other people type to mention you or assign you work, unique within this tenant.": "Der Kurzname, den andere eingeben, um Sie zu erwähnen oder Ihnen Arbeit zuzuweisen, eindeutig in diesem Mandanten.",
		"Actor id":           "Akteur-ID",
		"Copy your actor id": "Ihre Akteur-ID kopieren",
		"The identifier the command line, the API and the audit log use for you. It never changes, while a handle can.": "Die Kennung, die Kommandozeile, API und Audit-Log für Sie verwenden. Sie ändert sich nie, ein Handle dagegen schon.",
		"Your role in this tenant. Its scopes add to any granted to you directly.":                                      "Ihre Rolle in diesem Mandanten. Ihre Berechtigungen kommen zu denen hinzu, die Ihnen direkt erteilt wurden.",
		"Whether you may administer the whole installation, every tenant included, right now.":                          "Ob Sie gerade die gesamte Installation verwalten dürfen, alle Mandanten eingeschlossen.",
		"Signed in with": "Angemeldet mit",
		"What this request authenticated with: the session this browser holds, or an API token.": "Womit sich diese Anfrage authentifiziert hat: die Sitzung dieses Browsers oder ein API-Token.",
		"Browser session": "Browsersitzung",
		"Everything you may do here, from your role and from any scope granted to you directly.":                           "Alles, was Sie hier tun dürfen, aus Ihrer Rolle und aus direkt erteilten Berechtigungen.",
		"The address you sign in with. It belongs to your account, which is the same in every tenant you are a member of.": "Die Adresse, mit der Sie sich anmelden. Sie gehört zu Ihrem Konto, das in jedem Mandanten, dem Sie angehören, dasselbe ist.",
		"Every tenant your account is a member of, with your handle and role there.":                                       "Jeder Mandant, dem Ihr Konto angehört, mit Ihrem Handle und Ihrer Rolle dort.",
		"this tenant": "dieser Mandant",
		"Not set":     "Nicht festgelegt",
		"None":        "Keine",
		"Yes":         "Ja",
		"No":          "Nein",

		// The actor screen.
		"A person or an agent of this tenant, by the name others use for it. What it may do is not shown here.": "Eine Person oder ein Agent dieses Mandanten, unter dem Namen, den andere verwenden. Was sie oder er darf, wird hier nicht gezeigt.",
		"About":                "Über",
		"Copy the actor id":    "Akteur-ID kopieren",
		"Tasks assigned to %s": "Aufgaben, die %s zugewiesen sind",
		"Your profile":         "Ihr Profil",
		"Recent activity":      "Letzte Aktivität",
		"All activity":         "Alle Aktivitäten",
		"Recent activity needs the audit:read scope, which you do not hold.": "Die letzte Aktivität braucht die Berechtigung audit:read, die Sie nicht haben.",
		"Open %s's page": "Seite von %s öffnen",

		// The credential filter on the token and SSH key screens.
		"Whose credentials":                    "Wessen Zugangsdaten",
		"Handle or id, empty for yours":        "Handle oder ID, leer für Ihre",
		"Show":                                 "Anzeigen",
		"Show yours":                           "Ihre anzeigen",
		"No actor of this tenant is named %s.": "Kein Akteur dieses Mandanten heißt %s.",
		"Listing what %s holds.":               "Es wird aufgelistet, was %s besitzt.",
		"Keys enrolled against the actor named below. Enrolling a key here always enrols it against your own actor.": "Schlüssel, die für den unten genannten Akteur registriert sind. Ein hier registrierter Schlüssel wird immer für Ihren eigenen Akteur registriert.",

		// The subtask tree.
		"Direct subtasks":  "Direkte Unteraufgaben",
		"Subtasks of %s":   "Unteraufgaben von %s",
		"Show %d subtask":  "%d Unteraufgabe anzeigen",
		"Show %d subtasks": "%d Unteraufgaben anzeigen",
		"This tree is too large to draw whole, so only direct subtasks are shown.":                                                                                         "Dieser Baum ist zu groß, um ihn ganz darzustellen, daher werden nur direkte Unteraufgaben gezeigt.",
		"The whole tree under this task. Three levels show at once, and a deeper level loads when you ask for it. The arrow beside a subtask folds its own subtasks away.": "Der gesamte Baum unter dieser Aufgabe. Drei Ebenen werden auf einmal gezeigt, eine tiefere wird auf Anfrage geladen. Der Pfeil neben einer Unteraufgabe klappt deren eigene Unteraufgaben ein.",
	})
}
