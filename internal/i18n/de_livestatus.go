// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The live connection indicator beside the tenant's name and its
// connection details dialog.
func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Live updates":     "Live-Aktualisierungen",
		"Live updates: %s": "Live-Aktualisierungen: %s",
		"Connecting":       "Verbinden",
		"Reconnecting":     "Neu verbinden",
		"Polling":          "Abfrage",
		"Offline":          "Offline",
		"Changes made elsewhere show up here as they happen.":                                                                           "Änderungen von anderswo erscheinen hier, sobald sie passieren.",
		"Connecting to live updates. Changes show up once it is through.":                                                               "Verbindung zu Live-Aktualisierungen wird aufgebaut. Änderungen erscheinen, sobald sie steht.",
		"The live connection dropped and is being tried again. Changes made meanwhile show up once it is back.":                         "Die Live-Verbindung ist abgebrochen und wird erneut versucht. Zwischenzeitliche Änderungen erscheinen, sobald sie wieder steht.",
		"The live connection keeps failing, so this page asks the server every 15 seconds instead. Changes can take that long to show.": "Die Live-Verbindung schlägt immer wieder fehl, daher fragt diese Seite stattdessen alle 15 Sekunden beim Server nach. Änderungen können so lange brauchen, bis sie erscheinen.",
		"The server cannot be reached. This page keeps trying and catches up when it is back.":                                          "Der Server ist nicht erreichbar. Diese Seite versucht es weiter und holt auf, sobald er wieder da ist.",
		"Connection details":                 "Verbindungsdetails",
		"Carried over":                       "Übertragung",
		"WebSocket":                          "WebSocket",
		"Polling over HTTP every 15 seconds": "Abfrage über HTTP alle 15 Sekunden",
		"Since":                              "Seit",
		"Last reached the server":            "Server zuletzt erreicht",
		"Last event":                         "Letztes Ereignis",
		"Nothing yet.":                       "Noch nichts.",
		"Failures in a row":                  "Fehlschläge in Folge",
		"Next try":                           "Nächster Versuch",
		"Last error":                         "Letzter Fehler",
		"Nothing has failed since this page was loaded.": "Seit dem Laden dieser Seite ist nichts fehlgeschlagen.",
		"During":                     "Bei",
		"Fetching the page":          "Seite wird abgerufen",
		"HTTP status":                "HTTP-Status",
		"Close code":                 "Schließcode",
		"Message":                    "Meldung",
		"Could not reach the server": "Server nicht erreichbar",
		"Connection closed":          "Verbindung geschlossen",
		"Refused by the server":      "Vom Server abgelehnt",
		"HTTP error":                 "HTTP-Fehler",
		"What it may mean":           "Was es bedeuten kann",
		"A proxy or load balancer may be closing idle connections. Its read timeout should be above 60 s for /api/v1/events.": "Ein Proxy oder Load Balancer schließt möglicherweise inaktive Verbindungen. Sein Lese-Timeout sollte für /api/v1/events über 60 s liegen.",
		"The WebSocket fails while plain HTTP works, so the proxy in front of tix may not forward WebSocket upgrades.":        "Der WebSocket schlägt fehl, während einfaches HTTP funktioniert, daher leitet der Proxy vor tix WebSocket-Upgrades möglicherweise nicht weiter.",
		"The browser could not reach the host. Check the network and that the server is running.":                             "Der Browser konnte den Host nicht erreichen. Prüfen Sie das Netzwerk und ob der Server läuft.",
		"The server refused live updates to this session. Signing in again usually fixes it.":                                 "Der Server hat dieser Sitzung Live-Aktualisierungen verweigert. Eine erneute Anmeldung behebt das meist.",
		"The server or a proxy answered HTTP %s. Its logs should say why.":                                                    "Der Server oder ein Proxy antwortete mit HTTP %s. Seine Protokolle sollten den Grund nennen.",
		"Recent changes": "Letzte Änderungen",
		"Reconnect now":  "Jetzt neu verbinden",
		"Copy details":   "Details kopieren",
		"Connection details copied. Cookies and tokens are left out.": "Verbindungsdetails kopiert. Cookies und Tokens sind nicht enthalten.",
	})
}
