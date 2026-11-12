// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Snooze":                                 "Reporter",
		"Unsnooze":                               "Ne plus reporter",
		"Snooze %s":                              "Reporter %s",
		"Snoozed until %s":                       "Reportée jusqu’à %s",
		"Snoozed by %s":                          "Reportée par %s",
		"snoozed by %s":                          "reportée par %s",
		"until %s":                               "jusqu’à %s",
		"Later today":                            "Plus tard aujourd’hui",
		"Tomorrow morning":                       "Demain matin",
		"Next week":                              "La semaine prochaine",
		"Pick a date":                            "Choisir une date",
		"Hide it from every listing until later": "La masquer de toutes les listes jusqu’à plus tard",
		"A snoozed task leaves every listing, for everybody, until the time you pick, then comes back by itself. Agents cannot claim it meanwhile. is:snoozed finds it.": "Une tâche reportée quitte toutes les listes, pour tout le monde, jusqu’à l’heure choisie, puis revient d’elle-même. Les agents ne peuvent pas la prendre entre-temps. is:snoozed la retrouve.",
	})
}
