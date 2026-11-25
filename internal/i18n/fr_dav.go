// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The Tasks in other apps section of the settings screen and the screen
// showing an app password once.
func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Tasks in other apps":  "Tâches dans d'autres applications",
		"Add an app password":  "Ajouter un mot de passe d'application",
		"CalDAV address":       "Adresse CalDAV",
		"CalDAV":               "CalDAV",
		"Expired":              "Expiré",
		"Revoked":              "Révoqué",
		"read-only":            "lecture seule",
		"Created %s.":          "Créé %s.",
		"Last used %s.":        "Dernière utilisation %s.",
		"Never used.":          "Jamais utilisé.",
		"Revoked %s.":          "Révoqué %s.",
		"For example, iPhone":  "Par exemple, iPhone",
		"Create password":      "Créer le mot de passe",
		"app password revoked": "mot de passe d'application révoqué",
		"App password":         "Mot de passe d'application",
		"Sign in with":         "Se connecter avec",
		"Server address":       "Adresse du serveur",
		"Username":             "Nom d'utilisateur",
		"Assigned to me":       "Qui me sont assignées",
		"One list per project": "Une liste par projet",
		"Both":                 "Les deux",
		"Show your tasks in Apple Reminders, Thunderbird, DAVx5 with tasks.org, or Evolution. Each app signs in with an app password of its own.":                                                                         "Affichez vos tâches dans Rappels d'Apple, Thunderbird, DAVx5 avec tasks.org ou Evolution. Chaque application se connecte avec son propre mot de passe d'application.",
		"An app password belongs to a person who reads tasks, and this session does not speak for one, so there is nothing to manage here.":                                                                               "Un mot de passe d'application appartient à une personne qui lit des tâches, et cette session ne parle pour personne, il n'y a donc rien à gérer ici.",
		"CalDAV is how calendar and reminders apps read tasks from a server. Apple Reminders, Thunderbird, DAVx5 with tasks.org and Evolution all speak it. For now the apps can read your tasks and cannot change them.": "CalDAV permet aux applications d'agenda et de rappels de lire des tâches sur un serveur. Rappels d'Apple, Thunderbird, DAVx5 avec tasks.org et Evolution le prennent en charge. Pour l'instant, les applications peuvent lire vos tâches sans les modifier.",
		"CalDAV is turned off on this server (caldav.enabled)":                                                                                                                        "CalDAV est désactivé sur ce serveur (caldav.enabled)",
		"Give your app this address, your username and an app password. Your account password does not work here.":                                                                    "Donnez à votre application cette adresse, votre nom d'utilisateur et un mot de passe d'application. Le mot de passe de votre compte ne fonctionne pas ici.",
		"No app passwords yet. Add one for each app you sign in.":                                                                                                                     "Aucun mot de passe d'application pour l'instant. Ajoutez-en un pour chaque application que vous connectez.",
		"The app or device this password is for, so you know which one to revoke later. Each of your live app passwords has its own name.":                                            "L'application ou l'appareil auquel ce mot de passe est destiné, pour savoir plus tard lequel révoquer. Chacun de vos mots de passe d'application actifs a son propre nom.",
		"One list of the tasks assigned to you, in every project.":                                                                                                                    "Une liste des tâches qui vous sont assignées, dans tous les projets.",
		"A list for each project you can read, with everybody's tasks.":                                                                                                               "Une liste pour chaque projet que vous pouvez lire, avec les tâches de tout le monde.",
		"Assigned to me and every project, so your own tasks appear twice.":                                                                                                           "Mes tâches assignées et chaque projet, vos propres tâches apparaissent donc deux fois.",
		"Read-only, even once apps can change tasks":                                                                                                                                  "Lecture seule, même quand les applications pourront modifier les tâches",
		"The app signed in with this password is refused from its next request. It cannot be undone; add a new password to sign the app in again.":                                    "L'application connectée avec ce mot de passe est refusée dès sa prochaine requête. C'est irréversible ; ajoutez un nouveau mot de passe pour reconnecter l'application.",
		"An app password signs one calendar or reminders app in to your tasks over CalDAV. It reads tasks as you and nothing else, and revoking it in Settings stops that app alone.": "Un mot de passe d'application connecte une application d'agenda ou de rappels à vos tâches via CalDAV. Il lit les tâches en votre nom et rien d'autre, et le révoquer dans les Paramètres n'arrête que cette application.",
		"Sign %s in with these three values. The password is not shown again, so copy it now.":                                                                                        "Connectez %s avec ces trois valeurs. Le mot de passe ne sera plus affiché, copiez-le maintenant.",
		"<strong>Apple Reminders</strong>: Settings, then Calendar, Accounts, Add Account, Other, and Add CalDAV Account.":                                                            "<strong>Rappels d'Apple</strong> : Réglages, puis Calendrier, Comptes, Ajouter un compte, Autre et Ajouter un compte CalDAV.",
		"<strong>Thunderbird</strong>: New Calendar, On the Network, then the username and the server address.":                                                                       "<strong>Thunderbird</strong> : Nouvel agenda, Sur le réseau, puis le nom d'utilisateur et l'adresse du serveur.",
		"<strong>DAVx5</strong>: add an account with a URL and username, then pick tasks.org to show the lists.":                                                                      "<strong>DAVx5</strong> : ajoutez un compte avec une URL et un nom d'utilisateur, puis choisissez tasks.org pour afficher les listes.",
		"<strong>Evolution</strong>: New, then Task List, type CalDAV, and the server address.":                                                                                       "<strong>Evolution</strong> : Nouveau, puis Liste de tâches, type CalDAV et l'adresse du serveur.",
	})
}
