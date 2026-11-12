// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{
		"Each setting says where it is kept: on this device, in this browser's cookies or storage; synced to your account, so it follows you everywhere you sign in; or for the whole tenant.": "Всяка настройка казва къде се пази: на това устройство, в бисквитките или хранилището на този браузър; синхронизирана с профила ви, така че ви следва навсякъде, където влезете; или за цялата организация.",
		"Search settings":                                 "Търсене в настройките",
		"No setting matches “%s”":                         "Няма настройка, която да отговаря на „%s“",
		"Try another word, or clear the search.":          "Опитайте с друга дума или изчистете търсенето.",
		"On this device":                                  "На това устройство",
		"Synced to your account":                          "Синхронизирано с профила ви",
		"Whole tenant":                                    "Цялата организация",
		"Settings here apply to everyone in this tenant.": "Настройките тук важат за всички в тази организация.",
		"Start screen":                                    "Начален екран",
		"Apply start screen":                              "Приложи началния екран",
		"Where tix opens in this browser.":                "Къде се отваря tix в този браузър.",
		"The screen this browser opens when you go to the address of tix itself, and after you sign in without following a link.": "Екранът, който този браузър отваря, когато отидете на адреса на самия tix, и след влизане, когато не сте следвали връзка.",
		"Reduce motion": "Без анимации",
		"Keep tix still even when the system allows animation.": "tix остава неподвижен дори когато системата позволява анимации.",
		"Turns off animation, sliding and smooth scrolling in this browser. Your system's own reduced motion setting already does this; this keeps tix still even when the system allows animation.": "Изключва анимациите, плъзгането и плавното превъртане в този браузър. Системната настройка за намалено движение вече прави това; тази пази tix неподвижен дори когато системата позволява анимации.",
		"Notify me about new inbox items":                "Известявай ме за нови входящи",
		"Browser notifications while a tix tab is open.": "Известия от браузъра, докато има отворен раздел с tix.",
		"A notification from your browser for each new inbox item, while a tix tab is open and you are looking at something else. Turning it on asks the browser for permission. A closed tab cannot be told: channels below deliver everywhere.": "Известие от браузъра за всяко ново входящо, докато има отворен раздел с tix и гледате нещо друго. Включването иска разрешение от браузъра. Затворен раздел не може да бъде известен: каналите по-долу доставят навсякъде.",
		"Hide what they say":               "Скрий съдържанието",
		"Say only that something arrived.": "Казва само, че нещо е пристигнало.",
		"Notifications say only that something new reached your inbox, without who did what or the task's title, for a screen other people can see.": "Известията казват само, че нещо ново е пристигнало във входящите, без кой какво е направил и без заглавието на задачата, за екран, който и други виждат.",
		"Something new reached your inbox.":       "Нещо ново пристигна във входящите ви.",
		"This browser cannot show notifications.": "Този браузър не може да показва известия.",
		"Notifications are blocked for this site. Allow them in the browser's site settings, then turn this on.": "Известията са блокирани за този сайт. Разрешете ги в настройките на браузъра за сайта и после включете това.",
	})
}
