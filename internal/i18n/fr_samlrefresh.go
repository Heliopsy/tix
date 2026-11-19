// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{
		"%d retiring certificates":                                           "%d certificats en retrait",
		"IdP metadata not read yet":                                          "Métadonnées de l'IdP pas encore lues",
		"IdP metadata read failed":                                           "La lecture des métadonnées de l'IdP a échoué",
		"IdP metadata read, certificates updated":                            "Métadonnées de l'IdP lues, certificats mis à jour",
		"IdP metadata read, nothing changed":                                 "Métadonnées de l'IdP lues, rien n'a changé",
		"Read the IdP metadata again now and merge its signing certificates": "Relire maintenant les métadonnées de l'IdP et en reprendre les certificats de signature",
		"Refresh metadata":                                                   "Actualiser les métadonnées",
		"metadata refresh failed":                                            "l'actualisation des métadonnées a échoué",
		"metadata refreshed, certificates updated":                           "métadonnées actualisées, certificats modifiés",
		"metadata refreshed, nothing changed":                                "métadonnées actualisées, rien n'a changé",
		"next read":                                                          "prochaine lecture",
		"not read on a schedule":                                             "pas de lecture planifiée",
		"the stored certificates keep working":                               "les certificats enregistrés restent valables",
	})
}
