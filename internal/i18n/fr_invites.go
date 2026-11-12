// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// Creating a user: a password or an invite link.
		"How they sign in":       "Comment se connecter",
		"Set a password":         "Définir un mot de passe",
		"Send an invite link":    "Envoyer un lien d’invitation",
		"Create invite link":     "Créer un lien d’invitation",
		"Link expires after":     "Le lien expire après",
		"1 day":                  "1 jour",
		"3 days":                 "3 jours",
		"Invite links":           "Liens d’invitation",
		"Invite link":            "Lien d’invitation",
		"Send this link to them": "Envoyez-lui ce lien",
		"Require a password change at first sign-in":                                                 "Exiger un changement de mot de passe à la première connexion",
		"Ask to change the password at next sign-in":                                                 "Demander de changer le mot de passe à la prochaine connexion",
		"You get a link to send them. It works once, and they choose their own password with it.":    "Vous obtenez un lien à lui envoyer. Il fonctionne une fois, et la personne choisit son propre mot de passe avec.",
		"It works once and expires on %s. tix does not send email, and the link is not shown again.": "Il fonctionne une fois et expire le %s. tix n’envoie pas d’e-mail, et le lien ne sera plus affiché.",
		"An invite link lets the person choose their own password. It works once and expires; tix sends no email, so you pass the link on yourself. A password you type yourself can be made to change at their first sign-in.": "Un lien d’invitation permet à la personne de choisir son propre mot de passe. Il fonctionne une fois et expire ; tix n’envoie pas d’e-mail : vous transmettez donc le lien vous-même. Un mot de passe que vous saisissez vous-même peut devoir être changé à sa première connexion.",
		"Their next sign-in with a password stops at a page asking for a new one, and goes no further until they choose it. Sessions already open and API tokens are not affected.":                                             "Sa prochaine connexion avec un mot de passe s’arrête sur une page qui en demande un nouveau, et ne va pas plus loin tant qu’elle ne l’a pas choisi. Les sessions déjà ouvertes et les jetons d’API ne sont pas concernés.",

		// A change the account's other tenants keep from a tenant admin.
		"%s also belongs to another tenant, so only they or a global admin can ask them to change their password. Leave the box as it was to save the rest.": "%s appartient aussi à un autre espace : seuls cette personne ou un administrateur global peuvent lui demander de changer son mot de passe. Laissez la case telle quelle pour enregistrer le reste.",
		"%s also belongs to another tenant, so only they or a global admin can clear their second factor.":                                                   "%s appartient aussi à un autre espace : seuls cette personne ou un administrateur global peuvent effacer son second facteur.",

		// Pending invites.
		"Pending invites": "Invitations en attente",
		"Links issued and not used yet. Re-issuing stops the old link and gives you a new one to send; revoking stops it and keeps the account, without a password.": "Liens émis et pas encore utilisés. Réémettre arrête l’ancien lien et vous en donne un nouveau à envoyer ; révoquer l’arrête et conserve le compte, sans mot de passe.",
		"invited":                    "invité",
		"invited by %s":              "invité par %s",
		"expires %s":                 "expire %s",
		"Re-issue":                   "Réémettre",
		"Re-issue the invite for %s": "Réémettre l’invitation de %s",
		"Revoke the invite for %s":   "Révoquer l’invitation de %s",
		"Revoke the invite for %s?":  "Révoquer l’invitation de %s ?",
		"The link stops working at once. The account stays, without a password, until you delete it or invite it again.": "Le lien cesse aussitôt de fonctionner. Le compte reste, sans mot de passe, jusqu’à ce que vous le supprimiez ou l’invitiez à nouveau.",
		"invite created":   "invitation créée",
		"invite re-issued": "invitation réémise",
		"invite revoked":   "invitation révoquée",

		// The invite page.
		"Accept your invite": "Accepter votre invitation",
		"Join %s":            "Rejoindre %s",
		"Choose a password to finish setting up your account. This link works once.":    "Choisissez un mot de passe pour terminer la configuration de votre compte. Ce lien fonctionne une fois.",
		"You already have an account. Enter its password to accept; it stays as it is.": "Vous avez déjà un compte. Saisissez son mot de passe pour accepter ; il reste tel quel.",
		"Signing in as":                "Connexion en tant que",
		"Current password":             "Mot de passe actuel",
		"Password again":               "Confirmer le mot de passe",
		"At least 12 characters.":      "Au moins 12 caractères.",
		"Accept and sign in":           "Accepter et se connecter",
		"Code from your authenticator": "Code de votre authentificateur",
		"Your account has a second factor, so joining another tenant asks for a code from it too.": "Votre compte a un second facteur : rejoindre un autre espace demande donc aussi un code de celui-ci.",
		"This link does not work": "Ce lien ne fonctionne pas",
		"This invite link is not valid. It may have expired, been used already or been revoked. Ask whoever invited you for a new one.": "Ce lien d’invitation n’est pas valide. Il a peut-être expiré, déjà été utilisé ou été révoqué. Demandez-en un nouveau à la personne qui vous a invité.",
		"Go to sign in": "Aller à la connexion",

		// The password change a sign-in can be stopped at.
		"Choose a new password": "Choisir un nouveau mot de passe",
		"Your administrator asked you to replace your password before you go on.": "Votre administrateur vous a demandé de remplacer votre mot de passe avant de continuer.",
		"New password":       "Nouveau mot de passe",
		"New password again": "Confirmer le nouveau mot de passe",
		"At least 12 characters, and not the one you have now.": "Au moins 12 caractères, et différent de l’actuel.",
		"Save and sign in": "Enregistrer et se connecter",
		"Back to sign in":  "Retour à la connexion",
	})
}
