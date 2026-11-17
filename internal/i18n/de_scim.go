// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The SCIM provisioning screen, its scope and its access channel.
func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"SCIM provisioning": "SCIM-Provisionierung",
		"An identity provider such as Entra ID, Okta, Authentik or JumpCloud creates, updates, deactivates and removes this tenant's people and groups over SCIM 2.0. It signs in with a token of its own that reaches the SCIM endpoints and nothing else.": "Ein Identitätsanbieter wie Entra ID, Okta, Authentik oder JumpCloud legt die Personen und Gruppen dieses Mandanten über SCIM 2.0 an, ändert, deaktiviert und entfernt sie. Er meldet sich mit einem eigenen Token an, das nur die SCIM-Endpunkte erreicht.",
		"Provisioning":                "Provisionierung",
		"Endpoint":                    "Endpunkt",
		"Provisioned users":           "Provisionierte Benutzer",
		"Groups":                      "Gruppen",
		"Last change by the provider": "Letzte Änderung durch den Anbieter",
		"Turning it off answers the provider with not found until it is turned on again. The token is kept.": "Ausgeschaltet antwortet tix dem Anbieter mit „nicht gefunden“, bis es wieder eingeschaltet wird. Das Token bleibt erhalten.",
		"Turn SCIM off": "SCIM ausschalten",
		"Turning it on the first time creates the scim agent and shows its token once.": "Beim ersten Einschalten wird der Agent scim angelegt und sein Token einmal angezeigt.",
		"Turn SCIM on":       "SCIM einschalten",
		"Provisioning token": "Provisionierungs-Token",
		"Issue a new token":  "Neues Token ausstellen",
		"No provisioning token. Turn SCIM on, or issue one.": "Kein Provisionierungs-Token. Schalten Sie SCIM ein oder stellen Sie eines aus.",
		"Issue a new provisioning token?":                    "Neues Provisionierungs-Token ausstellen?",
		"The current provisioning token stops working at once, and the new one is shown once. Give it to the identity provider straight away.": "Das aktuelle Provisionierungs-Token funktioniert sofort nicht mehr, und das neue wird einmal angezeigt. Geben Sie es gleich an den Identitätsanbieter weiter.",
		"Group mappings": "Gruppenzuordnungen",
		"Map a group":    "Gruppe zuordnen",
		"A person the provider pushed holds the highest role mapped to their groups. Leaving the last mapped group drops them to viewer.": "Eine vom Anbieter übertragene Person erhält die höchste Rolle, die ihren Gruppen zugeordnet ist. Verlässt sie die letzte zugeordnete Gruppe, wird sie viewer.",
		"Group":                                 "Gruppe",
		"No groups mapped.":                     "Keine Gruppen zugeordnet.",
		"Group display name":                    "Anzeigename der Gruppe",
		"Recent provider changes":               "Letzte Änderungen des Anbieters",
		"Change":                                "Änderung",
		"Record":                                "Datensatz",
		"The provider has changed nothing yet.": "Der Anbieter hat noch nichts geändert.",
		"All provider changes in the activity feed": "Alle Änderungen des Anbieters in der Aktivität",
		"SCIM provisioning is on":                   "SCIM-Provisionierung ist eingeschaltet",
		"SCIM provisioning is off":                  "SCIM-Provisionierung ist ausgeschaltet",
		"group mapped":                              "Gruppe zugeordnet",
		"mapping removed":                           "Zuordnung entfernt",
		"via SCIM":                                  "über SCIM",
		"via SCIM (token %s)":                       "über SCIM (Token %s)",
		"Provision users and groups from an identity provider over SCIM. It carries no other scope and works only on /scim/v2.": "Benutzer und Gruppen von einem Identitätsanbieter über SCIM provisionieren. Trägt keinen anderen Bereich und funktioniert nur unter /scim/v2.",
	})
}
