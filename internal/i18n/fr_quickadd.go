// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// The one-line add box and its quick-add syntax. The syntax itself is
		// not translated: !high, #tag and due:fri are what the parser reads in
		// every language.
		"Quick add":              "Ajout rapide",
		"What the line sets":     "Ce que la ligne définit",
		"no such project":        "projet introuvable",
		"nobody has this handle": "personne n’a cet identifiant",
		"Type the attributes into the line: !high or !1 to !5 for the priority, #tag, @handle for the assignee, due: and a day, +project, ^infra-3 for the parent, status:doing. They are taken out of the title. Put a backslash before one, or quotes around it, to keep it as text.": "Saisissez les attributs dans la ligne : !high ou !1 à !5 pour la priorité, #tag, @handle pour le responsable, due: suivi d’un jour, +project, ^infra-3 pour le parent, status:doing. Ils sont retirés du titre. Placez une barre oblique inverse devant l’un d’eux, ou entourez-le de guillemets, pour le garder comme texte.",
	})
}
