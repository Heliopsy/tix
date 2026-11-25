// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The Tasks in other apps section of the settings screen and the screen
// showing an app password once.
func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Tasks in other apps":  "Aufgaben in anderen Apps",
		"Add an app password":  "App-Passwort hinzufügen",
		"CalDAV address":       "CalDAV-Adresse",
		"CalDAV":               "CalDAV",
		"Expired":              "Abgelaufen",
		"Revoked":              "Widerrufen",
		"read-only":            "nur lesen",
		"Created %s.":          "Erstellt %s.",
		"Last used %s.":        "Zuletzt verwendet %s.",
		"Never used.":          "Nie verwendet.",
		"Revoked %s.":          "Widerrufen %s.",
		"For example, iPhone":  "Zum Beispiel iPhone",
		"Create password":      "Passwort erstellen",
		"app password revoked": "App-Passwort widerrufen",
		"App password":         "App-Passwort",
		"Sign in with":         "Anmelden mit",
		"Server address":       "Serveradresse",
		"Username":             "Benutzername",
		"Assigned to me":       "Mir zugewiesen",
		"One list per project": "Eine Liste pro Projekt",
		"Both":                 "Beides",
		"Show your tasks in Apple Reminders, Thunderbird, DAVx5 with tasks.org, or Evolution. Each app signs in with an app password of its own.":                                                                                                            "Zeigen Sie Ihre Aufgaben in Apple Erinnerungen, Thunderbird, DAVx5 mit tasks.org oder Evolution. Jede App meldet sich mit einem eigenen App-Passwort an.",
		"An app password belongs to a person who reads tasks, and this session does not speak for one, so there is nothing to manage here.":                                                                                                                  "Ein App-Passwort gehört einer Person, die Aufgaben liest, und diese Sitzung spricht für keine, daher gibt es hier nichts zu verwalten.",
		"CalDAV is how calendar and reminders apps read and change tasks on a server. Apple Reminders, Thunderbird, DAVx5 with tasks.org and Evolution all speak it. An app can tick off, edit, add and cancel your tasks unless its password is read-only.": "Über CalDAV lesen und ändern Kalender- und Erinnerungs-Apps Aufgaben auf einem Server. Apple Erinnerungen, Thunderbird, DAVx5 mit tasks.org und Evolution beherrschen es. Eine App kann Ihre Aufgaben abhaken, bearbeiten, anlegen und abbrechen, sofern ihr Passwort nicht nur lesen darf.",
		"CalDAV is turned off on this server (caldav.enabled)":                                                                                                                        "CalDAV ist auf diesem Server ausgeschaltet (caldav.enabled)",
		"Give your app this address, your username and an app password. Your account password does not work here.":                                                                    "Geben Sie Ihrer App diese Adresse, Ihren Benutzernamen und ein App-Passwort. Ihr Kontopasswort funktioniert hier nicht.",
		"No app passwords yet. Add one for each app you sign in.":                                                                                                                     "Noch keine App-Passwörter. Legen Sie für jede App, die Sie anmelden, eines an.",
		"The app or device this password is for, so you know which one to revoke later. Each of your live app passwords has its own name.":                                            "Die App oder das Gerät, für das dieses Passwort ist, damit Sie später wissen, welches Sie widerrufen. Jedes Ihrer aktiven App-Passwörter hat einen eigenen Namen.",
		"One list of the tasks assigned to you, in every project.":                                                                                                                    "Eine Liste der Ihnen zugewiesenen Aufgaben aus allen Projekten.",
		"A list for each project you can read, with everybody's tasks.":                                                                                                               "Eine Liste für jedes Projekt, das Sie lesen dürfen, mit den Aufgaben aller.",
		"Assigned to me and every project, so your own tasks appear twice.":                                                                                                           "Mir zugewiesen und jedes Projekt, Ihre eigenen Aufgaben erscheinen also doppelt.",
		"Read-only: the app can list tasks but not change them":                                                                                                                       "Nur lesen: die App zeigt Aufgaben, ändert sie aber nicht",
		"The app signed in with this password is refused from its next request. It cannot be undone; add a new password to sign the app in again.":                                    "Die mit diesem Passwort angemeldete App wird ab ihrer nächsten Anfrage abgewiesen. Das lässt sich nicht rückgängig machen; legen Sie ein neues Passwort an, um die App wieder anzumelden.",
		"An app password signs one calendar or reminders app in to your tasks over CalDAV. It reads tasks as you and nothing else, and revoking it in Settings stops that app alone.": "Ein App-Passwort meldet eine Kalender- oder Erinnerungs-App über CalDAV bei Ihren Aufgaben an. Es liest Aufgaben in Ihrem Namen und sonst nichts, und wenn Sie es in den Einstellungen widerrufen, stoppt nur diese App.",
		"Sign %s in with these three values. The password is not shown again, so copy it now.":                                                                                        "Melden Sie %s mit diesen drei Werten an. Das Passwort wird nicht noch einmal angezeigt, kopieren Sie es also jetzt.",
		"<strong>Apple Reminders</strong>: Settings, then Calendar, Accounts, Add Account, Other, and Add CalDAV Account.":                                                            "<strong>Apple Erinnerungen</strong>: Einstellungen, dann Kalender, Accounts, Account hinzufügen, Andere und CalDAV-Account hinzufügen.",
		"<strong>Thunderbird</strong>: New Calendar, On the Network, then the username and the server address.":                                                                       "<strong>Thunderbird</strong>: Neuer Kalender, Im Netzwerk, dann Benutzername und Serveradresse.",
		"<strong>DAVx5</strong>: add an account with a URL and username, then pick tasks.org to show the lists.":                                                                      "<strong>DAVx5</strong>: ein Konto mit URL und Benutzername hinzufügen, dann tasks.org wählen, um die Listen anzuzeigen.",
		"<strong>Evolution</strong>: New, then Task List, type CalDAV, and the server address.":                                                                                       "<strong>Evolution</strong>: Neu, dann Aufgabenliste, Typ CalDAV und die Serveradresse.",
		"via CalDAV":                   "über CalDAV",
		"via CalDAV (app password %s)": "über CalDAV (App-Passwort %s)",
		"CalDAV app":                   "CalDAV-App",
	})
}
