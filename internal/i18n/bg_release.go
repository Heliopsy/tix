// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{
		"Update available": "Има обновление",
		"v%s is available": "Налична е v%s",
		"This installation runs v%s. The commands below install the newer release.": "Тази инсталация работи с v%s. Командите по-долу инсталират по-новата версия.",
		"Up to date":                              "Актуална",
		"This installation is up to date":         "Тази инсталация е актуална",
		"v%s is the newest published release.":    "v%s е най-новата публикувана версия.",
		"Not comparable":                          "Несравнима",
		"This build's version cannot be compared": "Версията на тази компилация не може да се сравни",
		"v%s is the newest published release, and v%s is not a version number it can be ordered against.": "v%s е най-новата публикувана версия, а v%s не е номер на версия, с който да се сравни.",
		"Check failed":                          "Проверката не успя",
		"The release feed could not be reached": "Каналът за версии не е достъпен",
		"That is the ordinary case for an isolated network. The server tries again within six hours.": "Това е обичайно за изолирана мрежа. Сървърът ще опита отново до шест часа.",
		"Not checked":     "Непроверено",
		"Not checked yet": "Още не е проверено",
		"The server asks the release feed in the background. Reload in a moment.": "Сървърът проверява канала за версии във фонов режим. Презаредете след малко.",
		"How to upgrade": "Как да обновите",
		"Use the way this server was installed. Nothing here restarts a running tix serve: restart it once the new version is in place.": "Използвайте начина, по който е инсталиран този сървър. Нищо тук не рестартира работещ tix serve: рестартирайте го, след като новата версия е на място.",
		"Release binary": "Двоичен файл от версия",
		"Installed with install.sh or unpacked from a release archive. Checks the download against its published checksum before replacing the binary.": "Инсталиран с install.sh или разархивиран от архив на версия. Сверява изтеглянето с публикуваната контролна сума, преди да замени двоичния файл.",
		"Container image": "Образ за контейнер",
		"Pull the new tag, then recreate the container on the same volume.": "Изтеглете новия таг и пресъздайте контейнера върху същия том.",
		"Go toolchain": "Инструменти на Go",
		"Installed with go install. tix update refuses this copy, because the toolchain owns it.": "Инсталиран с go install. tix update отказва да замени това копие, защото то принадлежи на инструментите на Go.",
		"Installed from a package manager? Upgrade it the way you installed it.":                  "Инсталирали сте от пакетен мениджър? Обновете по същия начин, по който сте инсталирали.",
		"What changed in v%s": "Какво се промени във v%s",
	})
}
