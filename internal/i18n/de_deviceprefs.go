// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Keep tix still even when the system allows animation.": "tix bleibt ruhig, auch wenn das System Animationen erlaubt.",
		"Reduce motion": "Bewegung reduzieren",
		"Each setting says where it is kept: on this device, in this browser's cookies or storage; synced to your account, so it follows you everywhere you sign in; or for the whole tenant.":                                                    "Jede Einstellung gibt an, wo sie gespeichert wird: auf diesem Gerät, in den Cookies oder im Speicher dieses Browsers; mit Ihrem Konto synchronisiert, sodass sie Ihnen überallhin folgt, wo Sie sich anmelden; oder für den ganzen Mandanten.",
		"The screen this browser opens when you go to the address of tix itself, and after you sign in without following a link.":                                                                                                                 "Der Bildschirm, den dieser Browser öffnet, wenn Sie die Adresse von tix selbst aufrufen, und nach der Anmeldung, wenn Sie keinem Link gefolgt sind.",
		"A notification from your browser for each new inbox item, while a tix tab is open and you are looking at something else. Turning it on asks the browser for permission. A closed tab cannot be told: channels below deliver everywhere.": "Eine Benachrichtigung Ihres Browsers für jeden neuen Eintrag im Posteingang, solange ein tix-Tab geöffnet ist und Sie sich etwas anderes ansehen. Beim Einschalten fragt der Browser um Erlaubnis. Ein geschlossener Tab erfährt nichts davon: Die Kanäle unten liefern überallhin.",
		"Browser notifications while a tix tab is open.": "Browser-Benachrichtigungen, solange ein tix-Tab geöffnet ist.",
		"Notifications say only that something new reached your inbox, without who did what or the task's title, for a screen other people can see.": "Benachrichtigungen sagen nur, dass etwas Neues in Ihrem Posteingang angekommen ist, ohne wer was getan hat und ohne den Titel der Aufgabe, für einen Bildschirm, den auch andere sehen.",
		"Notifications are blocked for this site. Allow them in the browser's site settings, then turn this on.":                                     "Benachrichtigungen sind für diese Website blockiert. Erlauben Sie sie in den Website-Einstellungen des Browsers und schalten Sie dies dann ein.",
		"Notify me about new inbox items": "Über neue Einträge im Posteingang benachrichtigen",
		"Turns off animation, sliding and smooth scrolling in this browser. Your system's own reduced motion setting already does this; this keeps tix still even when the system allows animation.": "Schaltet Animationen, Gleiten und weiches Scrollen in diesem Browser aus. Die Systemeinstellung für reduzierte Bewegung tut das bereits; diese hier hält tix ruhig, auch wenn das System Animationen erlaubt.",
		"Say only that something arrived.":                "Nur mitteilen, dass etwas angekommen ist.",
		"Where tix opens in this browser.":                "Wo tix in diesem Browser öffnet.",
		"Settings here apply to everyone in this tenant.": "Die Einstellungen hier gelten für alle in diesem Mandanten.",
		"On this device":                                  "Auf diesem Gerät",
		"Start screen":                                    "Startbildschirm",
		"Something new reached your inbox.":               "Etwas Neues ist in Ihrem Posteingang angekommen.",
		"No setting matches “%s”":                         "Keine Einstellung passt zu „%s“",
		"Try another word, or clear the search.":          "Versuchen Sie ein anderes Wort oder leeren Sie die Suche.",
		"Apply start screen":                              "Startbildschirm übernehmen",
		"Synced to your account":                          "Mit Ihrem Konto synchronisiert",
		"Hide what they say":                              "Inhalt ausblenden",
		"This browser cannot show notifications.":         "Dieser Browser kann keine Benachrichtigungen anzeigen.",
		"Search settings":                                 "Einstellungen durchsuchen",
		"Whole tenant":                                    "Ganzer Mandant",
	})
}
