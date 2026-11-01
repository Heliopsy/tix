// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// Changing a project's key.
		"The key starts every task reference, such as INFRA-8, and the project's address. Changing it changes every reference; old links, references and filters keep working and lead to the new ones. You are asked to confirm before it changes.": "Ключът стои в началото на всяка препратка към задача, например INFRA-8, и в адреса на проекта. Смяната му променя всички препратки; старите връзки, препратки и филтри продължават да работят и водят към новите. Преди смяната ще бъдете помолени да потвърдите.",
		"Change the key of %s?": "Да се смени ли ключът на %s?",
		"The key changes from %s to %s, and every task reference changes with it:":                                                      "Ключът се сменя от %s на %s и всяка препратка към задача се променя с него:",
		"Old links, references and filters that use %s keep working and lead here. No other project can take %s while this one exists.": "Старите връзки, препратки и филтри с %s продължават да работят и водят тук. Никой друг проект не може да вземе %s, докато този съществува.",
		"Change the key to %s": "Смени ключа на %s",
		"A browser cannot carry a file to this screen, so choose the image again.": "Браузърът не може да пренесе файл до този екран, затова изберете изображението отново.",
	})
}
