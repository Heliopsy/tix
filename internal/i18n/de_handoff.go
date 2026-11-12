// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Hand to an agent":          "An einen Agenten übergeben",
		"Prompt":                    "Prompt",
		"Client":                    "Client",
		"Setting it up:":            "Einrichtung:",
		"Start it from your client": "Aus Ihrem Client starten",
		"With the tix MCP tools":    "Mit den tix-MCP-Tools",
		"The agent needs tix connected first: the tix CLI or the tix MCP tools, with a token of its own. Nothing here carries a token.": "Der Agent braucht zuerst eine tix-Verbindung: die tix-CLI oder die tix-MCP-Tools, mit einem eigenen Token. Nichts hier enthält einen Token.",
		"Run this in the repository the work is in. It is quoted for sh, bash and zsh.":                                                 "Führen Sie das im Repository aus, in dem die Arbeit liegt. Es ist für sh, bash und zsh maskiert.",
		"%s has no command that starts it with a prompt. Paste this into its chat; it uses the tix MCP tools:":                          "%s hat keinen Befehl, der es mit einem Prompt startet. Fügen Sie das in seinen Chat ein; es verwendet die tix-MCP-Tools:",
	})
}
