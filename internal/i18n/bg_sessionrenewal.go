// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Stay signed in on this browser after it closes. Each visit keeps it going, up to a limit this server sets. Leave it unticked on a shared computer. If this tenant ends sessions left unused, that still applies.": "Останете влезли в този браузър и след затварянето му. Всяко посещение го удължава, до граница, зададена от сървъра. Не го отмятайте на споделен компютър. Ако този тенант прекратява неизползвани сесии, това продължава да важи.",
		"For %s after your last visit, on this browser.": "За %s след последното ви посещение, в този браузър.",
		"How long a sign-in on this tenant lasts after it is made and again after each use, such as 8h, up to the server's maximum lifetime. It can be shorter than the server's but not longer. A new value applies from each session's next use.": "Колко трае входът в този тенант след като е направен и отново след всяко ползване, например 8h, до максималния живот, зададен от сървъра. Може да е по-кратко от това на сървъра, но не и по-дълго. Новата стойност важи от следващото ползване на всяка сесия.",
	})
}
