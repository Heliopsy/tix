// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{
		"Ping": "Провери",
		"Send this endpoint a signed webhook.ping event": "Изпраща на тази крайна точка подписано събитие webhook.ping",
		"ping delivered": "пробната доставка пристигна",
		"ping failed":    "пробната доставка не успя",
		"ping queued":    "пробната доставка е в опашката",
	})
}
