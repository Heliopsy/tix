// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// How a project is named in a picker.
		"Project names in pickers":                  "Noms des projets dans les sélecteurs",
		"How a project reads where you choose one.": "L’apparence d’un projet là où vous le choisissez.",
		"How a project reads in every list you choose one from: the add box, Add with details, Copy to, New from template, a heading's New task in, the move target when a project is deleted, and the statistics filter. The name with its key, the key first, the name alone, or the key alone. A long name is shortened; the key never is.": "L’apparence d’un projet dans chaque liste où vous le choisissez : la zone d’ajout, Ajouter avec détails, Copier vers, Nouvelle à partir d’un modèle, Nouvelle tâche dans d’un en-tête, la destination des tâches quand un projet est supprimé, et le filtre des statistiques. Le nom avec sa clé, la clé d’abord, le nom seul ou la clé seule. Un nom long est raccourci ; la clé jamais.",
		"Apply project names": "Appliquer les noms de projets",
		"Name (KEY)":          "Nom (CLÉ)",
		"KEY · Name":          "CLÉ · Nom",
		"Name":                "Nom",
		"KEY":                 "CLÉ",
	})
}
