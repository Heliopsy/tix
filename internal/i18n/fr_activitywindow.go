// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The activity filter's action box and its From and To time window.
func init() {
	register(French, map[string]string{
		"Action, for example task.update": "Action, par exemple task.update",
		"From":                            "Du",
		"To":                              "Au",
		"action %s":                       "action %s",
		"on or after %s":                  "le %s ou après",
		"on or before %s":                 "le %s ou avant",
	})
}
