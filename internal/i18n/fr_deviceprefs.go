// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Keep tix still even when the system allows animation.": "Garder tix immobile même si le système autorise les animations.",
		"Reduce motion": "Réduire les animations",
		"Each setting says where it is kept: on this device, in this browser's cookies or storage; synced to your account, so it follows you everywhere you sign in; or for the whole tenant.":                                                    "Chaque paramètre indique où il est conservé : sur cet appareil, dans les cookies ou le stockage de ce navigateur ; synchronisé avec votre compte, pour vous suivre partout où vous vous connectez ; ou pour tout l’espace.",
		"The screen this browser opens when you go to the address of tix itself, and after you sign in without following a link.":                                                                                                                 "L’écran que ce navigateur ouvre quand vous allez à l’adresse de tix elle-même, et après une connexion sans avoir suivi de lien.",
		"A notification from your browser for each new inbox item, while a tix tab is open and you are looking at something else. Turning it on asks the browser for permission. A closed tab cannot be told: channels below deliver everywhere.": "Une notification de votre navigateur pour chaque nouvel élément de la boîte de réception, tant qu’un onglet tix est ouvert et que vous regardez autre chose. L’activer demande l’autorisation au navigateur. Un onglet fermé ne peut pas être prévenu : les canaux ci-dessous livrent partout.",
		"Browser notifications while a tix tab is open.": "Notifications du navigateur tant qu’un onglet tix est ouvert.",
		"Notifications say only that something new reached your inbox, without who did what or the task's title, for a screen other people can see.": "Les notifications disent seulement que quelque chose de nouveau est arrivé dans votre boîte de réception, sans qui a fait quoi ni le titre de la tâche, pour un écran que d’autres peuvent voir.",
		"Notifications are blocked for this site. Allow them in the browser's site settings, then turn this on.":                                     "Les notifications sont bloquées pour ce site. Autorisez-les dans les paramètres du site du navigateur, puis activez ceci.",
		"Notify me about new inbox items": "Me prévenir des nouveaux éléments de la boîte de réception",
		"Turns off animation, sliding and smooth scrolling in this browser. Your system's own reduced motion setting already does this; this keeps tix still even when the system allows animation.": "Désactive les animations, les glissements et le défilement fluide dans ce navigateur. Le réglage de réduction des animations de votre système le fait déjà ; celui-ci garde tix immobile même si le système autorise les animations.",
		"Say only that something arrived.":                "Dire seulement que quelque chose est arrivé.",
		"Where tix opens in this browser.":                "Où tix s’ouvre dans ce navigateur.",
		"Settings here apply to everyone in this tenant.": "Les paramètres ici s’appliquent à tout le monde dans cet espace.",
		"On this device":                                  "Sur cet appareil",
		"Start screen":                                    "Écran d’accueil",
		"Something new reached your inbox.":               "Quelque chose de nouveau est arrivé dans votre boîte de réception.",
		"No setting matches “%s”":                         "Aucun paramètre ne correspond à « %s »",
		"Try another word, or clear the search.":          "Essayez un autre mot, ou effacez la recherche.",
		"Apply start screen":                              "Appliquer l’écran d’accueil",
		"Synced to your account":                          "Synchronisé avec votre compte",
		"Hide what they say":                              "Masquer le contenu",
		"This browser cannot show notifications.":         "Ce navigateur ne peut pas afficher de notifications.",
		"Search settings":                                 "Rechercher dans les paramètres",
		"Whole tenant":                                    "Tout l’espace",
	})
}
