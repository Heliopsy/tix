// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// Task opens in's Panel position and the divider beside the panel.
func init() {
	register(Bulgarian, map[string]string{
		"Open a task as its own page, in a dialog over the list, or in a panel beside it.": "Отваряй задачата като отделна страница, в диалог върху списъка или в панел до него.",
		"Page leaves the listing for the task's own screen. Dialog opens the task over the listing you are on: the address changes so a reload or a copied link opens the full page, Back or Escape closes it, and saves made in it update the listing underneath. Panel opens it the same way beside the listing on a window at least 1100 pixels wide, with a divider you can drag, and as a dialog on a narrower one. A ctrl-click or a middle click still opens a new tab.": "Страница напуска списъка и отваря собствения екран на задачата. Диалог отваря задачата върху списъка, в който си: адресът се сменя, така че презареждане или копирана връзка отварят цялата страница, Назад или Escape го затварят, а промените, запазени в него, обновяват списъка отдолу. Панел я отваря по същия начин до списъка, когато прозорецът е широк поне 1100 пиксела, с разделител, който можеш да плъзгаш, а в по-тесен прозорец като диалог. Ctrl-щракване или щракване със средния бутон все така отваря нов раздел.",
		"Panel":             "Панел",
		"Resize task panel": "Преоразмери панела на задачата",
	})
}
