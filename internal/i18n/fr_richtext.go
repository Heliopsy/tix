// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Editor":         "Éditeur",
		"Rich text":      "Texte enrichi",
		"Markdown":       "Markdown",
		"Formatting":     "Mise en forme",
		"Bold":           "Gras",
		"Italic":         "Italique",
		"Strikethrough":  "Barré",
		"Heading":        "Titre",
		"Bulleted list":  "Liste à puces",
		"Numbered list":  "Liste numérotée",
		"Quote":          "Citation",
		"Link":           "Lien",
		"Table":          "Tableau",
		"Add row":        "Ajouter une ligne",
		"Add column":     "Ajouter une colonne",
		"Delete row":     "Supprimer la ligne",
		"Delete column":  "Supprimer la colonne",
		"Delete table":   "Supprimer le tableau",
		"Link address":   "Adresse du lien",
		"Add link":       "Ajouter un lien",
		"Checklist item": "Élément de checklist",
		"The rich text editor could not be loaded. Your text is unchanged.":                          "L’éditeur de texte enrichi n’a pas pu être chargé. Votre texte est inchangé.",
		"This text uses Markdown the rich text editor cannot keep exactly, so it stays in Markdown.": "Ce texte utilise du Markdown que l’éditeur de texte enrichi ne peut pas conserver tel quel, il reste donc en Markdown.",
	})
}
