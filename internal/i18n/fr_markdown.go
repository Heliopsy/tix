// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Preview":             "Aperçu",
		"Write":               "Écrire",
		"Loading preview…":    "Chargement de l’aperçu…",
		"Nothing to preview.": "Rien à prévisualiser.",
		"The preview could not be drawn. Your text is unchanged.": "L’aperçu n’a pas pu être affiché. Votre texte est inchangé.",
		"Checklist":                     "Checklist",
		"%d of %d checklist items done": "%d éléments de checklist faits sur %d",
		"item ticked":                   "élément coché",
		"item cleared":                  "élément décoché",
		"Written in Markdown: **bold**, lists, tables, `code`, links, and - [ ] for a checklist the task screen can tick. A reference such as INFRA-8 and an @handle become links. HTML is shown as typed, and images are linked rather than loaded.": "Écrit en Markdown : **gras**, listes, tableaux, `code`, liens, et - [ ] pour une checklist que l’écran de la tâche peut cocher. Une référence comme INFRA-8 et un @identifiant deviennent des liens. Le HTML est affiché tel que saisi, et les images sont liées plutôt que chargées.",
	})
}
