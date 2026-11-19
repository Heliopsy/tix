// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{
		"%d retiring certificates":                                           "%d auslaufende Zertifikate",
		"IdP metadata not read yet":                                          "IdP-Metadaten noch nicht gelesen",
		"IdP metadata read failed":                                           "Lesen der IdP-Metadaten fehlgeschlagen",
		"IdP metadata read, certificates updated":                            "IdP-Metadaten gelesen, Zertifikate aktualisiert",
		"IdP metadata read, nothing changed":                                 "IdP-Metadaten gelesen, nichts geändert",
		"Read the IdP metadata again now and merge its signing certificates": "Die IdP-Metadaten jetzt neu lesen und ihre Signaturzertifikate übernehmen",
		"Refresh metadata":                                                   "Metadaten aktualisieren",
		"metadata refresh failed":                                            "Aktualisieren der Metadaten fehlgeschlagen",
		"metadata refreshed, certificates updated":                           "Metadaten aktualisiert, Zertifikate geändert",
		"metadata refreshed, nothing changed":                                "Metadaten aktualisiert, nichts geändert",
		"next read":                                                          "nächstes Lesen",
		"not read on a schedule":                                             "wird nicht planmäßig gelesen",
		"the stored certificates keep working":                               "die gespeicherten Zertifikate gelten weiter",
	})
}
