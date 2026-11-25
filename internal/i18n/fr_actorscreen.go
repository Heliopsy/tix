// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// Settings, Profile.
		"Profile": "Profil",
		"Who you are signed in as here, and with what authority.": "Sous quelle identité vous êtes connecté ici, et avec quelle autorité.",
		"Open your actor page": "Ouvrir votre page d’acteur",
		"The short name other people type to mention you or assign you work, unique within this tenant.": "Le nom court que les autres saisissent pour vous mentionner ou vous assigner du travail, unique dans cet espace.",
		"Actor id":           "Identifiant d’acteur",
		"Copy your actor id": "Copier votre identifiant d’acteur",
		"The identifier the command line, the API and the audit log use for you. It never changes, while a handle can.": "L’identifiant que la ligne de commande, l’API et le journal d’audit utilisent pour vous. Il ne change jamais, alors qu’un identifiant court peut changer.",
		"Your role in this tenant. Its scopes add to any granted to you directly.":                                      "Votre rôle dans cet espace. Ses portées s’ajoutent à celles qui vous sont accordées directement.",
		"Whether you may administer the whole installation, every tenant included, right now.":                          "Si vous pouvez administrer toute l’installation, tous les espaces compris, en ce moment.",
		"Signed in with": "Connecté avec",
		"What this request authenticated with: the session this browser holds, or an API token.": "Ce avec quoi cette requête s’est authentifiée : la session de ce navigateur, ou un jeton d’API.",
		"Browser session": "Session du navigateur",
		"Everything you may do here, from your role and from any scope granted to you directly.":                           "Tout ce que vous pouvez faire ici, d’après votre rôle et les portées qui vous sont accordées directement.",
		"The address you sign in with. It belongs to your account, which is the same in every tenant you are a member of.": "L’adresse avec laquelle vous vous connectez. Elle appartient à votre compte, qui est le même dans chaque espace dont vous êtes membre.",
		"Every tenant your account is a member of, with your handle and role there.":                                       "Chaque espace dont votre compte est membre, avec votre identifiant et votre rôle.",
		"this tenant": "cet espace",
		"Not set":     "Non défini",
		"None":        "Aucun",
		"Yes":         "Oui",
		"No":          "Non",

		// The actor screen.
		"A person or an agent of this tenant, by the name others use for it. What it may do is not shown here.": "Une personne ou un agent de cet espace, sous le nom que les autres utilisent. Ce qu’il peut faire n’est pas affiché ici.",
		"About":                "À propos",
		"Copy the actor id":    "Copier l’identifiant d’acteur",
		"Tasks assigned to %s": "Tâches assignées à %s",
		"Your profile":         "Votre profil",
		"Recent activity":      "Activité récente",
		"All activity":         "Toute l’activité",
		"Recent activity needs the audit:read scope, which you do not hold.": "L’activité récente nécessite la portée audit:read, que vous n’avez pas.",
		"Open %s's page": "Ouvrir la page de %s",

		// The credential filter on the token and SSH key screens.
		"Whose credentials":                    "Identifiants de qui",
		"Handle or id, empty for yours":        "Identifiant ou id d’acteur, vide pour les vôtres",
		"Show":                                 "Afficher",
		"Show yours":                           "Afficher les vôtres",
		"No actor of this tenant is named %s.": "Aucun acteur de cet espace ne s’appelle %s.",
		"Listing what %s holds.":               "Liste de ce que %s détient.",
		"Keys enrolled against the actor named below. Enrolling a key here always enrols it against your own actor.": "Clés enregistrées pour l’acteur indiqué ci-dessous. Enregistrer une clé ici l’enregistre toujours pour votre propre acteur.",

		// The subtask tree.
		"Direct subtasks":  "Sous-tâches directes",
		"Subtasks of %s":   "Sous-tâches de %s",
		"Show %d subtask":  "Afficher %d sous-tâche",
		"Show %d subtasks": "Afficher %d sous-tâches",
		"This tree is too large to draw whole, so only direct subtasks are shown.":                                                                                         "Cette arborescence est trop grande pour être dessinée en entier ; seules les sous-tâches directes sont affichées.",
		"The whole tree under this task. Three levels show at once, and a deeper level loads when you ask for it. The arrow beside a subtask folds its own subtasks away.": "Toute l’arborescence sous cette tâche. Trois niveaux s’affichent à la fois, et un niveau plus profond se charge à la demande. La flèche à côté d’une sous-tâche replie ses propres sous-tâches.",
	})
}
