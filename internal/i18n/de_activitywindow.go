// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The activity filter's action box and its From and To time window.
func init() {
	register(German, map[string]string{
		"Action, for example task.update": "Aktion, z. B. task.update",
		"From":                            "Von",
		"To":                              "Bis",
		"action %s":                       "Aktion %s",
		"on or after %s":                  "am oder nach %s",
		"on or before %s":                 "am oder vor %s",
	})
}
