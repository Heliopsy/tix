// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The Live events screen: the tenant's raw event stream, followed live.
func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Live events": "Live-Ereignisse",
		"Live":        "Live",
		"The tenant's events as they happen, newest first.": "Die Ereignisse des Mandanten, sobald sie passieren, neueste zuerst.",
		"Every event this tenant emits, as clients receive it: the stream tix watch prints. Nothing here is stored; a reload starts from now unless you replay from a sequence number. Activity is the audit log, kept and searchable.": "Jedes Ereignis, das dieser Mandant aussendet, so wie Clients es erhalten: der Strom, den tix watch ausgibt. Hier wird nichts gespeichert; ein Neuladen beginnt bei jetzt, sofern Sie nicht ab einer Sequenznummer wiedergeben. Aktivität ist das Audit-Log, gespeichert und durchsuchbar.",
		"Changes in this tenant show up here as they happen. Earlier ones are not shown unless you replay from a sequence number.":                                                                                                      "Änderungen in diesem Mandanten erscheinen hier, sobald sie passieren. Frühere werden nicht angezeigt, sofern Sie nicht ab einer Sequenznummer wiedergeben.",
		"Event types": "Ereignistypen",
		"Comma separated, sent to the server. A trailing * matches a prefix, as in task.*.": "Durch Kommas getrennt, an den Server gesendet. Ein * am Ende passt auf ein Präfix, wie in task.*.",
		"Actors": "Akteure",
		"Handles or ids, comma separated. Matched in this browser, as tix watch does.":                                "Handles oder IDs, durch Kommas getrennt. Abgeglichen in diesem Browser, so wie tix watch es tut.",
		"This picker does not list every project in this tenant, so a project missing from it cannot be chosen here.": "Diese Auswahl listet nicht jedes Projekt dieses Mandanten auf, daher kann ein fehlendes Projekt hier nicht gewählt werden.",
		"Replay from sequence number": "Ab Sequenznummer wiedergeben",
		"Shows the events after this number again, then follows live. Empty starts from now.": "Zeigt die Ereignisse nach dieser Nummer erneut und folgt dann live. Leer beginnt bei jetzt.",
		"Follow every event":                                       "Jedem Ereignis folgen",
		"No project has the key %s.":                               "Kein Projekt hat den Schlüssel %s.",
		"The sequence number %s is not a whole number above zero.": "Die Sequenznummer %s ist keine ganze Zahl über null.",
		"project %s":                     "Projekt %s",
		"type %s":                        "Typ %s",
		"actor %s":                       "Akteur %s",
		"after #%s":                      "nach #%s",
		"Pause":                          "Pausieren",
		"Resume":                         "Fortsetzen",
		"Clear the list":                 "Liste leeren",
		"Event":                          "Ereignis",
		"Event #%d":                      "Ereignis #%d",
		"Details":                        "Details",
		"Waiting for events":             "Warten auf Ereignisse",
		"Connecting to the event stream": "Verbindung zum Ereignisstrom wird aufgebaut",
		"Not following events.":          "Ereignisse werden nicht verfolgt.",
		"Offline. Reconnecting.":         "Offline. Verbindung wird wiederhergestellt.",
		"Paused.":                        "Pausiert.",
		"Paused, 1 new event.":           "Pausiert, 1 neues Ereignis.",
		"Paused, %d new events.":         "Pausiert, %d neue Ereignisse.",
		"Keeping the latest %d events; older ones are dropped.":                                "Die letzten %d Ereignisse werden behalten; ältere fallen weg.",
		"Events after #%d are no longer kept on the server. Showing new events from now.":      "Ereignisse nach #%d werden auf dem Server nicht mehr aufbewahrt. Neue Ereignisse werden ab jetzt angezeigt.",
		"The server refused this subscription: %s":                                             "Der Server hat dieses Abonnement abgelehnt: %s",
		"This server does not stream events.":                                                  "Dieser Server sendet keinen Ereignisstrom.",
		"Following events needs JavaScript. The filters above still make a link you can send.": "Das Verfolgen von Ereignissen braucht JavaScript. Die Filter oben ergeben trotzdem einen Link, den Sie versenden können.",
	})
}
