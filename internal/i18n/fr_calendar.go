// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The calendar feed section of the settings screen and the screen showing a
// feed address once.
func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Calendar feed":   "Flux de calendrier",
		"Calendar filter": "Filtre du calendrier",
		"A private address Google Calendar, Apple Calendar or Outlook subscribes to, so your due dates appear beside the rest of your week. Each task with a due date is an all-day event that links back here. Anybody holding the address can read what it lists, so treat it like a password: rotate it to issue a new one, revoke it to stop it.": "Une adresse privée à laquelle Google Calendar, Apple Calendar ou Outlook s’abonne, pour que vos échéances apparaissent à côté du reste de votre semaine. Chaque tâche avec une échéance est un événement sur la journée entière qui renvoie ici. Quiconque détient l’adresse peut lire ce qu’elle liste : traitez-la comme un mot de passe. Renouvelez-la pour en émettre une nouvelle, révoquez-la pour l’arrêter.",
		"Feed address": "Adresse du flux",
		"A calendar feed reads tasks as you, and this session cannot read tasks, so there is nothing to manage here.": "Un flux de calendrier lit les tâches en votre nom, et cette session ne peut pas lire les tâches : il n’y a donc rien à gérer ici.",
		"Feed filter": "Filtre du flux",
		"A filter expression, the same language as the task list's filter box. Leave it empty for the tasks assigned to you that are not finished. Only tasks with a due date are listed either way. Changing it keeps the address.": "Une expression de filtre, dans le même langage que la zone de filtre de la liste des tâches. Laissez vide pour les tâches qui vous sont assignées et ne sont pas terminées. Seules les tâches avec une échéance sont listées dans tous les cas. La modifier conserve l’adresse.",
		"Which tasks the feed lists. Empty means yours, not finished.": "Les tâches que liste le flux. Vide signifie les vôtres, non terminées.",
		"Save filter":        "Enregistrer le filtre",
		"Rotate the address": "Renouveler l’adresse",
		"Issues a new address with the same filter and stops the old one at once. Use it if the address was shared by mistake, or to subscribe somewhere new.": "Émet une nouvelle adresse avec le même filtre et arrête aussitôt l’ancienne. Utilisez-le si l’adresse a été partagée par erreur, ou pour vous abonner ailleurs.",
		"A new address; the old one stops.": "Une nouvelle adresse ; l’ancienne s’arrête.",
		"Rotate":                            "Renouveler",
		"Revoke the feed":                   "Révoquer le flux",
		"Stops the address at once. Subscribed calendars keep the events they last fetched until you remove the subscription there.": "Arrête aussitôt l’adresse. Les calendriers abonnés gardent les événements récupérés en dernier jusqu’à ce que vous y supprimiez l’abonnement.",
		"The address stops answering.":                                          "L’adresse ne répond plus.",
		"You have no calendar feed. Create one to subscribe to your due dates.": "Vous n’avez pas de flux de calendrier. Créez-en un pour vous abonner à vos échéances.",
		"Create feed": "Créer le flux",
		"No feed":     "Aucun flux",
		"Optional. Empty means yours, not finished.": "Facultatif. Vide signifie les vôtres, non terminées.",
		"A filter expression, the same language as the task list's filter box. Leave it empty for the tasks assigned to you that are not finished. Only tasks with a due date are listed either way. It can be changed later without changing the address.": "Une expression de filtre, dans le même langage que la zone de filtre de la liste des tâches. Laissez vide pour les tâches qui vous sont assignées et ne sont pas terminées. Seules les tâches avec une échéance sont listées dans tous les cas. Elle peut être modifiée plus tard sans changer l’adresse.",
		"Subscribe to this address in your calendar app. It is not shown again, so copy it now.":                                            "Abonnez-vous à cette adresse dans votre application de calendrier. Elle ne sera plus affichée : copiez-la maintenant.",
		"This address replaces the old one, which has stopped. Subscribe to it again wherever the old one was used. It is not shown again.": "Cette adresse remplace l’ancienne, qui est arrêtée. Abonnez-vous à nouveau partout où l’ancienne était utilisée. Elle ne sera plus affichée.",
		"Lists:": "Liste :",
		"the tasks assigned to you that are not finished":                                                                 "les tâches qui vous sont assignées et ne sont pas terminées",
		"<strong>Google Calendar</strong>: Other calendars, then From URL, and paste the address.":                        "<strong>Google Calendar</strong> : Autres agendas, puis À partir de l’URL, et collez l’adresse.",
		"<strong>Apple Calendar</strong>: File, then New Calendar Subscription, or open the subscribe link below.":        "<strong>Apple Calendar</strong> : Fichier, puis Nouvel abonnement à un calendrier, ou ouvrez le lien d’abonnement ci-dessous.",
		"<strong>Outlook</strong>: Add calendar, then Subscribe from web, and paste the address.":                         "<strong>Outlook</strong> : Ajouter un calendrier, puis S’abonner à partir du web, et collez l’adresse.",
		"Subscribe in this device's calendar":                                                                             "S’abonner dans le calendrier de cet appareil",
		"I have kept it, back to settings":                                                                                "Je l’ai conservée, retour aux paramètres",
		"calendar feed revoked":                                                                                           "flux de calendrier révoqué",
		"A private address your calendar subscribes to, so your due dates appear beside the rest of your week.":           "Une adresse privée à laquelle votre calendrier s’abonne, pour que vos échéances apparaissent à côté du reste de votre semaine.",
		"Created %s. The address is shown only once, when the feed is created or rotated; Rotate below issues a new one.": "Créé %s. L’adresse n’est affichée qu’une fois, à la création ou au renouvellement du flux ; Renouveler ci-dessous en émet une nouvelle.",
		"Google Calendar, Apple Calendar or Outlook subscribes to the address, and each task with a due date is an all-day event that links back here. Anybody holding the address can read what it lists, so treat it like a password. Only a hash of it is kept, which is why it was shown once, when it was created, and cannot be shown again. Rotate issues a new one; calendars subscribed to the old address stop updating. More in docs/calendar.md.": "Google Calendar, Apple Calendar ou Outlook s’abonne à l’adresse, et chaque tâche avec une échéance est un événement sur la journée entière qui renvoie ici. Quiconque détient l’adresse peut lire ce qu’elle liste : traitez-la comme un mot de passe. Seule une empreinte en est conservée, c’est pourquoi elle n’a été affichée qu’une fois, à sa création, et ne peut plus l’être. Renouveler en émet une nouvelle ; les calendriers abonnés à l’ancienne adresse cessent de se mettre à jour. Plus d’informations dans docs/calendar.md.",
		"Google Calendar, Apple Calendar or Outlook subscribes to the address, and each task with a due date is an all-day event that links back here. Creating a feed shows its address once, on the screen that follows, with a Copy button. Anybody holding it can read what it lists, so treat it like a password. Only a hash of it is kept, so it cannot be shown again; rotating issues a new one. More in docs/calendar.md.":                          "Google Calendar, Apple Calendar ou Outlook s’abonne à l’adresse, et chaque tâche avec une échéance est un événement sur la journée entière qui renvoie ici. Créer un flux affiche son adresse une fois, sur l’écran suivant, avec un bouton Copier. Quiconque la détient peut lire ce qu’elle liste : traitez-la comme un mot de passe. Seule une empreinte en est conservée : elle ne peut donc plus être affichée, et le renouvellement en émet une nouvelle. Plus d’informations dans docs/calendar.md.",
		"For example, project:infra tag:ops": "Par exemple, project:infra tag:ops",
	})
}
