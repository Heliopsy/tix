// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// The global admin area: every tenant, the global admins, and the
		// users of any tenant.
		"Global admin":  "Administrateur global",
		"Global admins": "Administrateurs globaux",
		"Every tenant of this installation, and who may administer all of them.": "Chaque espace de cette installation, et qui peut tous les administrer.",
		"Tenants":           "Espaces",
		"No tenants yet":    "Aucun espace pour l’instant",
		"Create a tenant":   "Créer un espace",
		"Create tenant":     "Créer l’espace",
		"First admins":      "Premiers administrateurs",
		"Rename %s":         "Renommer %s",
		"Users of %s":       "Utilisateurs de %s",
		"Users of a tenant": "Utilisateurs d’un espace",
		"One email per line. Each becomes an admin of the new tenant. A new address gets a password generated and shown to you once; an address that already has an account keeps its own password and second factor.": "Une adresse e-mail par ligne. Chacune devient administrateur du nouvel espace. Une nouvelle adresse reçoit un mot de passe généré et affiché une seule fois ; une adresse qui a déjà un compte garde son mot de passe et son second facteur.",
		"Copy these passwords now": "Copiez ces mots de passe maintenant",
		"They are shown once and cannot be retrieved again. If one is lost, set a new password from the tenant's users.":                                          "Ils ne sont affichés qu’une fois et ne peuvent plus être récupérés. Si l’un est perdu, définissez un nouveau mot de passe depuis les utilisateurs de l’espace.",
		"Its people can no longer sign in, its data stops being reachable and its domains stop resolving. Nothing is destroyed, but nothing here brings it back.": "Ses membres ne peuvent plus se connecter, ses données deviennent inaccessibles et ses domaines ne répondent plus. Rien n’est détruit, mais rien ici ne permet de le rétablir.",
		"Grant global admin":           "Nommer administrateur global",
		"Revoke global admin from %s":  "Retirer le rôle d’administrateur global à %s",
		"Revoke global admin from %s?": "Retirer le rôle d’administrateur global à %s ?",
		"A global admin creates tenants and administers every tenant here. Name a user who already exists, in any tenant.":                     "Un administrateur global crée des espaces et administre tous les espaces d’ici. Nommez un utilisateur qui existe déjà, dans n’importe quel espace.",
		"They keep their own tenant and their role in it, and stop administering the others at once. The last global admin cannot be revoked.": "Il garde son propre espace et son rôle dans celui-ci, et cesse aussitôt d’administrer les autres. Le dernier administrateur global ne peut pas être retiré.",
		"tenant created":       "espace créé",
		"tenant renamed":       "espace renommé",
		"tenant deleted":       "espace supprimé",
		"global admin granted": "administrateur global nommé",
		"global admin revoked": "rôle d’administrateur global retiré",
		// Shared domains, set by a global admin on a tenant's screen.
		"Shared domains":   "Domaines partagés",
		"Shared":           "Partagé",
		"this tenant only": "cet espace uniquement",
		"Share this host":  "Partager cet hôte",
		"Stop sharing":     "Ne plus partager",
		"A shared domain's host also serves the tenant a session, a token or the X-Tix-Tenant header names, so several tenants work on one address. With neither it serves this tenant. A domain that is not shared serves this tenant alone and refuses every other tenant's credential. Only a global admin changes it.": "L’hôte d’un domaine partagé sert aussi l’espace qu’indiquent une session, un jeton ou l’en-tête X-Tix-Tenant, pour que plusieurs espaces fonctionnent sur une même adresse. Sans aucun d’eux, il sert cet espace. Un domaine non partagé ne sert que cet espace et refuse l’identifiant de tout autre espace. Seul un administrateur global peut le modifier.",
		"domain shared":           "domaine partagé",
		"domain no longer shared": "domaine plus partagé",
	})
}
