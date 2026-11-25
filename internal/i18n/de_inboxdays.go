// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Summary":                                "Übersicht",
		"Open":                                   "Offen",
		"Due today":                              "Heute fällig",
		"Done in 7 days":                         "In 7 Tagen erledigt",
		"Kinds to show":                          "Anzuzeigende Arten",
		"Nothing of these kinds.":                "Nichts von diesen Arten.",
		"Yesterday":                              "Gestern",
		"Earlier":                                "Früher",
		"Stop mentions":                          "Keine Erwähnungen mehr",
		"Stop assignments":                       "Keine Zuweisungen mehr",
		"Stop comments":                          "Keine Kommentare mehr",
		"Stop status changes":                    "Keine Statuswechsel mehr",
		"Mentions no longer reach your inbox":    "Erwähnungen erreichen Ihren Posteingang nicht mehr",
		"Assignments no longer reach your inbox": "Zuweisungen erreichen Ihren Posteingang nicht mehr",
		"Comments no longer reach your inbox":    "Kommentare erreichen Ihren Posteingang nicht mehr",
		"Status changes no longer reach your inbox":                                "Statuswechsel erreichen Ihren Posteingang nicht mehr",
		"Items already in your inbox stay. Turn it back on under Settings, Inbox.": "Einträge, die schon in Ihrem Posteingang sind, bleiben. Unter Einstellungen, Posteingang lässt es sich wieder einschalten.",
	})
}
