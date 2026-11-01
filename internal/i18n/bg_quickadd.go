// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(Bulgarian, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		// The one-line add box and its quick-add syntax. The syntax itself is
		// not translated: !high, #tag and due:fri are what the parser reads in
		// every language.
		"Quick add":              "Бързо добавяне",
		"What the line sets":     "Какво задава редът",
		"no such project":        "няма такъв проект",
		"nobody has this handle": "никой няма този псевдоним",
		"Type the attributes into the line: !high or !1 to !5 for the priority, #tag, @handle for the assignee, due:fri, due:tomorrow, due:+3d or due:2026-11-20, +project, ^infra-3 for the parent, status:doing. They are taken out of the title. Put a backslash before one, or quotes around it, to keep it as text.": "Пишете атрибутите направо в реда: !high или от !1 до !5 за приоритета, #етикет, @псевдоним за изпълнителя, due:fri, due:tomorrow, due:+3d или due:2026-11-20 за крайния срок, +проект, ^infra-3 за родителската задача, status:doing. Те се махат от заглавието. Сложете обратна наклонена черта пред някой от тях или го оградете в кавички, за да остане като текст.",
	})
}
