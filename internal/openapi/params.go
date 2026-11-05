// SPDX-License-Identifier: AGPL-3.0-or-later

package openapi

import (
	"reflect"
	"strings"
)

// pathParamDocs describes each path parameter by name. A name that means
// something else under one collection is keyed "collection/{name}" too, by
// the path segment before it, and that entry wins.
var pathParamDocs = map[string]string{
	"{ref}":                   "Task reference: the project key and number, such as INFRA-42, with the key not case sensitive, or the task's ID.",
	"projects/{ref}":          "Project key, such as infra, not case sensitive, or the project's ID.",
	"templates/{ref}":         "Template name, not case sensitive, or the template's ID.",
	"tenants/{ref}":           "Tenant key or ID. Only the caller's own tenant is found.",
	"{dep}":                   "The task depended on: a reference such as INFRA-41, or its ID.",
	"{name}":                  "Tag name.",
	"{key}":                   "Custom field key, as the project defines it.",
	"workflows/{key}":         "Workflow key. Reading and deleting also accept the workflow's ID.",
	"{filter}":                "Saved filter ID, or its name, not case sensitive. The caller's own filter wins over a shared one of the same name.",
	"{view}":                  "View: board, lanes, mine, or project:KEY for one project's board.",
	"{group}":                 "Group of the view: a state key on board or a project's board, a project key on lanes, or a deadline bucket on mine (overdue, today, week, later or none).",
	"{hostname}":              "Hostname, such as tix.example.com. Case, a port and a trailing dot are ignored.",
	"{actorID}":               "Actor ID of the member.",
	"{file}":                  "Feed file name: the feed's secret followed by .ics, as the feed address ends.",
	"actors/{id}":             "Actor ID.",
	"users/{id}":              "User's actor ID.",
	"tokens/{id}":             "API token ID.",
	"ssh-keys/{id}":           "SSH key ID.",
	"comments/{id}":           "Comment ID.",
	"inbox/{id}":              "Inbox item ID.",
	"channels/{id}":           "Notification channel ID.",
	"channel-deliveries/{id}": "Channel delivery ID.",
	"webhooks/{id}":           "Webhook endpoint ID.",
	"deliveries/{id}":         "Webhook delivery ID.",
	"sources/{id}":            "Sync source ID.",
	"connections/{id}":        "Connection ID, as the connection listing gives it. It is local to the server that answered.",
}

// pathParamDoc describes the path parameter of a route pattern, empty when
// the table has no entry for it.
func pathParamDoc(pattern, name string) string {
	wild := "{" + name + "}"
	before, _, ok := strings.Cut(pattern, "/"+wild)
	if ok {
		segment := before[strings.LastIndex(before, "/")+1:]
		if d, ok := pathParamDocs[segment+"/"+wild]; ok {
			return d
		}
	}
	return pathParamDocs[wild]
}

// queryDocs maps each query parameter an entry reads to its field comment.
func (g *generator) queryDocs(e entry) map[string]string {
	out := map[string]string{}
	add := func(e entry) {
		m, ok := serviceMethod(e.method)
		if q := query(e, m, ok); q != nil {
			g.queryFields(reflect.TypeOf(q), out)
		}
	}
	add(e)
	for _, a := range e.also {
		add(a)
	}
	return out
}

// queryFields records the comment of every query-tagged field of t,
// embedded structs included.
func (g *generator) queryFields(t reflect.Type, out map[string]string) {
	for i := range t.NumField() {
		f := t.Field(i)
		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			g.queryFields(f.Type, out)
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("query"), ",")
		if name == "" || name == "-" {
			continue
		}
		if _, seen := out[name]; !seen {
			out[name] = g.src.fields[typeKey(t)][f.Name]
		}
	}
}

// describeParams sets each parameter's description: a query parameter's from
// its field comment, a path parameter's from pathParamDocs.
func (g *generator) describeParams(op map[string]any, e entry) {
	params, _ := op["parameters"].([]any)
	var docs map[string]string
	for _, raw := range params {
		p, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if d, _ := p["description"].(string); d != "" {
			continue
		}
		name, _ := p["name"].(string)
		var d string
		switch p["in"] {
		case "path":
			d = pathParamDoc(e.route.Pattern, name)
		case "query":
			if docs == nil {
				docs = g.queryDocs(e)
			}
			d = docs[name]
		}
		if d != "" {
			p["description"] = d
		}
	}
}

// prefixDocs describes the parameter families a query reads by prefix, from
// the doc comment of the constant in the query type's package that spells
// each prefix.
func (g *generator) prefixDocs(q any, prefixes []string) map[string]any {
	pkg, _, _ := strings.Cut(typeKey(reflect.TypeOf(q)), ".")
	out := map[string]any{}
	for _, x := range prefixes {
		if d := g.src.values[pkg][x]; d != "" {
			out[x] = d
		}
	}
	return out
}
