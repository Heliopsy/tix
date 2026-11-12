// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The command palette: its dialog, the opener, the group names and the
// actions it offers.
func init() {
	register(French, map[string]string{
		"Open the command palette": "Ouvrir la palette de commandes",
		"Command palette":          "Palette de commandes",
		"Search":                   "Rechercher",
		"Search tasks, projects, screens and actions": "Rechercher des tâches, projets, écrans et actions",
		"Results": "Résultats",
		"<kbd>↑</kbd> <kbd>↓</kbd> to move, <kbd>Enter</kbd> to open, <kbd>Esc</kbd> to close": "<kbd>↑</kbd> <kbd>↓</kbd> pour se déplacer, <kbd>Enter</kbd> pour ouvrir, <kbd>Esc</kbd> pour fermer",
		"New task in %s":                   "Nouvelle tâche dans %s",
		"Recent":                           "Récents",
		"Actions":                          "Actions",
		"Screens":                          "Écrans",
		"Searching tasks":                  "Recherche de tâches",
		"The task search did not respond.": "La recherche de tâches n’a pas répondu.",
		"No matches":                       "Aucun résultat",
		"Open the filter":                  "Ouvrir le filtre",
		"Show tasks as a list":             "Afficher les tâches en liste",
		"Show tasks as a board":            "Afficher les tâches en tableau",
		"Show tasks in lanes":              "Afficher les tâches en couloirs",
		"Switch the colour scheme":         "Changer de jeu de couleurs",
	})
}
