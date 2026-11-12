// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"File":                                "Fichier",
		"Files up to %s. Accepted types: %s.": "Fichiers jusqu’à %s. Types acceptés : %s.",
		"Largest artifact file":               "Plus grand fichier d’artefact",
		"Accepted artifact types":             "Types d’artefact acceptés",
		"Leave either empty to use the server's limit, shown greyed.": "Laissez l’un ou l’autre vide pour utiliser la limite du serveur, affichée en grisé.",
		"The largest file an artifact may carry on this tenant, such as 512KiB or 5MiB. It can be lower than the server's limit but not higher. Files already attached stay as they are.":                                                                          "Le plus grand fichier qu’un artefact peut contenir dans cet espace, comme 512KiB ou 5MiB. Il peut être inférieur à la limite du serveur mais pas supérieur. Les fichiers déjà joints restent tels quels.",
		"The media types an artifact file may have on this tenant, separated by commas, such as image/*, application/pdf. Only types the server accepts can be listed. Whatever the type, a file is always offered as a download and never opened in the browser.": "Les types de médias qu’un fichier d’artefact peut avoir dans cet espace, séparés par des virgules, comme image/*, application/pdf. Seuls les types que le serveur accepte peuvent être listés. Quel que soit le type, un fichier est toujours proposé en téléchargement et jamais ouvert dans le navigateur.",
	})
}
