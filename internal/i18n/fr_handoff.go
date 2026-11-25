// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Hand to an agent":          "Confier à un agent",
		"Prompt":                    "Prompt",
		"Client":                    "Client",
		"Setting it up:":            "Configuration :",
		"Start it from your client": "Le lancer depuis votre client",
		"With the tix MCP tools":    "Avec les outils MCP de tix",
		"The agent needs tix connected first: the tix CLI or the tix MCP tools, with a token of its own. Nothing here carries a token.": "L’agent a d’abord besoin que tix soit connecté : la CLI tix ou les outils MCP de tix, avec son propre jeton. Rien ici ne contient de jeton.",
		"Run this in the repository the work is in. It is quoted for sh, bash and zsh.":                                                 "Exécutez ceci dans le dépôt où se trouve le travail. C’est échappé pour sh, bash et zsh.",
		"%s has no command that starts it with a prompt. Paste this into its chat; it uses the tix MCP tools:":                          "%s n’a pas de commande qui le lance avec un prompt. Collez ceci dans son chat ; il utilise les outils MCP de tix :",
	})
}
