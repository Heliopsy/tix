// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"This tenant requires a second factor. Set one up to go on.": "Cet espace exige un second facteur. Configurez-en un pour continuer.",
		"This tenant requires a second factor from %s.":              "Cet espace exige un second facteur à partir du %s.",
		"Set up a second factor":                                     "Configurer un second facteur",
		"This tenant requires a second factor, and this account has none yet. Until you add one you can only set it up, read your account's history or sign out.": "Cet espace exige un second facteur, et ce compte n’en a pas encore. Tant que vous n’en ajoutez pas, vous pouvez seulement le configurer, lire l’historique de votre compte ou vous déconnecter.",
		"This tenant requires a second factor from %s. Add one now and nothing changes for you then.":                                                             "Cet espace exige un second facteur à partir du %s. Ajoutez-en un maintenant et rien ne changera pour vous à cette date.",
		"A second factor asks for a code from your phone after your password, so a leaked password alone does not sign anybody in.":                               "Un second facteur demande un code de votre téléphone après votre mot de passe : un mot de passe divulgué ne suffit donc à connecter personne.",
		"You need an authenticator app on your phone, such as one from your phone's maker or any TOTP app.":                                                       "Il vous faut une application d’authentification sur votre téléphone, comme celle du fabricant de votre téléphone ou n’importe quelle application TOTP.",
		"Everybody":                "Tout le monde",
		"Without a second factor":  "Sans second facteur",
		"2FA":                      "2FA",
		"no 2FA":                   "sans 2FA",
		"Second factor policy":     "Politique de second facteur",
		"Off: each person chooses": "Désactivée : chacun choisit",
		"Required for everybody":   "Obligatoire pour tout le monde",
		"Grace period":             "Délai de grâce",
		"People without a second factor are refused from %s.": "Les personnes sans second facteur sont refusées à partir du %s.",
		"Who has none yet": "Qui n’en a pas encore",
		"Required: on every surface, a person with no second factor can only set one up, read their own account or sign out, with a session or with a token of theirs. Tokens of agents are not affected. You need a second factor yourself before you can turn it on.": "Obligatoire : sur toutes les interfaces, une personne sans second facteur peut seulement en configurer un, lire son propre compte ou se déconnecter, avec une session ou un de ses jetons. Les jetons des agents ne sont pas concernés. Il vous faut vous-même un second facteur avant de pouvoir l’activer.",
		"How long people without a second factor may keep working after the policy is turned on, with a banner naming the deadline. Written as a duration: 7d, 48h. Empty or 0 applies it at once.":                                                                     "Combien de temps les personnes sans second facteur peuvent continuer à travailler après l’activation de la politique, avec une bannière indiquant la date limite. S’écrit comme une durée : 7d, 48h. Vide ou 0 l’applique aussitôt.",
	})
}
