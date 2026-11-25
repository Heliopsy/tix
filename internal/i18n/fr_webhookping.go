// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{
		"Ping": "Ping",
		"Send this endpoint a signed webhook.ping event": "Envoyer à ce point d’accès un événement webhook.ping signé",
		"ping delivered": "ping livré",
		"ping failed":    "échec du ping",
		"ping queued":    "ping en file d’attente",
	})
}
