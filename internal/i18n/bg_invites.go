// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// Creating a user: a password or an invite link.
		"How they sign in":       "Как ще влизат",
		"Set a password":         "Задай парола",
		"Send an invite link":    "Изпрати покана с връзка",
		"Create invite link":     "Създай връзка за покана",
		"Link expires after":     "Връзката изтича след",
		"1 day":                  "1 ден",
		"3 days":                 "3 дни",
		"Invite links":           "Връзки за покана",
		"Invite link":            "Връзка за покана",
		"Send this link to them": "Изпратете им тази връзка",
		"Require a password change at first sign-in":                                                 "Изисквай смяна на паролата при първото влизане",
		"Ask to change the password at next sign-in":                                                 "Поискай смяна на паролата при следващото влизане",
		"You get a link to send them. It works once, and they choose their own password with it.":    "Получавате връзка, която да им изпратите. Тя работи веднъж и с нея те избират собствена парола.",
		"It works once and expires on %s. tix does not send email, and the link is not shown again.": "Работи веднъж и изтича на %s. tix не изпраща имейли и връзката не се показва отново.",
		"An invite link lets the person choose their own password. It works once and expires; tix sends no email, so you pass the link on yourself. A password you type yourself can be made to change at their first sign-in.": "Връзката за покана позволява на човека сам да избере парола. Тя работи веднъж и изтича; tix не изпраща имейли, затова вие предавате връзката. Парола, която въвеждате вие, може да се наложи да бъде сменена при първото влизане.",
		"Their next sign-in with a password stops at a page asking for a new one, and goes no further until they choose it. Sessions already open and API tokens are not affected.":                                             "Следващото им влизане с парола спира на страница, която иска нова, и не продължава, докато не я изберат. Вече отворените сесии и API токените не се засягат.",

		// A change the account's other tenants keep from a tenant admin.
		"%s also belongs to another tenant, so only they or a global admin can ask them to change their password. Leave the box as it was to save the rest.": "%s е член и на друг тенант, затова само те или глобален администратор могат да поискат смяна на паролата им. Оставете полето както беше, за да запазите останалото.",
		"%s also belongs to another tenant, so only they or a global admin can clear their second factor.":                                                   "%s е член и на друг тенант, затова само те или глобален администратор могат да изчистят втория им фактор.",

		// Pending invites.
		"Pending invites": "Чакащи покани",
		"Links issued and not used yet. Re-issuing stops the old link and gives you a new one to send; revoking stops it and keeps the account, without a password.": "Издадени и още неизползвани връзки. Повторното издаване спира старата връзка и ви дава нова за изпращане; отмяната я спира и запазва акаунта, без парола.",
		"invited":                    "поканен",
		"invited by %s":              "поканен от %s",
		"expires %s":                 "изтича %s",
		"Re-issue":                   "Издай отново",
		"Re-issue the invite for %s": "Издай отново поканата за %s",
		"Revoke the invite for %s":   "Отмени поканата за %s",
		"Revoke the invite for %s?":  "Да се отмени ли поканата за %s?",
		"The link stops working at once. The account stays, without a password, until you delete it or invite it again.": "Връзката спира да работи веднага. Акаунтът остава, без парола, докато не го изтриете или поканите отново.",
		"invite created":   "поканата е създадена",
		"invite re-issued": "поканата е издадена отново",
		"invite revoked":   "поканата е отменена",

		// The invite page.
		"Accept your invite": "Приемете поканата",
		"Join %s":            "Присъединете се към %s",
		"Choose a password to finish setting up your account. This link works once.":    "Изберете парола, за да завършите акаунта си. Тази връзка работи веднъж.",
		"You already have an account. Enter its password to accept; it stays as it is.": "Вече имате акаунт. Въведете паролата му, за да приемете; тя остава същата.",
		"Signing in as":                "Влизате като",
		"Current password":             "Текуща парола",
		"Password again":               "Паролата отново",
		"At least 12 characters.":      "Поне 12 знака.",
		"Accept and sign in":           "Приеми и влез",
		"Code from your authenticator": "Код от приложението за удостоверяване",
		"Your account has a second factor, so joining another tenant asks for a code from it too.": "Акаунтът ви има втори фактор, затова присъединяването към друг тенант изисква и код от него.",
		"This link does not work": "Тази връзка не работи",
		"This invite link is not valid. It may have expired, been used already or been revoked. Ask whoever invited you for a new one.": "Тази връзка за покана не е валидна. Може да е изтекла, вече да е използвана или да е отменена. Поискайте нова от човека, който ви е поканил.",
		"Go to sign in": "Към влизането",

		// The password change a sign-in can be stopped at.
		"Choose a new password": "Изберете нова парола",
		"Your administrator asked you to replace your password before you go on.": "Администраторът ви поиска да смените паролата си, преди да продължите.",
		"New password":       "Нова парола",
		"New password again": "Новата парола отново",
		"At least 12 characters, and not the one you have now.": "Поне 12 знака и различна от сегашната.",
		"Save and sign in": "Запази и влез",
		"Back to sign in":  "Обратно към влизането",
	})
}
