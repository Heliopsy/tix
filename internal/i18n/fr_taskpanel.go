// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// Task opens in's Panel position and the divider beside the panel.
func init() {
	register(French, map[string]string{
		"Open a task as its own page, in a dialog over the list, or in a panel beside it.": "Ouvrir une tâche dans sa propre page, dans une boîte de dialogue par-dessus la liste, ou dans un panneau à côté.",
		"Page leaves the listing for the task's own screen. Dialog opens the task over the listing you are on: the address changes so a reload or a copied link opens the full page, Back or Escape closes it, and saves made in it update the listing underneath. Panel opens it the same way beside the listing on a window at least 1100 pixels wide, with a divider you can drag, and as a dialog on a narrower one. A ctrl-click or a middle click still opens a new tab.": "Page quitte la liste pour l’écran propre de la tâche. Boîte de dialogue ouvre la tâche par-dessus la liste où vous êtes : l’adresse change pour qu’un rechargement ou un lien copié ouvre la page complète, Retour ou Échap la ferme, et les enregistrements faits dedans mettent à jour la liste en dessous. Panneau l’ouvre de la même façon à côté de la liste dans une fenêtre d’au moins 1100 pixels de large, avec un séparateur que vous pouvez faire glisser, et en boîte de dialogue dans une fenêtre plus étroite. Un ctrl-clic ou un clic du milieu ouvre toujours un nouvel onglet.",
		"Panel":             "Panneau",
		"Resize task panel": "Redimensionner le panneau de la tâche",
	})
}
