// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{
		"Update available": "Mise à jour disponible",
		"v%s is available": "v%s est disponible",
		"This installation runs v%s. The commands below install the newer release.": "Cette installation exécute v%s. Les commandes ci-dessous installent la version plus récente.",
		"Up to date":                              "À jour",
		"This installation is up to date":         "Cette installation est à jour",
		"v%s is the newest published release.":    "v%s est la dernière version publiée.",
		"Not comparable":                          "Non comparable",
		"This build's version cannot be compared": "La version de ce build ne peut pas être comparée",
		"v%s is the newest published release, and v%s is not a version number it can be ordered against.": "v%s est la dernière version publiée, et v%s n’est pas un numéro de version avec lequel elle peut être ordonnée.",
		"Check failed":                          "Échec de la vérification",
		"The release feed could not be reached": "Le flux des versions est injoignable",
		"That is the ordinary case for an isolated network. The server tries again within six hours.": "C’est le cas habituel pour un réseau isolé. Le serveur réessaie dans les six heures.",
		"Not checked":     "Non vérifié",
		"Not checked yet": "Pas encore vérifié",
		"The server asks the release feed in the background. Reload in a moment.": "Le serveur interroge le flux des versions en arrière-plan. Rechargez dans un instant.",
		"How to upgrade": "Comment mettre à jour",
		"Use the way this server was installed. Nothing here restarts a running tix serve: restart it once the new version is in place.": "Utilisez la méthode d’installation de ce serveur. Rien ici ne redémarre un tix serve en cours : redémarrez-le une fois la nouvelle version en place.",
		"Release binary": "Binaire de version",
		"Installed with install.sh or unpacked from a release archive. Checks the download against its published checksum before replacing the binary.": "Installé avec install.sh ou décompressé depuis une archive de version. Vérifie le téléchargement avec sa somme de contrôle publiée avant de remplacer le binaire.",
		"Container image": "Image de conteneur",
		"Pull the new tag, then recreate the container on the same volume.": "Récupérez le nouveau tag, puis recréez le conteneur sur le même volume.",
		"Go toolchain": "Chaîne d’outils Go",
		"Installed with go install. tix update refuses this copy, because the toolchain owns it.": "Installé avec go install. tix update refuse cette copie, car la chaîne d’outils la gère.",
		"Installed from a package manager? Upgrade it the way you installed it.":                  "Installé depuis un gestionnaire de paquets ? Mettez-le à jour de la même façon que vous l’avez installé.",
		"What changed in v%s": "Nouveautés de v%s",
		"What's new":          "Nouveautés",
		"From the changelog this build carries: the version running here, then the releases before it.": "D’après le changelog fourni avec ce build : la version qui tourne ici, puis les versions précédentes.",
		"You are running this": "Vous utilisez cette version",
		"v%s is not a tagged release, so these are the newest entries in the changelog it was built with.": "v%s n’est pas une version étiquetée, voici donc les entrées les plus récentes du changelog avec lequel elle a été compilée.",
		"The changelog this build carries has no entry for v%s, so these are its newest entries.":          "Le changelog fourni avec ce build n’a pas d’entrée pour v%s, voici donc ses entrées les plus récentes.",
		"This build carries no changelog.": "Ce build ne contient pas de changelog.",
		"Links have been removed.":         "Les liens ont été retirés.",
	})
}
