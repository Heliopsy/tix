// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Hand to an agent":          "Дай на агент",
		"Prompt":                    "Инструкция",
		"Client":                    "Клиент",
		"Setting it up:":            "Настройка:",
		"Start it from your client": "Стартирай го от своя клиент",
		"With the tix MCP tools":    "С MCP инструментите на tix",
		"The agent needs tix connected first: the tix CLI or the tix MCP tools, with a token of its own. Nothing here carries a token.": "Агентът първо трябва да има връзка с tix: командния ред tix или MCP инструментите на tix, със собствен токен. Нищо тук не съдържа токен.",
		"Run this in the repository the work is in. It is quoted for sh, bash and zsh.":                                                 "Изпълни това в хранилището, в което е работата. Кавичките са за sh, bash и zsh.",
		"%s has no command that starts it with a prompt. Paste this into its chat; it uses the tix MCP tools:":                          "%s няма команда, която да го стартира с инструкция. Постави това в чата му; то използва MCP инструментите на tix:",
	})
}
