// SPDX-License-Identifier: AGPL-3.0-or-later

// Package mcp serves the tix HTTP API to Model Context Protocol clients as
// tools generated from the OpenAPI document, so an agent's tools are the API
// and cannot drift from it. An overlay holds the few decisions the document
// cannot make: what to hide, what to rename and what is destructive.
package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// contentJSON is the one media type a tool reads and answers.
const contentJSON = "application/json"

// Param is one path or query parameter of an operation.
type Param struct {
	Name        string
	In          string
	Required    bool
	Description string
	Schema      map[string]any
}

// Operation is one operation of the OpenAPI document, its references resolved.
type Operation struct {
	ID          string
	Method      string
	Path        string
	Summary     string
	Description string
	// Scope is the bearer scope the operation needs, empty when an
	// authenticated actor is enough or the operation is public.
	Scope        string
	Params       []Param
	Body         map[string]any
	BodyRequired bool
	// Unsupported says why no tool can speak this operation, empty when one can.
	Unsupported string
}

// Document is the part of the OpenAPI document tools are built from.
type Document struct {
	Operations []Operation
	schemas    map[string]any
}

// Lookup returns the operation with the given operationId.
func (d *Document) Lookup(id string) (*Operation, bool) {
	for i := range d.Operations {
		if d.Operations[i].ID == id {
			return &d.Operations[i], true
		}
	}
	return nil, false
}

// LoadDocument reads an OpenAPI document in its JSON form.
func LoadDocument(raw []byte) (*Document, error) {
	var doc struct {
		Paths      map[string]map[string]json.RawMessage `json:"paths"`
		Components struct {
			Schemas map[string]any `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("reading the openapi document: %w", err)
	}
	d := &Document{schemas: doc.Components.Schemas}
	for path, item := range doc.Paths {
		for method, rawOp := range item {
			op, err := d.operation(strings.ToUpper(method), path, rawOp)
			if err != nil {
				return nil, fmt.Errorf("reading %s %s: %w", strings.ToUpper(method), path, err)
			}
			d.Operations = append(d.Operations, op)
		}
	}
	sort.Slice(d.Operations, func(i, j int) bool { return d.Operations[i].ID < d.Operations[j].ID })
	return d, nil
}

type rawOperation struct {
	OperationID string `json:"operationId"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	Parameters  []struct {
		Name        string         `json:"name"`
		In          string         `json:"in"`
		Required    bool           `json:"required"`
		Description string         `json:"description"`
		Schema      map[string]any `json:"schema"`
	} `json:"parameters"`
	RequestBody *struct {
		Required bool                            `json:"required"`
		Content  map[string]struct{ Schema any } `json:"content"`
	} `json:"requestBody"`
	Responses map[string]struct {
		Content map[string]json.RawMessage `json:"content"`
	} `json:"responses"`
	Security []map[string][]string `json:"security"`
}

func (d *Document) operation(method, path string, raw json.RawMessage) (Operation, error) {
	var in rawOperation
	if err := json.Unmarshal(raw, &in); err != nil {
		return Operation{}, err
	}
	if in.OperationID == "" {
		return Operation{}, fmt.Errorf("no operationId")
	}
	op := Operation{
		ID: in.OperationID, Method: method, Path: path,
		Summary: in.Summary, Description: in.Description,
		Scope: bearerScope(in.Security),
	}
	for _, p := range in.Parameters {
		schema, _ := d.resolve(p.Schema, nil).(map[string]any)
		op.Params = append(op.Params, Param{Name: p.Name, In: p.In, Required: p.Required || p.In == "path",
			Description: p.Description, Schema: schema})
	}
	if in.RequestBody != nil {
		for media, c := range in.RequestBody.Content {
			if media != contentJSON {
				op.Unsupported = "reads a " + media + " body, not JSON"
				continue
			}
			op.Body, _ = d.resolve(c.Schema, nil).(map[string]any)
			op.BodyRequired = in.RequestBody.Required
		}
	}
	if op.Unsupported == "" {
		op.Unsupported = answers(in.Responses)
	}
	return op, nil
}

// answers returns why the operation's success response is not one a tool can
// return, or empty when it answers JSON or nothing.
func answers(responses map[string]struct {
	Content map[string]json.RawMessage `json:"content"`
}) string {
	var other []string
	for code, r := range responses {
		if code == "101" {
			return "upgrades to a WebSocket stream"
		}
		if !strings.HasPrefix(code, "2") {
			continue
		}
		if code == "204" || len(r.Content) == 0 {
			return ""
		}
		if _, ok := r.Content[contentJSON]; ok {
			return ""
		}
		for media := range r.Content {
			other = append(other, media)
		}
	}
	if len(other) == 0 {
		return ""
	}
	sort.Strings(other)
	return "answers " + strings.Join(other, ", ") + ", not JSON"
}

// bearerScope returns the scope the bearer scheme demands, if any.
func bearerScope(security []map[string][]string) string {
	for _, req := range security {
		if scopes, ok := req["bearer"]; ok && len(scopes) > 0 {
			return scopes[0]
		}
	}
	return ""
}

// resolve returns a copy of schema with every component reference inlined.
// A reference already being expanded higher up is cut to a bare object, so a
// recursive type yields a finite schema.
func (d *Document) resolve(schema any, stack []string) any {
	switch v := schema.(type) {
	case map[string]any:
		if ref, ok := v["$ref"].(string); ok {
			name := strings.TrimPrefix(ref, "#/components/schemas/")
			for _, seen := range stack {
				if seen == name {
					return map[string]any{"type": "object", "description": "a nested " + name}
				}
			}
			return d.resolve(d.schemas[name], append(stack, name))
		}
		out := make(map[string]any, len(v))
		for k, child := range v {
			out[k] = d.resolve(child, stack)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = d.resolve(child, stack)
		}
		return out
	default:
		return v
	}
}

// isRead reports whether the method only reads.
func isRead(method string) bool { return method == http.MethodGet || method == http.MethodHead }
