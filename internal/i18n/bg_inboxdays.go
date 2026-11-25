// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{
		"Summary":                                "Обобщение",
		"Due today":                              "Със срок днес",
		"Open":                                   "Отворени",
		"Done in 7 days":                         "Готови за 7 дни",
		"Kinds to show":                          "Видове за показване",
		"Nothing of these kinds.":                "Нищо от тези видове.",
		"Yesterday":                              "Вчера",
		"Earlier":                                "По-рано",
		"Stop mentions":                          "Спри споменаванията",
		"Stop assignments":                       "Спри възлаганията",
		"Stop status changes":                    "Спри смените на състояние",
		"Stop comments":                          "Спри коментарите",
		"Mentions no longer reach your inbox":    "Споменаванията вече не стигат до входящите ви",
		"Assignments no longer reach your inbox": "Възлаганията вече не стигат до входящите ви",
		"Status changes no longer reach your inbox":                                "Смените на състояние вече не стигат до входящите ви",
		"Comments no longer reach your inbox":                                      "Коментарите вече не стигат до входящите ви",
		"Items already in your inbox stay. Turn it back on under Settings, Inbox.": "Известията, които вече са във входящите, остават. Включете го отново от Настройки, Входящи.",
	})
}
