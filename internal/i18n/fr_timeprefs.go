// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// Date preferences kept with the account, the tenant's default under
		// them and the server's under that.
		"Kept with your account, so every browser and terminal you sign in from shows dates this way. Inherit follows your tenant's default, which follows the server's.":                                                                                                              "Enregistré avec votre compte, pour que chaque navigateur et terminal depuis lequel vous vous connectez affiche les dates ainsi. Hériter suit la valeur par défaut de votre espace, qui suit celle du serveur.",
		"The zone every timestamp on every screen is written in. Inherit uses your tenant's default, and the server's when the tenant has none.":                                                                                                                                       "Le fuseau dans lequel chaque horodatage de chaque écran est écrit. Hériter utilise la valeur par défaut de votre espace, et celle du serveur quand l’espace n’en a pas.",
		"How a timestamp is written, and how a date you type is read: 06.11.2026 is the 6th of November under Dotted and Day first, and 11/06/2026 is under US. 2026-11-06 is read under every format. Inherit uses your tenant's default, and the server's when the tenant has none.": "La façon d’écrire un horodatage, et de lire une date saisie : 06.11.2026 est le 6 novembre avec Pointé et Jour en premier, et 11/06/2026 l’est avec US. 2026-11-06 est lu dans tous les formats. Hériter utilise la valeur par défaut de votre espace, et celle du serveur quand l’espace n’en a pas.",
		"In use:":                "Utilisé :",
		"Tenant default in use:": "Valeur de l’espace utilisée :",
		"from server default":    "selon la valeur du serveur",
		"from tenant default":    "selon la valeur de l’espace",
		"from your choice":       "selon votre choix",
		"server default":         "valeur du serveur",
		"tenant default":         "valeur de l’espace",
		"your choice":            "votre choix",
		"Inherit":                "Hériter",
		"the server's own zone":  "le fuseau du serveur",
		"How this tenant's people read timestamps unless they chose their own under Settings. Inherit uses the server's output.time_format.":      "La façon dont les personnes de cet espace lisent les horodatages, sauf si elles ont choisi la leur dans Paramètres. Hériter utilise output.time_format du serveur.",
		"The zone this tenant's people read timestamps in unless they chose their own under Settings. Inherit uses the server's output.timezone.": "Le fuseau dans lequel les personnes de cet espace lisent les horodatages, sauf si elles ont choisi le leur dans Paramètres. Hériter utilise output.timezone du serveur.",
		"Not yet": "Pas encore",
	})
}
