// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The SCIM provisioning screen, its scope and its access channel.
func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"SCIM provisioning": "Provisionnement SCIM",
		"An identity provider such as Entra ID, Okta, Authentik or JumpCloud creates, updates, deactivates and removes this tenant's people and groups over SCIM 2.0. It signs in with a token of its own that reaches the SCIM endpoints and nothing else.": "Un fournisseur d'identité comme Entra ID, Okta, Authentik ou JumpCloud crée, modifie, désactive et retire les personnes et les groupes de ce tenant via SCIM 2.0. Il s'authentifie avec son propre jeton, qui n'atteint que les points d'accès SCIM.",
		"Provisioning":                "Provisionnement",
		"Endpoint":                    "Point d'accès",
		"Provisioned users":           "Utilisateurs provisionnés",
		"Groups":                      "Groupes",
		"Last change by the provider": "Dernière modification par le fournisseur",
		"Turning it off answers the provider with not found until it is turned on again. The token is kept.": "Désactivé, tix répond au fournisseur « introuvable » jusqu'à sa réactivation. Le jeton est conservé.",
		"Turn SCIM off": "Désactiver SCIM",
		"Turning it on the first time creates the scim agent and shows its token once.": "La première activation crée l'agent scim et affiche son jeton une seule fois.",
		"Turn SCIM on":       "Activer SCIM",
		"Provisioning token": "Jeton de provisionnement",
		"Issue a new token":  "Émettre un nouveau jeton",
		"No provisioning token. Turn SCIM on, or issue one.": "Aucun jeton de provisionnement. Activez SCIM ou émettez-en un.",
		"Issue a new provisioning token?":                    "Émettre un nouveau jeton de provisionnement ?",
		"The current provisioning token stops working at once, and the new one is shown once. Give it to the identity provider straight away.": "Le jeton de provisionnement actuel cesse de fonctionner immédiatement, et le nouveau est affiché une seule fois. Donnez-le tout de suite au fournisseur d'identité.",
		"Group mappings": "Correspondances de groupes",
		"Map a group":    "Associer un groupe",
		"A person the provider pushed holds the highest role mapped to their groups. Leaving the last mapped group drops them to viewer.": "Une personne transmise par le fournisseur reçoit le rôle le plus élevé associé à ses groupes. En quittant son dernier groupe associé, elle redevient viewer.",
		"Group":                                 "Groupe",
		"No groups mapped.":                     "Aucun groupe associé.",
		"Group display name":                    "Nom affiché du groupe",
		"Recent provider changes":               "Modifications récentes du fournisseur",
		"Change":                                "Modification",
		"Record":                                "Enregistrement",
		"The provider has changed nothing yet.": "Le fournisseur n'a encore rien modifié.",
		"All provider changes in the activity feed": "Toutes les modifications du fournisseur dans l'activité",
		"SCIM provisioning is on":                   "Le provisionnement SCIM est activé",
		"SCIM provisioning is off":                  "Le provisionnement SCIM est désactivé",
		"group mapped":                              "groupe associé",
		"mapping removed":                           "correspondance retirée",
		"via SCIM":                                  "via SCIM",
		"via SCIM (token %s)":                       "via SCIM (jeton %s)",
		"Provision users and groups from an identity provider over SCIM. It carries no other scope and works only on /scim/v2.": "Provisionner les utilisateurs et les groupes depuis un fournisseur d'identité via SCIM. Ne porte aucune autre portée et ne fonctionne que sur /scim/v2.",
	})
}
