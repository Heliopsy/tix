// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// Changing a project's key.
		"The key starts every task reference, such as INFRA-8, and the project's address. Changing it changes every reference; old links, references and filters keep working and lead to the new ones. You are asked to confirm before it changes.": "Der Schlüssel steht am Anfang jeder Aufgabenreferenz, etwa INFRA-8, und der Adresse des Projekts. Eine Änderung ändert jede Referenz; alte Links, Referenzen und Filter funktionieren weiter und führen zu den neuen. Vor der Änderung werden Sie um Bestätigung gebeten.",
		"Change the key of %s?": "Schlüssel von %s ändern?",
		"The key changes from %s to %s, and every task reference changes with it:":                                                      "Der Schlüssel ändert sich von %s zu %s, und jede Aufgabenreferenz ändert sich mit:",
		"Old links, references and filters that use %s keep working and lead here. No other project can take %s while this one exists.": "Alte Links, Referenzen und Filter mit %s funktionieren weiter und führen hierher. Kein anderes Projekt kann %s übernehmen, solange dieses existiert.",
		"Change the key to %s": "Schlüssel in %s ändern",
		"becomes":              "wird zu",
		"A browser cannot carry a file to this screen, so choose the image again.": "Ein Browser kann keine Datei auf diese Seite mitnehmen, wählen Sie das Bild also erneut.",
	})
}
