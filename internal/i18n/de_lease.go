// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// Lease length: the task's Lease property and the board's Claims
		// panel. A claim is an Übernahme and its lease an Übernahmefrist, as on the
		// Tenant screen.
		"Lease":                             "Übernahmefrist",
		"Lease length":                      "Übernahmedauer",
		"Longest lease":                     "Längste Dauer",
		"Inherited":                         "Geerbt",
		"Claims":                            "Übernahmen",
		"Save claim length":                 "Übernahmedauer speichern",
		"claim length saved":                "Übernahmedauer gespeichert",
		"Claims last %s, set on this task":  "Übernahmen dauern %s, an dieser Aufgabe festgelegt",
		"Claims last %s, from the project":  "Übernahmen dauern %s, vom Projekt",
		"Claims last %s, from the workflow": "Übernahmen dauern %s, vom Workflow",
		"Claims last %s, from the tenant":   "Übernahmen dauern %s, vom Mandanten",
		"Claims last %s, from the server configuration": "Übernahmen dauern %s, aus der Serverkonfiguration",
		"Claims last %s, the built-in default":          "Übernahmen dauern %s, der eingebaute Standard",
		"Claims last %s, capped by the project maximum": "Übernahmen dauern %s, begrenzt durch das Projektmaximum",
		"Claims last %s, capped by the tenant maximum":  "Übernahmen dauern %s, begrenzt durch das Mandantenmaximum",
		"Claims last %s, capped by the server maximum":  "Übernahmen dauern %s, begrenzt durch das Servermaximum",
		"How long a claim on this task holds before it expires, when the claim names no length of its own. Empty uses the project's, then an authored workflow's, then the tenant's. Never longer than the maximum.": "Wie lange eine Übernahme dieser Aufgabe gilt, bevor sie abläuft, wenn die Übernahme keine eigene Dauer nennt. Leer verwendet die des Projekts, dann die eines eigenen Workflows, dann die des Mandanten. Nie länger als das Maximum.",
		"How long a claim on this project's tasks holds when neither the claim nor the task says. It outranks the workflow's lease and the tenant's. Empty inherits the tenant's.":                                   "Wie lange eine Übernahme der Aufgaben dieses Projekts gilt, wenn weder die Übernahme noch die Aufgabe etwas angibt. Hat Vorrang vor der Dauer des Workflows und des Mandanten. Leer erbt die des Mandanten.",
		"No claim on this project holds longer, whatever it asks for. It can only be lower than the tenant's maximum.":                                                                                               "Keine Übernahme in diesem Projekt gilt länger, egal was sie verlangt. Der Wert kann nur unter dem Maximum des Mandanten liegen.",
	})
}
