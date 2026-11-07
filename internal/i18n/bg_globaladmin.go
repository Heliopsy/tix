// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// The global admin area: every tenant, the global admins, and the
		// users of any tenant.
		"Global admin":  "Глобален администратор",
		"Global admins": "Глобални администратори",
		"Every tenant of this installation, and who may administer all of them.": "Всички тенанти на тази инсталация и кой може да администрира всички тях.",
		"Tenants":           "Тенанти",
		"No tenants yet":    "Още няма тенанти",
		"Create a tenant":   "Създаване на тенант",
		"Create tenant":     "Създай тенант",
		"First admins":      "Първи администратори",
		"Rename %s":         "Преименуване на %s",
		"Users of %s":       "Потребители на %s",
		"Users of a tenant": "Потребители на тенант",
		"One email per line. Each becomes an admin of the new tenant. A new address gets a password generated and shown to you once; an address that already has an account keeps its own password and second factor.": "По един имейл на ред. Всеки става администратор на новия тенант. Нов адрес получава генерирана парола, която ви се показва веднъж; адрес, който вече има акаунт, запазва своята парола и втори фактор.",
		"Copy these passwords now": "Копирайте тези пароли сега",
		"They are shown once and cannot be retrieved again. If one is lost, set a new password from the tenant's users.":                                          "Показват се веднъж и не могат да бъдат получени отново. Ако някоя бъде загубена, задайте нова парола от потребителите на тенанта.",
		"Its people can no longer sign in, its data stops being reachable and its domains stop resolving. Nothing is destroyed, but nothing here brings it back.": "Хората в него вече не могат да влизат, данните му стават недостъпни и домейните му спират да се разрешават. Нищо не се унищожава, но нищо тук не го връща обратно.",
		"Grant global admin":           "Даване на глобален администратор",
		"Revoke global admin from %s":  "Отнемане на глобален администратор от %s",
		"Revoke global admin from %s?": "Да се отнеме ли глобален администратор от %s?",
		"A global admin creates tenants and administers every tenant here. Name a user who already exists, in any tenant.":                     "Глобалният администратор създава тенанти и администрира всеки тенант тук. Посочете потребител, който вече съществува, в който и да е тенант.",
		"They keep their own tenant and their role in it, and stop administering the others at once. The last global admin cannot be revoked.": "Запазват своя тенант и ролята си в него и веднага спират да администрират останалите. Последният глобален администратор не може да бъде отнет.",
		"tenant created":       "тенантът е създаден",
		"tenant renamed":       "тенантът е преименуван",
		"tenant deleted":       "тенантът е изтрит",
		"global admin granted": "глобалният администратор е даден",
		"global admin revoked": "глобалният администратор е отнет",
	})
}
