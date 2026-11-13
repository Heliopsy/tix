// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// Task opens in's Panel position and the divider beside the panel.
func init() {
	register(German, map[string]string{
		"Open a task as its own page, in a dialog over the list, or in a panel beside it.": "Eine Aufgabe als eigene Seite, in einem Dialog über der Liste oder in einem Panel daneben öffnen.",
		"Page leaves the listing for the task's own screen. Dialog opens the task over the listing you are on: the address changes so a reload or a copied link opens the full page, Back or Escape closes it, and saves made in it update the listing underneath. Panel opens it the same way beside the listing on a window at least 1100 pixels wide, with a divider you can drag, and as a dialog on a narrower one. A ctrl-click or a middle click still opens a new tab.": "Seite verlässt die Liste für den eigenen Bildschirm der Aufgabe. Dialog öffnet die Aufgabe über der aktuellen Liste: Die Adresse ändert sich, sodass ein Neuladen oder ein kopierter Link die volle Seite öffnet, Zurück oder Escape schließt ihn, und Speichern darin aktualisiert die Liste darunter. Panel öffnet sie auf dieselbe Weise neben der Liste, wenn das Fenster mindestens 1100 Pixel breit ist, mit einem Trenner, den Sie ziehen können, und in einem schmaleren Fenster als Dialog. Strg-Klick oder Mittelklick öffnet weiterhin einen neuen Tab.",
		"Panel":             "Panel",
		"Resize task panel": "Aufgaben-Panel anpassen",
	})
}
