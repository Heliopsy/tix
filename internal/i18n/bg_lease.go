// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// Lease length: the task's Lease property and the board's Claims
		// panel. A claim is a поемане and its lease a резервация, as on the
		// Tenant screen.
		"Lease":                             "Резервация",
		"Lease length":                      "Продължителност на резервацията",
		"Longest lease":                     "Най-дълга резервация",
		"Inherited":                         "Наследена",
		"Claims":                            "Поемания",
		"Save claim length":                 "Запази продължителността",
		"claim length saved":                "продължителността на поеманията е запазена",
		"Claims last %s, set on this task":  "Поеманията траят %s, зададено на задачата",
		"Claims last %s, from the project":  "Поеманията траят %s, от проекта",
		"Claims last %s, from the workflow": "Поеманията траят %s, от работния процес",
		"Claims last %s, from the tenant":   "Поеманията траят %s, от тенанта",
		"Claims last %s, from the server configuration": "Поеманията траят %s, от настройките на инсталацията",
		"Claims last %s, the built-in default":          "Поеманията траят %s, вградената стойност",
		"Claims last %s, capped by the project maximum": "Поеманията траят %s, ограничени от максимума на проекта",
		"Claims last %s, capped by the tenant maximum":  "Поеманията траят %s, ограничени от максимума на тенанта",
		"Claims last %s, capped by the server maximum":  "Поеманията траят %s, ограничени от максимума на инсталацията",
		"How long a claim on this task holds before it expires, when the claim names no length of its own. Empty uses the project's, then an authored workflow's, then the tenant's. Never longer than the maximum.": "Колко дълго трае поемането на тази задача, преди резервацията да изтече, когато поемането не посочва своя продължителност. Празно поле използва тази на проекта, после на авторски работен процес, после на тенанта. Никога по-дълго от максимума.",
		"How long a claim on this project's tasks holds when neither the claim nor the task says. It outranks the workflow's lease and the tenant's. Empty inherits the tenant's.":                                   "Колко дълго трае поемането на задачите в този проект, когато нито поемането, нито задачата казват. Има предимство пред резервацията на работния процес и на тенанта. Празно поле наследява тази на тенанта.",
		"No claim on this project holds longer, whatever it asks for. It can only be lower than the tenant's maximum.":                                                                                               "Никое поемане в този проект не трае по-дълго, каквото и да поиска. Може да бъде само по-нисък от максимума на тенанта.",
	})
}
