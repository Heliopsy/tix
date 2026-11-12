// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// Changing a project's key.
		"The key starts every task reference, such as INFRA-8, and the project's address. Changing it changes every reference; old links, references and filters keep working and lead to the new ones. You are asked to confirm before it changes.": "La clé commence chaque référence de tâche, comme INFRA-8, et l’adresse du projet. La modifier modifie chaque référence ; les anciens liens, références et filtres continuent de fonctionner et mènent aux nouveaux. Une confirmation vous est demandée avant le changement.",
		"Change the key of %s?": "Modifier la clé de %s ?",
		"The key changes from %s to %s, and every task reference changes with it:":                                                      "La clé passe de %s à %s, et chaque référence de tâche change avec elle :",
		"Old links, references and filters that use %s keep working and lead here. No other project can take %s while this one exists.": "Les anciens liens, références et filtres qui utilisent %s continuent de fonctionner et mènent ici. Aucun autre projet ne peut prendre %s tant que celui-ci existe.",
		"Change the key to %s": "Modifier la clé en %s",
		"becomes":              "devient",
		"A browser cannot carry a file to this screen, so choose the image again.": "Un navigateur ne peut pas transmettre un fichier à cet écran : choisissez à nouveau l’image.",
	})
}
