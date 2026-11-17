// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The SAML settings and addresses on the single sign-on screen.
func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"%d signing certificates": "%d certificats de signature",
		"A provider lets people sign in to this tenant with an account they already have: an OpenID Connect or SAML identity provider such as Keycloak, Authentik, Entra ID, Google or Okta, or GitHub. Each enabled provider is a Sign in with button on the sign-in screen.": "Un fournisseur permet de se connecter à ce locataire avec un compte existant : un fournisseur d'identité OpenID Connect ou SAML comme Keycloak, Authentik, Entra ID, Google ou Okta, ou GitHub. Chaque fournisseur actif est un bouton « Se connecter avec » sur l'écran de connexion.",
		"ACS URL": "URL ACS",
		"Accept sign-ins started at the identity provider (SAML)":                                                                                   "Accepter les connexions lancées chez le fournisseur d'identité (SAML)",
		"For a SAML provider these name the assertion's attributes, by Name or FriendlyName; empty reads the names common identity providers send.": "Pour un fournisseur SAML, ces champs nomment les attributs de l'assertion, par Name ou FriendlyName ; vide lit les noms qu'envoient les fournisseurs d'identité courants.",
		"IdP entity ID (SAML)":                  "Entity ID de l'IdP (SAML)",
		"IdP metadata URL (SAML)":               "URL des métadonnées de l'IdP (SAML)",
		"IdP sign-in URL, HTTP-Redirect (SAML)": "URL de connexion de l'IdP, HTTP-Redirect (SAML)",
		"IdP signing certificates, PEM (SAML)":  "Certificats de signature de l'IdP, PEM (SAML)",
		"Metadata":                              "Métadonnées",
		"NameID format to request (SAML)":       "Format de NameID à demander (SAML)",
		"One or more certificates the identity provider signs with. Add the next one before the IdP rolls its key over, and remove the old one after.": "Un ou plusieurs certificats avec lesquels le fournisseur d'identité signe. Ajoutez le suivant avant que l'IdP change de clé, et retirez l'ancien ensuite.",
		"Or paste the IdP metadata XML (SAML)":                                           "Ou collez le XML des métadonnées de l'IdP (SAML)",
		"Read the IdP metadata again on save":                                            "Relire les métadonnées de l'IdP à l'enregistrement",
		"Register this address as the redirect URL with every OIDC and GitHub provider:": "Enregistrez cette adresse comme URL de redirection auprès de chaque fournisseur OIDC et GitHub :",
		"SP entity ID":                 "Entity ID du SP",
		"SP entity ID override (SAML)": "Entity ID du SP personnalisé (SAML)",
		"Second factor amr, acr or SAML AuthnContextClassRef values, comma separated":                                                                               "Valeurs amr, acr ou AuthnContextClassRef SAML du second facteur, séparées par des virgules",
		"Skip the tenant's second factor when the provider reports one (OIDC, SAML)":                                                                                "Ignorer le second facteur du locataire quand le fournisseur en signale un (OIDC, SAML)",
		"The identity provider's SAML metadata address. Saving reads its entity ID, sign-in URL and signing certificates; anything you fill in below wins over it.": "L'adresse des métadonnées SAML du fournisseur d'identité. L'enregistrement en lit l'entity ID, l'URL de connexion et les certificats de signature ; ce que vous saisissez ci-dessous l'emporte.",
		"accepts IdP-initiated sign-in": "accepte les connexions lancées par l'IdP",
		"the metadata address":          "l'adresse des métadonnées",
	})
}
