// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The SAML settings and addresses on the single sign-on screen.
func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"%d signing certificates": "%d сертификата за подпис",
		"A provider lets people sign in to this tenant with an account they already have: an OpenID Connect or SAML identity provider such as Keycloak, Authentik, Entra ID, Google or Okta, or GitHub. Each enabled provider is a Sign in with button on the sign-in screen.": "Доставчикът позволява на хората да влизат в този наемател с акаунт, който вече имат: доставчик на идентичност по OpenID Connect или SAML като Keycloak, Authentik, Entra ID, Google или Okta, или GitHub. Всеки включен доставчик е бутон „Вход с“ на екрана за вход.",
		"ACS URL": "ACS URL",
		"Accept sign-ins started at the identity provider (SAML)":                                                                                   "Приемай влизания, започнати при доставчика на идентичност (SAML)",
		"For a SAML provider these name the assertion's attributes, by Name or FriendlyName; empty reads the names common identity providers send.": "За SAML доставчик тези полета назовават атрибутите на твърдението, по Name или FriendlyName; празно чете имената, които обичайните доставчици изпращат.",
		"IdP entity ID (SAML)":                  "Entity ID на IdP (SAML)",
		"IdP metadata URL (SAML)":               "URL на метаданните на IdP (SAML)",
		"IdP sign-in URL, HTTP-Redirect (SAML)": "URL за вход на IdP, HTTP-Redirect (SAML)",
		"IdP signing certificates, PEM (SAML)":  "Сертификати за подпис на IdP, PEM (SAML)",
		"Metadata":                              "Метаданни",
		"NameID format to request (SAML)":       "Формат на NameID за заявяване (SAML)",
		"One or more certificates the identity provider signs with. Add the next one before the IdP rolls its key over, and remove the old one after.": "Един или повече сертификата, с които доставчикът на идентичност подписва. Добавете следващия, преди IdP да смени ключа си, и премахнете стария след това.",
		"Or paste the IdP metadata XML (SAML)":                                           "Или поставете XML метаданните на IdP (SAML)",
		"Read the IdP metadata again on save":                                            "Прочети метаданните на IdP отново при запазване",
		"Register this address as the redirect URL with every OIDC and GitHub provider:": "Регистрирайте този адрес като URL за пренасочване при всеки OIDC и GitHub доставчик:",
		"SP entity ID":                 "Entity ID на SP",
		"SP entity ID override (SAML)": "Собствен Entity ID на SP (SAML)",
		"Second factor amr, acr or SAML AuthnContextClassRef values, comma separated": "Стойности amr, acr или SAML AuthnContextClassRef за втори фактор, разделени със запетая",
		"Skip the tenant's second factor when the provider reports one (OIDC, SAML)":  "Пропусни втория фактор на наемателя, когато доставчикът съобщи за такъв (OIDC, SAML)",
		"The identity provider's SAML metadata address. Saving reads its entity ID, sign-in URL and signing certificates, and the server reads it again on a schedule to pick up a new certificate; anything you fill in below wins over it.": "Адресът на SAML метаданните на доставчика на идентичност. При запазване се четат неговият entity ID, URL за вход и сертификати за подпис, а сървърът го чете отново по график, за да вземе нов сертификат; попълненото по-долу има предимство.",
		"accepts IdP-initiated sign-in": "приема влизания, започнати от IdP",
		"the metadata address":          "адресът на метаданните",
	})
}
