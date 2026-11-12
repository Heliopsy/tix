// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"File":                                "Datei",
		"Files up to %s. Accepted types: %s.": "Dateien bis %s. Erlaubte Typen: %s.",
		"Largest artifact file":               "Größte Artefaktdatei",
		"Accepted artifact types":             "Erlaubte Artefakttypen",
		"Leave either empty to use the server's limit, shown greyed.": "Lassen Sie ein Feld leer, um das grau angezeigte Limit des Servers zu verwenden.",
		"The largest file an artifact may carry on this tenant, such as 512KiB or 5MiB. It can be lower than the server's limit but not higher. Files already attached stay as they are.":                                                                          "Die größte Datei, die ein Artefakt in diesem Mandanten enthalten darf, etwa 512KiB oder 5MiB. Der Wert kann unter dem Limit des Servers liegen, aber nicht darüber. Bereits angehängte Dateien bleiben unverändert.",
		"The media types an artifact file may have on this tenant, separated by commas, such as image/*, application/pdf. Only types the server accepts can be listed. Whatever the type, a file is always offered as a download and never opened in the browser.": "Die Medientypen, die eine Artefaktdatei in diesem Mandanten haben darf, durch Kommas getrennt, etwa image/*, application/pdf. Nur Typen, die der Server akzeptiert, können angegeben werden. Unabhängig vom Typ wird eine Datei immer als Download angeboten und nie im Browser geöffnet.",
	})
}
