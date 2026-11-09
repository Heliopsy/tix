// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// WorkflowTemplateContext is the catalog context of the built-in workflow
// templates' names, purposes and state labels, so a label such as Open or Done
// is translated for a template without taking the meaning it has elsewhere.
const WorkflowTemplateContext = "workflow template"

// WorkflowTemplateText translates a built-in workflow template's name, purpose
// or state label, falling back to the English text.
func WorkflowTemplateText(lang, text string) string { return C(lang, WorkflowTemplateContext, text) }

// workflowTemplateEntries are the templates' texts, keyed by their English.
var workflowTemplateEntries = map[string]string{
	"Simple Kanban":             "Обикновен канбан",
	"Personal (GTD)":            "Лично (GTD)",
	"Development with review":   "Разработка с преглед",
	"Bug triage":                "Сортиране на грешки",
	"Releases":                  "Издания",
	"Agent queue with sign-off": "Опашка за агенти с одобрение",
	"Incident":                  "Инцидент",
	"Support tickets":           "Заявки за поддръжка",
	"Approval or request":       "Одобрение или искане",
	"Sales or BizDev pipeline":  "Продажби или бизнес развитие",
	"Hiring":                    "Наемане",
	"Content or editorial":      "Съдържание или редакция",
	"Move cards left to right, nothing else.":              "Местете картите отляво надясно, нищо повече.",
	"Capture everything, decide later, do the next thing.": "Запишете всичко, решете по-късно, направете следващото.",
	"Code goes through review and QA before it's done.":    "Кодът минава през преглед и тестване, преди да е готов.",
	"Confirm bugs before anyone fixes them.":               "Потвърдете грешките, преди някой да ги поправя.",
	"Ship a change and prove it worked.":                   "Пуснете промяна и докажете, че работи.",
	"Agents do the work, a human approves it.":             "Агентите вършат работата, човек я одобрява.",
	"Restore service first, learn from it after.":          "Първо възстановете услугата, после си извлечете поука.",
	"Answer people and close the loop.":                    "Отговорете на хората и затворете въпроса.",
	"Someone asks, someone authorised decides.":            "Някой иска, упълномощен човек решава.",
	"Track deals from first contact to signed.":            "Следете сделките от първия контакт до подписа.",
	"Move candidates through interviews to a decision.":    "Водете кандидатите през интервютата до решение.",
	"Turn ideas into published pieces.":                    "Превърнете идеите в публикувани материали.",
	"Applied":                                              "Кандидатствал",
	"Approved":                                             "Одобрено",
	"Backlog":                                              "Натрупани",
	"Cancelled":                                            "Отменено",
	"Claimed":                                              "Поето",
	"Closed":                                               "Затворено",
	"Confirmed":                                            "Потвърдено",
	"Contacted":                                            "Осъществен контакт",
	"Deployed":                                             "Внедрено",
	"Deploying":                                            "Внедрява се",
	"Detected":                                             "Открит",
	"Doing":                                                "В процес",
	"Done":                                                 "Готово",
	"Drafting":                                             "Чернова",
	"Dropped":                                              "Изоставено",
	"Duplicate":                                            "Дубликат",
	"Editing":                                              "Редакция",
	"Fixed":                                                "Поправено",
	"Fixing":                                               "Поправя се",
	"Hired":                                                "Нает",
	"Idea":                                                 "Идея",
	"Inbox":                                                "Входящи",
	"In progress":                                          "В процес",
	"In review":                                            "В преглед",
	"Interview":                                            "Интервю",
	"Investigating":                                        "Разследва се",
	"Lead":                                                 "Потенциален клиент",
	"Lost":                                                 "Загубена",
	"Mitigated":                                            "Овладян",
	"Needs review":                                         "Чака преглед",
	"Negotiation":                                          "Преговори",
	"New":                                                  "Ново",
	"Next":                                                 "Следващо",
	"Offer":                                                "Оферта",
	"Open":                                                 "Отворено",
	"Planned":                                              "Планирано",
	"Post-mortem done":                                     "Анализът е готов",
	"Proposal":                                             "Предложение",
	"Published":                                            "Публикувано",
	"QA":                                                   "Тестване",
	"Qualified":                                            "Квалифицирана",
	"Queued":                                               "На опашка",
	"Ready":                                                "Готово за работа",
	"Rejected":                                             "Отхвърлено",
	"Requested":                                            "Поискано",
	"Resolved":                                             "Решено",
	"Rolled back":                                          "Върнато назад",
	"Screening":                                            "Първичен подбор",
	"Someday":                                              "Някой ден",
	"Staged":                                               "Подготвено",
	"To do":                                                "За правене",
	"Triage":                                               "Сортиране",
	"Triaged":                                              "Сортирано",
	"Under review":                                         "В преглед",
	"Verified":                                             "Проверено",
	"Waiting":                                              "Чака",
	"Waiting on customer":                                  "Чака клиента",
	"Withdrew":                                             "Оттегли се",
	"Won":                                                  "Спечелена",
	"Won't fix":                                            "Няма да се поправя",
}

func init() {
	entries := make(map[string]string, len(workflowTemplateEntries))
	for k, v := range workflowTemplateEntries {
		entries[WorkflowTemplateContext+Context+k] = v
	}
	register(Bulgarian, entries)
}

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// Workflow templates on the Workflows and Projects screens.
		"Add from template": "Добави от шаблон",
		"Add workflow":      "Добави работния процес",
		"Approve or reject work on a workflow edge that asks for it.": "Одобряване или отхвърляне на работа при преход в работния процес, който го изисква.",
		"Already in this tenant":           "Вече е в този тенант",
		"Built-in":                         "Вграден",
		"Built-in templates":               "Вградени шаблони",
		"Workflows in this tenant":         "Работни процеси в този тенант",
		"the template's key":               "ключът на шаблона",
		"If the key is taken":              "Ако ключът е зает",
		"Refuse":                           "Откажи",
		"Add it under the next free key":   "Добави го под следващия свободен ключ",
		"Replace the existing workflow":    "Замени съществуващия работен процес",
		"Keep the existing workflow":       "Запази съществуващия работен процес",
		"workflow created from a template": "работният процес е създаден от шаблон",
		"The template's custom fields are added to a project when the project is created from the template.":                                                          "Потребителските полета на шаблона се добавят към проект, когато проектът е създаден от шаблона.",
		"The key the new workflow is created under. Leave it empty to use the template's own key, which starts with tix- and is reserved for the built-in templates.": "Ключът, под който се създава новият работен процес. Оставете го празно, за да се използва ключът на шаблона, който започва с tix- и е запазен за вградените шаблони.",
	})
}
