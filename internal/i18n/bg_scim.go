// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The SCIM provisioning screen, its scope and its access channel.
func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"SCIM provisioning": "SCIM провизиране",
		"An identity provider such as Entra ID, Okta, Authentik or JumpCloud creates, updates, deactivates and removes this tenant's people and groups over SCIM 2.0. It signs in with a token of its own that reaches the SCIM endpoints and nothing else.": "Доставчик на идентичност като Entra ID, Okta, Authentik или JumpCloud създава, обновява, деактивира и премахва хората и групите на този тенант чрез SCIM 2.0. Той се удостоверява със собствен токен, който достига само до SCIM адресите.",
		"Provisioning":                "Провизиране",
		"Endpoint":                    "Адрес",
		"Provisioned users":           "Провизирани потребители",
		"Groups":                      "Групи",
		"Last change by the provider": "Последна промяна от доставчика",
		"Turning it off answers the provider with not found until it is turned on again. The token is kept.": "Изключването отговаря на доставчика с „не е намерено“, докато не бъде включено отново. Токенът се запазва.",
		"Turn SCIM off": "Изключи SCIM",
		"Turning it on the first time creates the scim agent and shows its token once.": "Първото включване създава агента scim и показва токена му веднъж.",
		"Turn SCIM on":       "Включи SCIM",
		"Provisioning token": "Токен за провизиране",
		"Issue a new token":  "Издай нов токен",
		"No provisioning token. Turn SCIM on, or issue one.": "Няма токен за провизиране. Включете SCIM или издайте токен.",
		"Issue a new provisioning token?":                    "Да се издаде ли нов токен за провизиране?",
		"The current provisioning token stops working at once, and the new one is shown once. Give it to the identity provider straight away.": "Текущият токен за провизиране спира да работи веднага, а новият се показва веднъж. Дайте го на доставчика на идентичност веднага.",
		"Group mappings": "Съответствия на групи",
		"Map a group":    "Свържи група с роля",
		"A person the provider pushed holds the highest role mapped to their groups. Leaving the last mapped group drops them to viewer.": "Човек, провизиран от доставчика, получава най-високата роля, свързана с групите му. Когато напусне последната свързана група, ролята му става viewer.",
		"Group":                                 "Група",
		"No groups mapped.":                     "Няма свързани групи.",
		"Group display name":                    "Име на групата",
		"Recent provider changes":               "Последни промени от доставчика",
		"Change":                                "Промяна",
		"Record":                                "Запис",
		"The provider has changed nothing yet.": "Доставчикът още не е променил нищо.",
		"All provider changes in the activity feed": "Всички промени от доставчика в активността",
		"SCIM provisioning is on":                   "SCIM провизирането е включено",
		"SCIM provisioning is off":                  "SCIM провизирането е изключено",
		"group mapped":                              "групата е свързана",
		"mapping removed":                           "съответствието е премахнато",
		"via SCIM":                                  "чрез SCIM",
		"via SCIM (token %s)":                       "чрез SCIM (токен %s)",
		"Provision users and groups from an identity provider over SCIM. It carries no other scope and works only on /scim/v2.": "Провизиране на потребители и групи от доставчик на идентичност чрез SCIM. Не носи друг обхват и работи само на /scim/v2.",
	})
}
