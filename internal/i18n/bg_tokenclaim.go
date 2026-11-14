// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The "require a claim" setting on the issue-a-token form and its mark on
// the token listing.
func init() {
	register(Bulgarian, map[string]string{
		"Require a claim before changing a task": "Изисквай поемане преди промяна на задача",
		"claim required":                         "изисква поемане",
		"Whatever holds this token may move, edit, tag, move to another project or delete a task only while it holds a live claim on it, so every change it makes is tied to work it took. Claiming, creating tasks, commenting and reading stay open. Tick it for an AI agent; leave it clear for a person's script. Rotating the token keeps the setting.": "Който държи този токен, може да мести, редактира, етикетира, прехвърля в друг проект или изтрива задача само докато я е поел с активно поемане, така че всяка негова промяна е свързана с работа, която е поел. Поемането, създаването на задачи, коментирането и четенето остават отворени. Отметнете го за AI агент; оставете го празно за скрипт на човек. Ротирането на токена запазва настройката.",
	})
}
