// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{
		"Ping": "Ping",
		"Send this endpoint a signed webhook.ping event": "Diesem Endpunkt ein signiertes webhook.ping-Ereignis senden",
		"ping delivered": "Ping zugestellt",
		"ping failed":    "Ping fehlgeschlagen",
		"ping queued":    "Ping eingereiht",
	})
}
