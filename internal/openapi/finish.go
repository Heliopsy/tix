// SPDX-License-Identifier: AGPL-3.0-or-later

package openapi

import (
	"net/http"
	"sort"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/wire"
)

// finish adds what the reflector does not: security schemes, descriptions
// read from the source, the streamed and alternative media types, and links
// to the OpenSpec capabilities that specify each route.
func (g *generator) finish(doc map[string]any, entries []entry) {
	scopes := make([]string, 0, len(core.AllScopes))
	for _, s := range core.AllScopes {
		scopes = append(scopes, "`"+string(s)+"`")
	}
	scopeNote := "Each operation lists the scope it needs; an empty list needs an authenticated actor " +
		"and nothing more. The scopes are " + strings.Join(scopes, ", ") + "."
	components := obj(doc, "components")
	components["securitySchemes"] = map[string]any{
		schemeBearer: map[string]any{
			"type": "http", "scheme": "bearer",
			"description": "An API token, `Authorization: Bearer TOKEN`. " + scopeNote,
		},
		schemeSession: map[string]any{
			"type": "apiKey", "in": "cookie", "name": wire.SessionCookieName,
			"description": "The browser session cookie `POST " + wire.RouteLogin + "` sets. " + scopeNote,
		},
	}

	tags := map[string]bool{}
	paths := obj(doc, "paths")
	for _, e := range entries {
		op := obj(obj(paths, e.route.Pattern), strings.ToLower(e.route.Method))
		tags[tagOf(e.name)] = true
		g.describe(op, e)
		g.describeParams(op, e)
		if takesIdempotencyKey(e) {
			params, _ := op["parameters"].([]any)
			op["parameters"] = append(params, idempotencyKeyParam())
		}
		if e.contract.Public {
			op["security"] = []any{}
		}
		if rb, ok := op["requestBody"].(map[string]any); ok {
			rb["required"] = !e.contract.OptionalBody
			if e.contract.BodyForm {
				addFormBody(rb)
			}
		}
		responses := obj(op, "responses")
		shareErrors(components, responses)
		for code, r := range responses {
			if !strings.HasPrefix(code, "2") {
				continue
			}
			content, ok := r.(map[string]any)["content"].(map[string]any)
			if !ok {
				continue
			}
			if e.contract.Content == wire.ContentNDJSON {
				content[wire.ContentNDJSON] = map[string]any{"x-tix-stream": "one record per line"}
			}
			addStreamed(doc, content)
			for _, alt := range e.contract.Alt {
				content[alt] = altContent(alt)
			}
		}
		if p, ok := e.contract.Query.(interface{ QueryPrefixes() []string }); ok {
			var prefixes []any
			for _, x := range p.QueryPrefixes() {
				prefixes = append(prefixes, x)
			}
			op["x-tix-query-prefixes"] = prefixes
			op["x-tix-query-prefix-descriptions"] = g.prefixDocs(e.contract.Query, p.QueryPrefixes())
		}
	}

	names := make([]string, 0, len(tags))
	for t := range tags {
		names = append(names, t)
	}
	sort.Strings(names)
	list := make([]any, 0, len(names))
	for _, t := range names {
		list = append(list, map[string]any{"name": t})
	}
	doc["tags"] = list
}

// describe sets an operation's summary and description from the service
// method's doc comment, the command that reaches it, and its OpenSpec links.
func (g *generator) describe(op map[string]any, e entry) {
	text := e.summary
	if text == "" {
		text = g.src.methods[e.method]
	}
	if text == "" {
		text = e.name + "."
	}
	summary, _, _ := strings.Cut(text, ". ")
	op["summary"] = strings.TrimSuffix(summary, ".")
	parts := []string{text}
	if e.cli != "" {
		parts = append(parts, "Command line: `"+e.cli+"`.")
	}
	for _, a := range e.also {
		parts = append(parts, "With `?"+a.route.Query+"` this route is `"+a.name+"`: "+
			firstNonEmpty(g.src.methods[a.method], a.name+"."))
	}
	if caps := g.specsFor(e); len(caps) > 0 {
		links := make([]string, 0, len(caps))
		for _, c := range caps {
			links = append(links, "[`"+c+"`](openspec/specs/"+c+"/spec.md)")
		}
		parts = append(parts, "Specified in "+strings.Join(links, ", ")+".")
		op["x-tix-openspec"] = caps
	}
	op["description"] = strings.Join(parts, "\n\n")
}

