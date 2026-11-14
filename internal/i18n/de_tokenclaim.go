// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The "require a claim" setting on the issue-a-token form and its mark on
// the token listing.
func init() {
	register(German, map[string]string{
		"Require a claim before changing a task": "Vor dem Ändern einer Aufgabe eine Übernahme verlangen",
		"claim required":                         "Übernahme nötig",
		"Whatever holds this token may move, edit, tag, move to another project or delete a task only while it holds a live claim on it, so every change it makes is tied to work it took. Claiming, creating tasks, commenting and reading stay open. Tick it for an AI agent; leave it clear for a person's script. Rotating the token keeps the setting.": "Wer diesen Token hält, kann eine Aufgabe nur verschieben, bearbeiten, verschlagworten, in ein anderes Projekt verschieben oder löschen, solange er eine laufende Übernahme darauf hält, so gehört jede Änderung zu Arbeit, die er übernommen hat. Übernehmen, Aufgaben anlegen, Kommentieren und Lesen bleiben offen. Für einen KI-Agenten ankreuzen, für das Skript einer Person leer lassen. Das Rotieren des Tokens behält die Einstellung bei.",
	})
}
