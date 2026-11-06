// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The external sync screen's account of what runs on the server: the stored
// mapping document, the sealed credentials and what a source still lacks.
func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Sync runs on the tix server, not on your machine.": "Синхронизацията се изпълнява на сървъра на tix, а не на вашата машина.",
		"The mapping and the credentials a run uses have to be on the server. Upload the mapping and store the credentials in the source dialog, or from the command line with tix sync source set; the server's operator can use TIX_SYNC_ environment variables instead. Each source below says what its runs will use and what the server is still missing.": "Съпоставянето и идентификационните данни, които едно изпълнение използва, трябва да са на сървъра. Качете съпоставянето и запазете идентификационните данни в прозореца на източника или от командния ред с tix sync source set; операторът на сървъра може вместо това да използва променливи на средата TIX_SYNC_. Всеки източник по-долу казва какво ще използват изпълненията му и какво още липсва на сървъра.",
		"On the server":        "На сървъра",
		"Mapping":              "Съпоставяне",
		"Credentials":          "Идентификационни данни",
		"stored on the source": "съхранено в източника",
		"file on the server":   "файл на сървъра",
		"from the environment": "от средата",
		"none needed":          "не са нужни",
		"missing":              "липсва",
		"Missing on the server, so it cannot run:": "Липсва на сървъра, затова не може да се изпълни:",
		"Set up": "Настрой",
		"Runs execute on the tix server. Everything filled in here is stored there, on the source: the mapping document, the credentials and the settings. Anything left empty comes from the server's TIX_SYNC_ environment variables, which the Which variables list on each row names.": "Изпълненията стават на сървъра на tix. Всичко попълнено тук се съхранява там, в източника: документът за съпоставяне, идентификационните данни и настройките. Всичко оставено празно идва от променливите на средата TIX_SYNC_ на сървъра, които списъкът Кои променливи на всеки ред посочва.",
		"Setting up %s. Saving keeps its stored mapping and credentials unless you replace or clear them.": "Настройвате %s. Записването запазва съхраненото съпоставяне и идентификационните данни, освен ако не ги замените или изчистите.",
		"Checked on the server, nothing saved yet":                                                         "Проверено на сървъра, още нищо не е записано",
		"Imports into":             "Внася в",
		"Identified by":            "Идентифицира се по",
		"Statuses":                 "Статуси",
		"Missing on the server":    "Липсва на сървъра",
		"nothing, a run can start": "нищо, изпълнение може да започне",
		"Credentials typed before a check are not kept: type them again when you save.": "Идентификационните данни, въведени преди проверка, не се пазят: въведете ги отново, когато записвате.",
		"Mapping document": "Документ за съпоставяне",
		"The YAML that says which tix project records land in and what each external field becomes. Paste it here or choose the file below; it is stored on the source, on the server. Use Check to validate it against the project before saving.": "YAML документът, който казва в кой проект на tix попадат записите и в какво се превръща всяко външно поле. Поставете го тук или изберете файла по-долу; съхранява се в източника, на сървъра. Използвайте Провери, за да го валидирате спрямо проекта преди записване.",
		"Or upload a mapping file":   "Или качете файл за съпоставяне",
		"Mapping file on the server": "Файл за съпоставяне на сървъра",
		"A path on the server, for a mapping kept there by its operator. A document pasted or uploaded above wins over it.": "Път на сървъра, за съпоставяне, което операторът му държи там. Документ, поставен или качен по-горе, има предимство пред него.",
		"Remove the stored mapping document": "Премахни съхранения документ за съпоставяне",
		"Stored on the server, sealed with its key, and never shown again: only which ones are set and the last four characters of a long one. Leave these empty to keep what is stored. OpenProject takes the user apikey and an API key as the password; Jira takes an account and an API token.": "Съхраняват се на сървъра, запечатани с неговия ключ, и никога не се показват отново: само кои са зададени и последните четири знака на дългите. Оставете полетата празни, за да запазите съхраненото. OpenProject приема потребител apikey и API ключ като парола; Jira приема акаунт и API токен.",
		"API token":                     "API токен",
		"User":                          "Потребител",
		"Password or API key":           "Парола или API ключ",
		"Remove the stored credentials": "Премахни съхранените идентификационни данни",
		"Each setting filled in is stored on the source and wins over its environment variable. generic takes a file on the server; jira and openproject take a url, a project and a query.": "Всяка попълнена настройка се съхранява в източника и има предимство пред своята променлива на средата. generic приема файл на сървъра; jira и openproject приемат url, проект и заявка.",
		"File on the server": "Файл на сървъра",
		"Check":              "Провери",
		"A name, a system, and what the form stores on it: the settings, the mapping document and the credentials. A stored value wins over its environment variable on the server, and one left empty is read from that variable when a run starts. Credentials are sealed with the server's key and never shown, audited or logged.": "Име, система и това, което формата съхранява в него: настройките, документът за съпоставяне и идентификационните данни. Съхранената стойност има предимство пред своята променлива на средата на сървъра, а оставената празна се чете от тази променлива при стартиране на изпълнението. Идентификационните данни се запечатват с ключа на сървъра и никога не се показват, одитират или записват в журнали.",
		"A mapping": "Съпоставяне",
	})
}
