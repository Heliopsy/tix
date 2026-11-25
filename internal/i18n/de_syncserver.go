// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The external sync screen's account of what runs on the server: the stored
// mapping document, the sealed credentials and what a source still lacks.
func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Sync runs on the tix server, not on your machine.": "Sync läuft auf dem tix-Server, nicht auf Ihrem Rechner.",
		"The mapping and the credentials a run uses have to be on the server. Upload the mapping and store the credentials in the source dialog, or from the command line with tix sync source set; the server's operator can use TIX_SYNC_ environment variables instead, for the sources it binds to them. Each source below says what its runs will use and what the server is still missing.": "Das Mapping und die Zugangsdaten, die ein Lauf verwendet, müssen auf dem Server liegen. Laden Sie das Mapping hoch und speichern Sie die Zugangsdaten im Quellendialog oder auf der Kommandozeile mit tix sync source set; der Betreiber des Servers kann stattdessen TIX_SYNC_-Umgebungsvariablen für die Quellen verwenden, die er daran bindet. Jede Quelle unten zeigt, was ihre Läufe verwenden und was dem Server noch fehlt.",
		"On the server":        "Auf dem Server",
		"Mapping":              "Mapping",
		"Credentials":          "Zugangsdaten",
		"stored on the source": "in der Quelle gespeichert",
		"file on the server":   "Datei auf dem Server",
		"from the environment": "aus der Umgebung",
		"none needed":          "nicht nötig",
		"missing":              "fehlt",
		"Missing on the server, so it cannot run:": "Fehlt auf dem Server, daher kann sie nicht laufen:",
		"Set up": "Einrichten",
		"Runs execute on the tix server. Everything filled in here is stored there, on the source: the mapping document, the credentials and the settings. Anything left empty comes from the server's TIX_SYNC_ environment variables, which the Which variables list on each row names.": "Läufe werden auf dem tix-Server ausgeführt. Alles, was Sie hier ausfüllen, wird dort in der Quelle gespeichert: das Mapping-Dokument, die Zugangsdaten und die Einstellungen. Was leer bleibt, kommt aus den TIX_SYNC_-Umgebungsvariablen des Servers, die die Liste Welche Variablen in jeder Zeile nennt.",
		"Setting up %s. Saving keeps its stored mapping and credentials unless you replace or clear them.": "%s wird eingerichtet. Beim Speichern bleiben das gespeicherte Mapping und die Zugangsdaten erhalten, sofern Sie sie nicht ersetzen oder leeren.",
		"Checked on the server, nothing saved yet":                                                         "Auf dem Server geprüft, noch nichts gespeichert",
		"Imports into":             "Importiert in",
		"Identified by":            "Identifiziert durch",
		"Statuses":                 "Status",
		"Missing on the server":    "Fehlt auf dem Server",
		"nothing, a run can start": "nichts, ein Lauf kann starten",
		"Credentials typed before a check are not kept: type them again when you save.": "Vor einer Prüfung eingegebene Zugangsdaten werden nicht behalten: Geben Sie sie beim Speichern erneut ein.",
		"Mapping document": "Mapping-Dokument",
		"The YAML that says which tix project records land in and what each external field becomes. Paste it here or choose the file below; it is stored on the source, on the server. Use Check to validate it against the project before saving.": "Das YAML, das festlegt, in welchem tix-Projekt Datensätze landen und was aus jedem externen Feld wird. Fügen Sie es hier ein oder wählen Sie die Datei unten; es wird in der Quelle auf dem Server gespeichert. Mit Prüfen validieren Sie es vor dem Speichern gegen das Projekt.",
		"Or upload a mapping file":   "Oder eine Mapping-Datei hochladen",
		"Mapping file on the server": "Mapping-Datei auf dem Server",
		"A file in this tenant's sync directory on the server, which its operator sets with sync.file_root; a path outside it is refused. A document pasted or uploaded above wins over it.": "Eine Datei im Sync-Verzeichnis dieses Mandanten auf dem Server, das sein Betreiber mit sync.file_root festlegt; ein Pfad außerhalb davon wird abgelehnt. Ein oben eingefügtes oder hochgeladenes Dokument hat Vorrang.",
		"Remove the stored mapping document": "Gespeichertes Mapping-Dokument entfernen",
		"Stored on the server, sealed with its key, and never shown again: only which ones are set and the last four characters of a long one. Leave these empty to keep what is stored. OpenProject takes the user apikey and an API key as the password; Jira takes an account and an API token.": "Auf dem Server gespeichert, mit seinem Schlüssel versiegelt und nie wieder angezeigt: nur welche gesetzt sind und die letzten vier Zeichen eines langen Werts. Lassen Sie die Felder leer, um das Gespeicherte zu behalten. OpenProject nimmt den Benutzer apikey und einen API-Schlüssel als Passwort; Jira nimmt ein Konto und ein API-Token.",
		"API token":                     "API-Token",
		"User":                          "Benutzer",
		"Password or API key":           "Passwort oder API-Schlüssel",
		"Remove the stored credentials": "Gespeicherte Zugangsdaten entfernen",
		"Each setting filled in is stored on the source and wins over its environment variable. generic takes a file in this tenant's sync directory on the server; jira and openproject take a url, a project and a query. The server's environment credentials are never sent to a url stored here.": "Jede ausgefüllte Einstellung wird in der Quelle gespeichert und hat Vorrang vor ihrer Umgebungsvariable. generic nimmt eine Datei im Sync-Verzeichnis dieses Mandanten auf dem Server; jira und openproject nehmen eine url, ein Projekt und eine Abfrage. Die Umgebungs-Zugangsdaten des Servers werden nie an eine hier gespeicherte url gesendet.",
		"File on the server": "Datei auf dem Server",
		"Check":              "Prüfen",
		"A name, a system, and what the form stores on it: the settings, the mapping document and the credentials. A stored value wins over its environment variable on the server, and one left empty is read from that variable when a run starts. Credentials are sealed with the server's key and never shown, audited or logged.": "Ein Name, ein System und was das Formular darin speichert: die Einstellungen, das Mapping-Dokument und die Zugangsdaten. Ein gespeicherter Wert hat auf dem Server Vorrang vor seiner Umgebungsvariable, und ein leer gelassener wird beim Start eines Laufs aus dieser Variable gelesen. Zugangsdaten werden mit dem Schlüssel des Servers versiegelt und nie angezeigt, auditiert oder protokolliert.",
		"A mapping":                          "Ein Mapping",
		"Start from the OpenProject mapping": "Mit dem OpenProject-Mapping beginnen",
		"fills this in with a mapping for OpenProject work packages: subject, description, status, priority, assignee, dates, parent and type. Replace CHANGE-ME with the key of the tix project to import into.": "füllt dies mit einem Mapping für OpenProject-Arbeitspakete: Betreff, Beschreibung, Status, Priorität, Zuständige, Daten, übergeordnetes Element und Typ. Ersetzen Sie CHANGE-ME durch den Schlüssel des tix-Projekts, in das importiert wird.",
		"People with no tix account": "Personen ohne tix-Konto",
		"Map them under users in the mapping, or set users.unknown to placeholder.": "Ordnen Sie sie im Mapping unter users zu oder setzen Sie users.unknown auf placeholder.",
	})
}
