// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// The one-line add box and its quick-add syntax. The syntax itself is
		// not translated: !high, #tag and due:fri are what the parser reads in
		// every language.
		"Quick add":              "Schnell hinzufügen",
		"What the line sets":     "Was die Zeile festlegt",
		"no such project":        "kein solches Projekt",
		"nobody has this handle": "niemand hat dieses Handle",
		"Type the attributes into the line: !high or !1 to !5 for the priority, #tag, @handle for the assignee, due: and a day, +project, ^infra-3 for the parent, status:doing. They are taken out of the title. Put a backslash before one, or quotes around it, to keep it as text.": "Schreiben Sie die Attribute in die Zeile: !high oder !1 bis !5 für die Priorität, #tag, @handle für die zuständige Person, due: und einen Tag, +project, ^infra-3 für die übergeordnete Aufgabe, status:doing. Sie werden aus dem Titel entfernt. Setzen Sie einen Backslash davor oder Anführungszeichen darum, um sie als Text zu behalten.",
	})
}