// specsFor names the OpenSpec capabilities that mention an entry's route.
func (g *generator) specsFor(e entry) []string {
	var out []string
	for c, routes := range g.specs {
		m, err := NewMatcher([]string{e.route.Method + " " + e.route.Pattern})
		if err != nil {
			continue
		}
		for _, r := range routes {
			method, path, _ := strings.Cut(r, " ")
			if m.Match(method, path) != "" {
				out = append(out, c)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

// shareErrors moves an operation's error responses into components and
// refers to them, since every operation fails the same ways.
func shareErrors(components, responses map[string]any) {
	shared := obj(components, "responses")
	for code, r := range responses {
		content, _ := r.(map[string]any)["content"].(map[string]any)
		js, _ := content[wire.ContentJSON].(map[string]any)
		schema, _ := js["schema"].(map[string]any)
		if schema["$ref"] != "#/components/schemas/ErrorEnvelope" {
			continue
		}
		name := "Error" + code
		shared[name] = r
		responses[code] = map[string]any{"$ref": "#/components/responses/" + name}
	}
}

// addFormBody offers a JSON request body as a multipart form too: every
// field of the JSON body as a form field, and the file as a part named file.
func addFormBody(rb map[string]any) {
	content, ok := rb["content"].(map[string]any)
	if !ok {
		return
	}
	js, ok := content[wire.ContentJSON].(map[string]any)
	if !ok {
		return
	}
	content[wire.ContentMultipart] = map[string]any{"schema": map[string]any{
		"allOf": []any{js["schema"], map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file": map[string]any{"type": "string", "format": "binary",
					"description": "The file to read; content is used when there is no file."},
			},
		}},
	}}
}

// altContent describes a further media type a route answers in. JSON is an
// array of objects and NDJSON one object per line, since a route that
// negotiates them writes records whose keys the caller chose; anything else
// is text.
func altContent(media string) map[string]any {
	object := map[string]any{"type": "object"}
	switch media {
	case wire.ContentJSON:
		return map[string]any{"schema": map[string]any{"type": "array", "items": object}}
	case wire.ContentNDJSON:
		return map[string]any{"schema": object, "x-tix-stream": "one record per line"}
	default:
		return map[string]any{"schema": map[string]any{"type": "string"}}
	}
}

// addStreamed offers a listing as NDJSON too, one item per line, which every
// listing answers when the client sends Accept: application/x-ndjson.
func addStreamed(doc map[string]any, content map[string]any) {
	js, ok := content[wire.ContentJSON].(map[string]any)
	if !ok {
		return
	}
	schema := resolve(doc, js["schema"])
	props, _ := schema["properties"].(map[string]any)
	items, _ := props["items"].(map[string]any)
	if _, cursor := props["next_cursor"]; items == nil || !cursor {
		return
	}
	content[wire.ContentNDJSON] = map[string]any{
		"schema":       items["items"],
		"x-tix-stream": "one item per line, no envelope and no cursor",
	}
}

// resolve follows a local $ref to the schema it names.
func resolve(doc map[string]any, s any) map[string]any {
	m, _ := s.(map[string]any)
	ref, ok := m["$ref"].(string)
	if !ok {
		return m
	}
	name := strings.TrimPrefix(ref, "#/components/schemas/")
	out, _ := obj(obj(doc, "components"), "schemas")[name].(map[string]any)
	return out
}

// obj returns the object under key, creating it when absent.
func obj(m map[string]any, key string) map[string]any {
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	v := map[string]any{}
	m[key] = v
	return v
}

// firstNonEmpty returns the first of its arguments that is not empty.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// takesIdempotencyKey reports whether an operation is one the router accepts
// an Idempotency-Key on: an authenticated write under the API prefix.
func takesIdempotencyKey(e entry) bool {
	switch e.route.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return false
	}
	return strings.HasPrefix(e.route.Pattern, wire.APIPrefix) && !e.contract.Public &&
		!httpapi.IsPublicPath(e.route.Pattern)
}

// idempotencyKeyParam is the header a client sends to retry a write safely.
func idempotencyKeyParam() map[string]any {
	return map[string]any{
		"name":     wire.HeaderIdempotencyKey,
		"in":       "header",
		"required": false,
		"description": "Client-chosen key, such as a UUID, that makes retrying this write safe: a repeat of " +
			"the same request replays the first answer. See Retrying safely in docs/api.md.",
		"schema": map[string]any{
			"type": "string", "minLength": float64(1), "maxLength": float64(service.MaxIdempotencyKeyLen),
			"pattern": "^[ -~]+$",
		},
	}
}
