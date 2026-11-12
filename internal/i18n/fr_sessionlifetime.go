// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Keep me signed in":         "Rester connecté",
		"Up to %s on this browser.": "Jusqu’à %s sur ce navigateur.",
		"Kept signed in.":           "Connexion maintenue.",
		"Sign-in lasts":             "Durée d’une connexion",
		"Sign out when idle for":    "Déconnecter après une inactivité de",
		"Keep me signed in lasts":   "Durée de Rester connecté",
		"never":                     "jamais",
		"Leave any empty to use the server's setting, shown greyed.": "Laissez un champ vide pour utiliser le réglage du serveur, affiché en grisé.",
		"Stay signed in on this browser after it closes, until the time shown. Leave it unticked on a shared computer. If this tenant ends sessions left unused, that still applies.":                                                                     "Rester connecté sur ce navigateur après sa fermeture, jusqu’à l’heure indiquée. Laissez la case décochée sur un ordinateur partagé. Si cet espace met fin aux sessions inutilisées, cela s’applique toujours.",
		"How long a sign-in on this tenant lasts from the moment it is made, such as 8h. It can be shorter than the server's but not longer. Sessions already signed in keep the expiry they were given.":                                                 "Durée d’une connexion à cet espace à partir du moment où elle est établie, par exemple 8h. Elle peut être plus courte que celle du serveur, mais pas plus longue. Les sessions déjà connectées gardent l’expiration qui leur a été donnée.",
		"End a session nobody has used for this long, such as 30m, kept signed in or not. It applies to sessions already signed in as soon as it is saved. It can be shorter than the server's but not longer; empty with no server setting means never.": "Mettre fin à une session que personne n’a utilisée depuis cette durée, par exemple 30m, qu’elle soit maintenue connectée ou non. Cela s’applique aux sessions déjà connectées dès l’enregistrement. Elle peut être plus courte que celle du serveur, mais pas plus longue ; vide sans réglage serveur signifie jamais.",
		"How long a sign-in lasts when the person ticks Keep me signed in, such as 7d, or off to stop offering it. It can be shorter than the server's but not longer.":                                                                                   "Durée d’une connexion quand la personne coche Rester connecté, par exemple 7d, ou off pour ne plus le proposer. Elle peut être plus courte que celle du serveur, mais pas plus longue.",
	})
}
