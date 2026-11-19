// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The SAML settings and addresses on the single sign-on screen.
func init() {
	register(German, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"%d signing certificates": "%d Signaturzertifikate",
		"A provider lets people sign in to this tenant with an account they already have: an OpenID Connect or SAML identity provider such as Keycloak, Authentik, Entra ID, Google or Okta, or GitHub. Each enabled provider is a Sign in with button on the sign-in screen.": "Ein Anbieter lässt Personen sich mit einem Konto anmelden, das sie schon haben: einem OpenID-Connect- oder SAML-Identitätsanbieter wie Keycloak, Authentik, Entra ID, Google oder Okta, oder GitHub. Jeder aktive Anbieter ist eine Schaltfläche „Anmelden mit“ auf dem Anmeldebildschirm.",
		"ACS URL": "ACS-URL",
		"Accept sign-ins started at the identity provider (SAML)":                                                                                   "Anmeldungen annehmen, die beim Identitätsanbieter beginnen (SAML)",
		"For a SAML provider these name the assertion's attributes, by Name or FriendlyName; empty reads the names common identity providers send.": "Bei einem SAML-Anbieter benennen diese Felder die Attribute der Assertion, nach Name oder FriendlyName; leer liest die Namen, die gängige Identitätsanbieter senden.",
		"IdP entity ID (SAML)":                  "Entity-ID des IdP (SAML)",
		"IdP metadata URL (SAML)":               "Metadaten-URL des IdP (SAML)",
		"IdP sign-in URL, HTTP-Redirect (SAML)": "Anmelde-URL des IdP, HTTP-Redirect (SAML)",
		"IdP signing certificates, PEM (SAML)":  "Signaturzertifikate des IdP, PEM (SAML)",
		"Metadata":                              "Metadaten",
		"NameID format to request (SAML)":       "Anzuforderndes NameID-Format (SAML)",
		"One or more certificates the identity provider signs with. Add the next one before the IdP rolls its key over, and remove the old one after.": "Ein oder mehrere Zertifikate, mit denen der Identitätsanbieter signiert. Das nächste vor dem Schlüsselwechsel des IdP hinzufügen und das alte danach entfernen.",
		"Or paste the IdP metadata XML (SAML)":                                           "Oder die Metadaten-XML des IdP einfügen (SAML)",
		"Read the IdP metadata again on save":                                            "IdP-Metadaten beim Speichern neu lesen",
		"Register this address as the redirect URL with every OIDC and GitHub provider:": "Diese Adresse bei jedem OIDC- und GitHub-Anbieter als Weiterleitungs-URL eintragen:",
		"SP entity ID":                 "Entity-ID des SP",
		"SP entity ID override (SAML)": "Eigene Entity-ID des SP (SAML)",
		"Second factor amr, acr or SAML AuthnContextClassRef values, comma separated": "amr-, acr- oder SAML-AuthnContextClassRef-Werte für den zweiten Faktor, durch Kommas getrennt",
		"Skip the tenant's second factor when the provider reports one (OIDC, SAML)":  "Den zweiten Faktor des Mandanten überspringen, wenn der Anbieter einen meldet (OIDC, SAML)",
		"The identity provider's SAML metadata address. Saving reads its entity ID, sign-in URL and signing certificates, and the server reads it again on a schedule to pick up a new certificate; anything you fill in below wins over it.": "Die SAML-Metadatenadresse des Identitätsanbieters. Beim Speichern werden Entity-ID, Anmelde-URL und Signaturzertifikate daraus gelesen, und der Server liest sie regelmäßig neu, um ein neues Zertifikat zu übernehmen; was Sie unten eintragen, hat Vorrang.",
		"accepts IdP-initiated sign-in": "nimmt beim IdP begonnene Anmeldungen an",
		"the metadata address":          "die Metadatenadresse",
	})
}
