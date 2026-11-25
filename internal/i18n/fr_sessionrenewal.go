// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Stay signed in on this browser after it closes. Each visit keeps it going, up to a limit this server sets. Leave it unticked on a shared computer. If this tenant ends sessions left unused, that still applies.": "Rester connecté sur ce navigateur après sa fermeture. Chaque visite prolonge la connexion, jusqu’à une limite fixée par ce serveur. Laissez la case décochée sur un ordinateur partagé. Si cet espace met fin aux sessions inutilisées, cela s’applique toujours.",
		"For %s after your last visit, on this browser.": "Pendant %s après votre dernière visite, sur ce navigateur.",
		"How long a sign-in on this tenant lasts after it is made and again after each use, such as 8h, up to the server's maximum lifetime. It can be shorter than the server's but not longer. A new value applies from each session's next use.": "Durée d’une connexion à cet espace après son établissement et de nouveau après chaque utilisation, par exemple 8h, dans la limite de la durée de vie maximale du serveur. Elle peut être plus courte que celle du serveur, mais pas plus longue. Une nouvelle valeur s’applique dès la prochaine utilisation de chaque session.",
	})
}
