// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// The shared colour control.
		"Custom":                    "Eigene",
		"Custom colour, as #rrggbb": "Eigene Farbe, als #rrggbb",
		"Pick a custom colour":      "Eigene Farbe wählen",
		"Preview":                   "Vorschau",
		"Pick one of the predefined colours, or choose Custom and set any colour with the picker or as #rrggbb. Each colour scheme draws a custom colour as close to it as stays readable.":            "Wählen Sie eine der vordefinierten Farben, oder wählen Sie „Eigene“ und setzen Sie eine beliebige Farbe mit der Auswahl oder als #rrggbb. Jedes Farbschema stellt eine eigene Farbe so nah daran dar, wie es lesbar bleibt.",
		"The accent this tenant is drawn in. Pick a predefined theme, or choose Custom and set any accent with the picker or as #rrggbb; its tint and its shade for dark schemes are derived from it.": "Die Akzentfarbe dieses Mandanten. Wählen Sie ein vordefiniertes Design, oder wählen Sie „Eigene“ und setzen Sie einen beliebigen Akzent mit der Auswahl oder als #rrggbb; die Aufhellung und die Abdunklung für dunkle Schemata werden daraus abgeleitet.",
	})
}
