// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The "require a claim" setting on the issue-a-token form and its mark on
// the token listing.
func init() {
	register(French, map[string]string{
		"Require a claim before changing a task": "Exiger une prise en charge avant de modifier une tâche",
		"claim required":                         "prise en charge exigée",
		"Whatever holds this token may move, edit, tag, move to another project or delete a task only while it holds a live claim on it, so every change it makes is tied to work it took. Claiming, creating tasks, commenting and reading stay open. Tick it for an AI agent; leave it clear for a person's script. Rotating the token keeps the setting.": "Ce qui détient ce jeton ne peut déplacer, modifier, étiqueter, changer de projet ou supprimer une tâche que tant qu’il en détient une prise en charge active, de sorte que chaque changement est lié à un travail qu’il a pris. Prendre en charge, créer des tâches, commenter et lire restent ouverts. Cochez-le pour un agent IA ; laissez-le vide pour le script d’une personne. Faire tourner le jeton conserve le réglage.",
	})
}
