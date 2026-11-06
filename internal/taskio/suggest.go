// SPDX-License-Identifier: AGPL-3.0-or-later

package taskio

import (
	"slices"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// FieldInfo describes a custom field of the target project.
type FieldInfo struct {
	Key     string
	Label   string
	Type    core.FieldType
	Options []string
}

type candidate struct {
	target Target
	rank   int
}

func syn(t Target, rank int, names ...string) map[string]candidate {
	out := make(map[string]candidate, len(names))
	for _, n := range names {
		out[fold(n)] = candidate{t, rank}
	}
	return out
}

func merge(parts ...map[string]candidate) map[string]candidate {
	out := map[string]candidate{}
	for _, p := range parts {
		for k, v := range p {
			out[k] = v
		}
	}
	return out
}

var synonyms = merge(
	syn(TargetTitle, 0, "Summary", "Title", "Subject", "Заглавие", "Наименование"),
	syn(TargetTitle, 1, "Name", "Task", "Task name", "Име", "Задача"),
	syn(TargetBody, 0, "Description", "Body", "Desc", "Описание"),
	syn(TargetBody, 1, "Details", "Notes", "Content", "Детайли", "Бележки"),
	syn(TargetStatus, 0, "Status", "Статус", "Състояние"),
	syn(TargetStatus, 1, "State", "List", "Column", "Колона", "Списък"),
	syn(TargetPriority, 0, "Priority", "Prio", "Приоритет"),
	syn(TargetAssignee, 0, "Assignee", "Assigned to", "Owner", "Изпълнител", "Отговорник", "Assignee login"),
	syn(TargetAssignee, 1, "Assignees", "Assignee name", "Responsible", "Отговорници"),
	syn(TargetTags, 0, "Labels", "Tags", "Label", "Tag", "Етикети", "Етикет", "Тагове", "Таг"),
	syn(TargetDue, 0, "Due", "Due date", "Deadline", "Due at", "Due on", "Краен срок", "Срок", "Падеж"),
	syn(TargetParent, 0, "Parent", "Parent id", "Parent issue", "Parent key", "Parent ref", "Родител", "Родителска задача"),
	syn(TargetDependsOn, 0, "Depends on", "Blocked by", "Dependencies", "Dependency", "Зависи от", "Зависимости"),
	syn(TargetRecurrence, 0, "Recurrence", "Repeat", "Recurring", "Повторение"),
	syn(TargetRef, 0, "Ref"),
	syn(TargetExternalID, 0, "Issue key", "External ID", "Key", "Ключ"),
	syn(TargetExternalID, 1, "Number", "idShort", "Номер"),
	syn(TargetExternalID, 2, "ID", "Issue id", "Task id"),
)

var sourceSynonyms = map[string]map[string]candidate{
	SourceJira: merge(
		syn(TargetAlias, 0, "Issue id"),
		syn(TargetIgnore, 0, "Parent summary", "Status Category", "Status Category Changed", "Project key", "Project name", "Project type", "Project lead"),
		syn(TargetAppendBody, 0, "Issue Type", "Sprint", "Reporter", "Component/s", "Fix Version/s"),
	),
	SourceGitHub: merge(
		syn(TargetIgnore, 0, "id", "user.id"),
		syn(TargetAppendBody, 0, "milestone.title", "user.login"),
	),
	SourceTrello: merge(
		syn(TargetAlias, 0, "id"),
		syn(TargetStatus, 0, "list"),
		syn(TargetAssignee, 0, "members"),
		syn(TargetIgnore, 0, "closed", "shortLink"),
		syn(TargetAppendBody, 0, "checklists"),
	),
	SourceLinear: merge(
		syn(TargetExternalID, 0, "ID"),
		syn(TargetIgnore, 0, "Project ID", "Cycle Number"),
	),
	SourceTix: merge(
		syn(TargetExternalID, 0, "id"),
		syn(TargetIgnore, 0, "project", "starred", "project_id", "tenant_id", "seq"),
	),
}

var customAliases = [][]string{
	{"start", "startdate", "startat", "starton", "начало", "началнадата"},
	{"estimate", "points", "storypoints", "storypointestimate", "originalestimate", "оценка", "точки"},
}

var noise = map[string]bool{}

func init() {
	for _, n := range []string{
		"created", "updated", "resolved", "created at", "updated at", "closed at", "created date",
		"updated date", "last viewed", "last updated", "watchers", "watcher", "url", "html url",
		"closed", "archived", "canceled", "cancelled", "completed", "started", "triaged", "votes",
		"locked", "comments", "author association", "active lock reason", "state reason",
		"performed via github app", "time spent", "attachment", "attachments", "node id",
		"Създадена", "Създадено", "Обновена", "Обновено",
	} {
		noise[fold(n)] = true
	}
}

// Suggest proposes a target for each header, given the detected source and the project's custom fields.
func Suggest(headers []string, source string, fields []FieldInfo) map[string]Target {
	cands := make([]candidate, len(headers))
	best := map[Target]int{}
	for i, h := range headers {
		c := classify(h, source, fields)
		cands[i] = c
		if c.target.Multi() {
			continue
		}
		if j, ok := best[c.target]; !ok || c.rank < cands[j].rank {
			best[c.target] = i
		}
	}
	out := make(map[string]Target, len(headers))
	for i, h := range headers {
		c := cands[i]
		switch {
		case c.target.Multi() || best[c.target] == i:
			out[h] = c.target
		case c.target == TargetExternalID:
			out[h] = TargetAlias
		case c.target == TargetTitle || c.target == TargetBody || isCustom(c.target):
			out[h] = fallback(h)
		default:
			out[h] = TargetIgnore
		}
	}
	return out
}

func classify(h, source string, fields []FieldInfo) candidate {
	if c, ok := exportColumn(h, source, fields); ok {
		return c
	}
	f := fold(BaseHeader(h))
	if source == SourceJira {
		f = strings.TrimPrefix(f, "customfield")
	}
	if c, ok := sourceSynonyms[source][f]; ok {
		return c
	}
	if c, ok := synonyms[f]; ok {
		return c
	}
	for _, fi := range fields {
		if f == fold(fi.Key) || (fi.Label != "" && f == fold(fi.Label)) {
			return candidate{CustomTarget(fi.Key), 0}
		}
	}
	for _, group := range customAliases {
		if !slices.Contains(group, f) {
			continue
		}
		for _, fi := range fields {
			if slices.Contains(group, fold(fi.Key)) || slices.Contains(group, fold(fi.Label)) {
				return candidate{CustomTarget(fi.Key), 1}
			}
		}
	}
	return candidate{fallback(h), 0}
}

func fallback(h string) Target {
	f := fold(BaseHeader(h))
	if noise[f] {
		return TargetIgnore
	}
	if i := strings.LastIndex(h, "."); i >= 0 {
		switch fold(h[i+1:]) {
		case "name", "title", "login":
			return TargetAppendBody
		}
		return TargetIgnore
	}
	if len(f) > 3 && (strings.HasSuffix(f, "url") || strings.HasSuffix(f, "id")) {
		return TargetIgnore
	}
	return TargetAppendBody
}

func isCustom(t Target) bool {
	_, ok := t.CustomKey()
	return ok
}

// exportColumns are the columns tix export tasks writes that map straight onto
// a target of the same meaning, so a file tix wrote reads back with no mapping.
var exportColumns = map[string]Target{
	"ref": TargetRef, "title": TargetTitle, "body": TargetBody, "status": TargetStatus,
	"priority": TargetPriority, "assignee": TargetAssignee, "tags": TargetTags, "due": TargetDue,
	"parent": TargetParent, "depends_on": TargetDependsOn, "recurrence": TargetRecurrence,
	"id": TargetAlias, "created_at": TargetIgnore, "updated_at": TargetIgnore, "started_at": TargetIgnore,
	"completed_at": TargetIgnore, "deleted_at": TargetIgnore, "project": TargetIgnore,
	"creator": TargetIgnore, "comments": TargetIgnore, "history": TargetIgnore,
}

// exportColumn recognises a column of a file tix wrote by its exact name, and
// a custom field as custom:KEY or, flattened from JSON, custom.KEY in any file.
func exportColumn(h, source string, fields []FieldInfo) (candidate, bool) {
	if t, ok := exportColumns[h]; ok && source == SourceTix {
		return candidate{t, -1}, true
	}
	key, ok := strings.CutPrefix(h, customPrefix)
	if !ok {
		key, ok = strings.CutPrefix(h, "custom.")
	}
	if !ok {
		return candidate{}, false
	}
	for _, fi := range fields {
		if fi.Key == key {
			return candidate{CustomTarget(key), -1}, true
		}
	}
	return candidate{}, false
}
