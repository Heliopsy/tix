// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Something wrong?": "Un problème ?",
		"Copy these facts into a bug report on the tix project's issue tracker. They name versions and this browser, never a token, a password or your work.": "Copiez ces informations dans un rapport de bogue sur le gestionnaire de tickets du projet tix. Elles indiquent des versions et ce navigateur, jamais un jeton, un mot de passe ou votre travail.",
		"Include the server address":                                 "Inclure l’adresse du serveur",
		"Off keeps a private server's address out of what you copy.": "Désactivé, l’adresse d’un serveur privé reste hors de ce que vous copiez.",
		"Copy diagnostics":                                           "Copier les diagnostics",
	})
}
