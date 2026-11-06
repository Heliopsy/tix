// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// Date preferences kept with the account, the tenant's default under
		// them and the server's under that.
		"Kept with your account, so every browser and terminal you sign in from shows dates this way. Inherit follows your tenant's default, which follows the server's.":                                                                                                              "Пази се в профила ви, така че всеки браузър и терминал, от които влизате, показва датите така. „Наследяване“ следва настройката по подразбиране на тенанта, а тя следва тази на сървъра.",
		"The zone every timestamp on every screen is written in. Inherit uses your tenant's default, and the server's when the tenant has none.":                                                                                                                                       "Зоната, в която се изписва всеки час на всеки екран. „Наследяване“ използва настройката по подразбиране на тенанта, а когато няма такава, тази на сървъра.",
		"How a timestamp is written, and how a date you type is read: 06.11.2026 is the 6th of November under Dotted and Day first, and 11/06/2026 is under US. 2026-11-06 is read under every format. Inherit uses your tenant's default, and the server's when the tenant has none.": "Как се изписва моментът и как се чете дата, която въвеждате: 06.11.2026 е 6 ноември при „Dotted“ и „Day first“, а 11/06/2026 е 6 ноември при „US“. 2026-11-06 се чете при всеки формат. „Наследяване“ използва настройката по подразбиране на тенанта, а когато няма такава, тази на сървъра.",
		"In use:":                "В сила:",
		"Tenant default in use:": "В сила по подразбиране за тенанта:",
		"from server default":    "от настройката на сървъра",
		"from tenant default":    "от настройката на тенанта",
		"from your choice":       "по ваш избор",
		"server default":         "настройката на сървъра",
		"tenant default":         "настройката на тенанта",
		"your choice":            "ваш избор",
		"Inherit":                "Наследяване",
		"the server's own zone":  "собствената зона на сървъра",
		"How this tenant's people read timestamps unless they chose their own under Settings. Inherit uses the server's output.time_format.":      "Как хората в този тенант четат моментите, освен ако не са избрали свой формат в Настройки. „Наследяване“ използва output.time_format на сървъра.",
		"The zone this tenant's people read timestamps in unless they chose their own under Settings. Inherit uses the server's output.timezone.": "Зоната, в която хората в този тенант четат моментите, освен ако не са избрали своя в Настройки. „Наследяване“ използва output.timezone на сървъра.",
		"Not yet": "Все още не",
	})
}
