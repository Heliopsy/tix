// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Stay signed in on this browser after it closes. Each visit keeps it going, up to a limit this server sets. Leave it unticked on a shared computer. If this tenant ends sessions left unused, that still applies.": "Bleiben Sie in diesem Browser auch nach dem Schließen angemeldet. Jeder Besuch verlängert die Anmeldung, bis zu einer Grenze, die dieser Server festlegt. Auf einem gemeinsam genutzten Computer nicht ankreuzen. Wenn dieser Mandant unbenutzte Sitzungen beendet, gilt das weiterhin.",
		"For %s after your last visit, on this browser.": "Für %s nach Ihrem letzten Besuch, in diesem Browser.",
		"How long a sign-in on this tenant lasts after it is made and again after each use, such as 8h, up to the server's maximum lifetime. It can be shorter than the server's but not longer. A new value applies from each session's next use.": "Wie lange eine Anmeldung in diesem Mandanten nach ihrem Zeitpunkt und erneut nach jeder Nutzung gilt, etwa 8h, bis zur maximalen Lebensdauer des Servers. Sie kann kürzer sein als die des Servers, aber nicht länger. Ein neuer Wert gilt ab der nächsten Nutzung jeder Sitzung.",
	})
}
