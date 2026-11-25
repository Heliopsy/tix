// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Something wrong?": "Нещо не е наред?",
		"Copy these facts into a bug report on the tix project's issue tracker. They name versions and this browser, never a token, a password or your work.": "Копирайте тези данни в доклад за грешка в системата за проблеми на проекта tix. Те назовават версии и този браузър, никога токен, парола или работата ви.",
		"Include the server address":                                 "Включи адреса на сървъра",
		"Off keeps a private server's address out of what you copy.": "Изключено, адресът на частен сървър не влиза в копираното.",
		"Copy diagnostics":                                           "Копирай диагностиката",
	})
}
